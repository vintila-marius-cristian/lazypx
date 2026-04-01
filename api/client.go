package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxRetries     = 3
	retryBaseMs    = 500
	defaultTimeout = 15 * time.Second
)

// Client is the Proxmox API HTTP client.
type Client struct {
	baseURL    string
	authHeader string
	http       *http.Client
}

// NewClient creates a new Proxmox API client.
// tokenID format: "user@realm!tokenname"
// tokenSecret: UUID secret
func NewClient(host, tokenID, tokenSecret string, tlsInsecure bool) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: tlsInsecure, //nolint:gosec
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Client{
		baseURL:    strings.TrimRight(host, "/") + "/api2/json",
		authHeader: fmt.Sprintf("PVEAPIToken=%s=%s", tokenID, tokenSecret),
		http: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		},
	}
}

// get performs a GET request with retry/backoff, decoding JSON into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.doWithRetry(ctx, http.MethodGet, path, nil, out)
}

// post performs a POST request with retry/backoff.
func (c *Client) post(ctx context.Context, path string, body map[string]string, out any) error {
	return c.doWithRetry(ctx, http.MethodPost, path, body, out)
}

// put performs a PUT request with retry/backoff.
func (c *Client) put(ctx context.Context, path string, body map[string]string, out any) error {
	return c.doWithRetry(ctx, http.MethodPut, path, body, out)
}

// del performs a DELETE request.
func (c *Client) del(ctx context.Context, path string, out any) error {
	return c.doWithRetry(ctx, http.MethodDelete, path, nil, out)
}

// doWithRetry executes an HTTP request with exponential backoff retry.
// It only retries on errors where IsRetryable returns true.
func (c *Client) doWithRetry(ctx context.Context, method, path string, body map[string]string, out any) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))*float64(retryBaseMs)) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := c.do(ctx, method, path, body, out)
		if err == nil {
			return nil
		}
		// Classify and short-circuit on non-retryable errors
		classified := ClassifyError(err)
		if !IsRetryable(classified) {
			return classified
		}
		lastErr = classified
	}
	return fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

// ProxmoxError wraps an API HTTP error.
type ProxmoxError struct {
	StatusCode int
	Message    string
}

func (e *ProxmoxError) Error() string {
	return fmt.Sprintf("proxmox API error %d: %s", e.StatusCode, e.Message)
}

func isClientError(err error) bool {
	if e, ok := err.(*ProxmoxError); ok {
		return e.StatusCode >= 400 && e.StatusCode < 500
	}
	return false
}

// do executes a single HTTP request.
func (c *Client) do(ctx context.Context, method, path string, formBody map[string]string, out any) error {
	reqURL := c.baseURL + path

	var bodyReader io.Reader
	if len(formBody) > 0 {
		vals := url.Values{}
		for k, v := range formBody {
			vals.Set(k, v)
		}
		bodyReader = strings.NewReader(vals.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	if formBody != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		msg := RedactMessage(string(data))
		return &ProxmoxError{StatusCode: resp.StatusCode, Message: msg}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// Ping checks connectivity to the Proxmox API.
func (c *Client) Ping(ctx context.Context) error {
	var out APIResponse[any]
	return c.get(ctx, "/version", &out)
}
