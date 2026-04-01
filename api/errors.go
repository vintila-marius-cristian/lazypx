// Package api provides error classification for Proxmox API responses.
package api

import (
	"errors"
	"regexp"
	"strings"
)

// Sentinel errors for well-known Proxmox API conditions.
var (
	// ErrUnauthorized is returned when the API token is missing or expired (HTTP 401).
	ErrUnauthorized = errors.New("proxmox: unauthorized — check token_id and token_secret")
	// ErrForbidden is returned when the authenticated user lacks permission (HTTP 403).
	ErrForbidden = errors.New("proxmox: permission denied")
	// ErrNotFound is returned when the requested resource does not exist (HTTP 404).
	ErrNotFound = errors.New("proxmox: resource not found")
	// ErrLocked is returned when the resource is locked by another operation (HTTP 500 w/ lock msg).
	ErrLocked = errors.New("proxmox: resource is locked")
	// ErrQuorumLoss is returned when the cluster has lost quorum.
	ErrQuorumLoss = errors.New("proxmox: cluster quorum lost")
)

// ClassifyError maps a raw ProxmoxError to a typed sentinel where applicable,
// or returns the original error if no known mapping exists.
func ClassifyError(err error) error {
	if err == nil {
		return nil
	}
	var pe *ProxmoxError
	if !errors.As(err, &pe) {
		return err
	}
	switch pe.StatusCode {
	case 401:
		return ErrUnauthorized
	case 403:
		return ErrForbidden
	case 404:
		return ErrNotFound
	}
	// 5xx — check for known lock / quorum messages in body
	msg := strings.ToLower(pe.Message)
	if strings.Contains(msg, "is locked") || strings.Contains(msg, "lock") {
		return ErrLocked
	}
	if strings.Contains(msg, "quorum") {
		return ErrQuorumLoss
	}
	return err
}

// IsRetryable returns true if the error is likely transient and safe to retry.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Never retry auth or not-found
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound) {
		return false
	}
	var pe *ProxmoxError
	if errors.As(err, &pe) {
		// Retry 5xx server errors, not 4xx client errors
		return pe.StatusCode >= 500 || pe.StatusCode == 429
	}
	// Network errors are retryable
	return true
}

// RedactMessage strips potential credential material from an error message.
// Used before surfacing errors in the TUI or logs.
func RedactMessage(msg string) string {
	msg = redactSecrets(msg)
	if len(msg) > 500 {
		return msg[:500] + " …[truncated]"
	}
	return msg
}

// uuidPattern matches strings that look like UUIDs or token secrets.
var uuidPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// tokenPattern matches PVEAPIToken header values that may leak into error bodies.
var tokenPattern = regexp.MustCompile(`PVEAPIToken=\S+`)

func redactSecrets(s string) string {
	s = uuidPattern.ReplaceAllString(s, "[REDACTED-UUID]")
	s = tokenPattern.ReplaceAllString(s, "PVEAPIToken=[REDACTED]")
	return s
}
