package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lazypx/api"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// mockServer creates a test HTTP server that responds with the given status and body.
func mockServer(t *testing.T, status int, body string) (*httptest.Server, *api.Client) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	// Use TLSInsecure=true to accept the test server's self-signed cert.
	c := api.NewClient(srv.URL, "root@pam!test", "fake-secret", true)
	return srv, c
}

// sequentialServer serves multiple responses in order, cycling on the last one.
func sequentialServer(t *testing.T, responses []struct {
	Status int
	Body   string
}) (*httptest.Server, *api.Client) {
	t.Helper()
	idx := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := idx
		if i >= len(responses) {
			i = len(responses) - 1
		}
		idx++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(responses[i].Status)
		_, _ = w.Write([]byte(responses[i].Body))
	}))
	t.Cleanup(srv.Close)
	c := api.NewClient(srv.URL, "root@pam!test", "fake-secret", true)
	return srv, c
}

// ── Auth header ───────────────────────────────────────────────────────────────

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": {"version": "9.0"}}`))
	}))
	t.Cleanup(srv.Close)

	c := api.NewClient(srv.URL, "root@pam!mytoken", "my-secret-uuid", true)
	err := c.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	expected := "PVEAPIToken=root@pam!mytoken=my-secret-uuid"
	if gotAuth != expected {
		t.Errorf("auth header = %q, want %q", gotAuth, expected)
	}
}

// ── TLS insecure ─────────────────────────────────────────────────────────────

func TestTLSInsecureAcceptsSelfSigned(t *testing.T) {
	_, c := mockServer(t, 200, `{"data": {"version": "9.0"}}`)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("expected TLS insecure to work, got: %v", err)
	}
}

// ── Retry logic ───────────────────────────────────────────────────────────────

func TestRetrySucceedsAfterTransientErrors(t *testing.T) {
	_, c := sequentialServer(t, []struct {
		Status int
		Body   string
	}{
		{502, `{"errors": {"detail": "bad gateway"}}`},
		{502, `{"errors": {"detail": "bad gateway"}}`},
		{200, `{"data": {"version": "9.1"}}`},
	})
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
}

func TestRetryExhausted(t *testing.T) {
	_, c := sequentialServer(t, []struct {
		Status int
		Body   string
	}{
		{502, `{"errors": {"detail": "bad gateway"}}`},
		{502, `{"errors": {"detail": "bad gateway"}}`},
		{502, `{"errors": {"detail": "bad gateway"}}`},
	})
	if err := c.Ping(context.Background()); err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
}

// ── Error classification ──────────────────────────────────────────────────────

func TestErrorClassification401(t *testing.T) {
	_, c := mockServer(t, 401, `{"errors":{"permission":"Permission check failed"}}`)
	err := c.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !isUnauthorized(err) {
		t.Errorf("expected ErrUnauthorized, got %T: %v", err, err)
	}
}

func TestErrorClassification403(t *testing.T) {
	_, c := mockServer(t, 403, `{"errors":{"permission":"not allowed"}}`)
	err := c.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	// 403 should NOT be retried
	if api.IsRetryable(err) {
		t.Error("403 should not be retryable")
	}
}

func TestErrorClassification404NotRetryable(t *testing.T) {
	_, c := mockServer(t, 404, `{"errors":{"detail":"not found"}}`)
	_, err := c.GetNodes(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if api.IsRetryable(err) {
		t.Error("404 should not be retryable")
	}
}

func TestServerErrorIsRetryable(t *testing.T) {
	pe := &api.ProxmoxError{StatusCode: 500, Message: "internal error"}
	if !api.IsRetryable(pe) {
		t.Error("500 should be retryable")
	}
}

// ── GetNodes ──────────────────────────────────────────────────────────────────

func TestGetNodes(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"data": []map[string]any{
			{"node": "pve1", "status": "online", "cpu": 0.05, "maxcpu": 8, "mem": 1024, "maxmem": 8192},
			{"node": "pve2", "status": "online", "cpu": 0.12, "maxcpu": 16, "mem": 2048, "maxmem": 16384},
		},
	})
	_, c := mockServer(t, 200, string(body))
	nodes, err := c.GetNodes(context.Background())
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Node != "pve1" {
		t.Errorf("node[0].Node = %q, want pve1", nodes[0].Node)
	}
}

// ── Secret redaction ─────────────────────────────────────────────────────────

func TestSecretRedactionInErrors(t *testing.T) {
	longBody := make([]byte, 600)
	for i := range longBody {
		longBody[i] = 'x'
	}
	_, c := mockServer(t, 500, string(longBody))
	err := c.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	// Error message should be truncated
	if len(err.Error()) > 600 {
		t.Errorf("error message too long (%d chars), expected truncation", len(err.Error()))
	}
}

// ── Helper: check for ErrUnauthorized via errors.Is ──────────────────────────

func isUnauthorized(err error) bool {
	return api.IsRetryable(api.ClassifyError(err)) == false &&
		api.ClassifyError(err) == api.ErrUnauthorized
}

// ── StopVM & StopCT endpoints ───────────────────────────────────────────────

func TestStopVMEndpoint(t *testing.T) {
	// Verify it hits /nodes/{node}/qemu/{vmid}/status/stop
	var gotPath, gotMethod string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": "UPID:pve1:0000:0000:0000:qmstop:105:root@pam:"}`))
	}))
	t.Cleanup(srv.Close)

	c := api.NewClient(srv.URL, "root@pam!test", "fake-secret", true)
	upid, err := c.StopVM(context.Background(), "pve1", 105)
	if err != nil {
		t.Fatalf("StopVM failed: %v", err)
	}
	if upid == "" {
		t.Errorf("Expected UPID, got empty")
	}
	if gotMethod != "POST" {
		t.Errorf("Expected POST, got %s", gotMethod)
	}
	expectedPath := "/api2/json/nodes/pve1/qemu/105/status/stop"
	if gotPath != expectedPath {
		t.Errorf("Expected path %q, got %q", expectedPath, gotPath)
	}
}

func TestStopCTEndpoint(t *testing.T) {
	// Verify it hits /nodes/{node}/lxc/{vmid}/status/stop
	var gotPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": "UPID:pve1:0000:0000:0000:vzstop:105:root@pam:"}`))
	}))
	t.Cleanup(srv.Close)

	c := api.NewClient(srv.URL, "root@pam!test", "fake-secret", true)
	_, err := c.StopCT(context.Background(), "pve1", 105)
	if err != nil {
		t.Fatalf("StopCT failed: %v", err)
	}
	expectedPath := "/api2/json/nodes/pve1/lxc/105/status/stop"
	if gotPath != expectedPath {
		t.Errorf("Expected path %q, got %q", expectedPath, gotPath)
	}
}
