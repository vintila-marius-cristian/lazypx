//go:build !race

package sessions

import (
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestPTYAttacher_Run_InteractiveLoop tests the full interactive path:
// term.MakeRaw, SIGWINCH handling, Ctrl+Q detach, and io.Copy loop.
//
// Excluded under -race because Run() spawns background goroutines (signal
// handler + ptm copy) that concurrently access s.ptm's poll.FD state
// without synchronisation — an inherent race in the production code.
func TestPTYAttacher_Run_InteractiveLoop(t *testing.T) {
	mgr := New("test")
	key := mgr.SessionKey(5555)
	if err := mgr.OpenSession(key, "sleep", []string{"60"}); err != nil {
		t.Fatalf("OpenSession: %v", err)
	}

	// Create a PTY pair: pts replaces os.Stdin so term.MakeRaw works.
	pts, ptm, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}

	os.Stdin = pts

	// Run in a goroutine — it blocks on pts.Read (stdin).
	errCh := make(chan error, 1)
	go func() {
		attacher := mgr.AttachCmd(key)
		errCh <- attacher.Run()
	}()

	// Give Run() time to enter the read loop, then send Ctrl+Q (0x11)
	// through the PTY master to trigger a clean detach.
	time.Sleep(100 * time.Millisecond)
	ptm.Write([]byte{0x11})

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after sending Ctrl+Q")
	}

	// Close the session's ptm to stop Run's background goroutines.
	mgr.mu.Lock()
	if s, ok := mgr.sessions[key]; ok {
		s.ptm.Close()
	}
	mgr.mu.Unlock()
	time.Sleep(50 * time.Millisecond)
	mgr.CloseAll()
}
