package cache_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"lazypx/api"
	"lazypx/cache"
)

func newMockAPIClient(t *testing.T) *api.Client {
	t.Helper()
	nodes := map[string]any{
		"data": []map[string]any{
			{"node": "pve1", "status": "online"},
		},
	}
	emptyList := map[string]any{"data": []any{}}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		var body any
		switch r.URL.Path {
		case "/api2/json/nodes":
			body = nodes
		default:
			body = emptyList
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return api.NewClient(srv.URL, "root@pam!test", "secret", true)
}

// TestConcurrentRefresh verifies the cache handles 100 simultaneous callers safely.
func TestConcurrentRefresh(t *testing.T) {
	c := newMockAPIClient(t)
	ca := cache.New(c, 30*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap := ca.Get(context.Background())
			if snap.Error != nil {
				t.Errorf("unexpected error: %v", snap.Error)
			}
		}()
	}
	wg.Wait()
}

// TestTTLExpiry verifies that the cache treats a zero-time snapshot as stale.
func TestTTLExpiry(t *testing.T) {
	c := newMockAPIClient(t)
	ca := cache.New(c, 1*time.Millisecond)

	// First Get — populates cache
	snap1 := ca.Get(context.Background())
	if snap1.Error != nil {
		t.Fatalf("first get: %v", snap1.Error)
	}

	// Wait for TTL to expire
	time.Sleep(5 * time.Millisecond)

	// Second Get — should re-fetch (different FetchedAt)
	snap2 := ca.Get(context.Background())
	if snap2.Error != nil {
		t.Fatalf("second get: %v", snap2.Error)
	}
	if !snap2.FetchedAt.After(snap1.FetchedAt) {
		t.Error("expected second snapshot to have a newer FetchedAt after TTL expiry")
	}
}

// TestInvalidateForcesRefresh verifies that Invalidate causes the next Get to re-fetch.
func TestInvalidateForcesRefresh(t *testing.T) {
	c := newMockAPIClient(t)
	ca := cache.New(c, 5*time.Minute)

	snap1 := ca.Get(context.Background())
	if snap1.Error != nil {
		t.Fatalf("first get: %v", snap1.Error)
	}
	ca.Invalidate()

	snap2 := ca.Get(context.Background())
	if snap2.Error != nil {
		t.Fatalf("second get: %v", snap2.Error)
	}
	if !snap2.FetchedAt.After(snap1.FetchedAt) {
		t.Error("expected fresh snapshot after Invalidate")
	}
}
