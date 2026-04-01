package sessions

import (
	"testing"
	"time"
)

func TestPTYAttacher_SetStdinNil(t *testing.T) {
	PTYAttacher{}.SetStdin(nil)
}

func TestPTYAttacher_SetStdoutNil(t *testing.T) {
	PTYAttacher{}.SetStdout(nil)
}

func TestPTYAttacher_SetStderrNil(t *testing.T) {
	PTYAttacher{}.SetStderr(nil)
}

func TestPTYAttacher_Run_SessionNotFound(t *testing.T) {
	mgr := New("test")
	attacher := mgr.AttachCmd("nonexistent-key")
	err := attacher.Run()
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if err.Error() != "session not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPTYAttacher_Run_SessionDied(t *testing.T) {
	mgr := New("test")
	key := mgr.SessionKey(9999)
	if err := mgr.OpenSession(key, "true", nil); err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer mgr.CloseAll()

	time.Sleep(200 * time.Millisecond)

	attacher := mgr.AttachCmd(key)
	err := attacher.Run()
	if err == nil {
		t.Fatal("expected error for exited session")
	}
	if err.Error() != "session process has exited" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPTYAttacher_Run_AlreadyAttached(t *testing.T) {
	mgr := New("test")
	key := mgr.SessionKey(1234)
	if err := mgr.OpenSession(key, "sleep", []string{"60"}); err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer mgr.CloseAll()

	// Directly mark the session as already attached.
	mgr.mu.Lock()
	s := mgr.sessions[key]
	s.mu.Lock()
	s.attached = true
	s.mu.Unlock()
	mgr.mu.Unlock()

	attacher := mgr.AttachCmd(key)
	err := attacher.Run()
	if err == nil {
		t.Fatal("expected error for already-attached session")
	}
	if err.Error() != "session already attached" {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify attached was NOT reset (Run returned before touching it).
	s.mu.Lock()
	if !s.attached {
		t.Error("attached should still be true after early-return error")
	}
	s.mu.Unlock()
}

func TestOpenSession_AlreadyAlive(t *testing.T) {
	mgr, key := openSleepSession(t, 3333)
	defer mgr.CloseAll()

	// Call OpenSession again while session is still running — should be no-op.
	if err := mgr.OpenSession(key, "sleep", []string{"60"}); err != nil {
		t.Fatalf("OpenSession on alive session should return nil, got: %v", err)
	}
	if len(mgr.ListSessions()) != 1 {
		t.Error("should not create a duplicate session")
	}
}
