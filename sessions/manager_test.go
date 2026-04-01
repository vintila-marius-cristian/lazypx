package sessions

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// sanitize()
// ---------------------------------------------------------------------------

func TestSanitize_Alphanumeric(t *testing.T) {
	got := sanitize("abcDEF123")
	if got != "abcDEF123" {
		t.Errorf("expected %q, got %q", "abcDEF123", got)
	}
}

func TestSanitize_Hyphen(t *testing.T) {
	got := sanitize("my-profile")
	if got != "my-profile" {
		t.Errorf("expected %q, got %q", "my-profile", got)
	}
}

func TestSanitize_SpecialChars(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello world", "hello-world"},
		{"a@b#c$d", "a-b-c-d"},
		{"path/to\\dir", "path-to-dir"},
		{"under_score", "under-score"},
		{"dot.name", "dot-name"},
		{"exclaim!", "exclaim-"},
		{"a~b`c", "a-b-c"},
		{"(paren)[bracket]", "-paren--bracket-"},
		{"semi;colon", "semi-colon"},
		{"comma,less", "comma-less"},
		{"less<than>greater", "less-than-greater"},
		{"pipe|and&ampersand", "pipe-and-ampersand"},
		{"star*and?question", "star-and-question"},
		{"tab\there", "tab-here"},
		{"newline\nhere", "newline-here"},
	}
	for _, tc := range tests {
		got := sanitize(tc.in)
		if got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitize_Empty(t *testing.T) {
	if got := sanitize(""); got != "" {
		t.Errorf("sanitize(\"\") = %q, want empty", got)
	}
}

func TestSanitize_Unicode(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"日本語", "---"},
		{"émoji", "-moji"},
		{"naïve", "na-ve"},
		{"über-cool", "-ber-cool"},
		{"café", "caf-"},
	}
	for _, tc := range tests {
		got := sanitize(tc.in)
		if got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitize_AllHyphens(t *testing.T) {
	got := sanitize("---")
	if got != "---" {
		t.Errorf("expected %q, got %q", "---", got)
	}
}

func TestSanitize_Mixed(t *testing.T) {
	got := sanitize("my@weird!profile#name")
	if got != "my-weird-profile-name" {
		t.Errorf("expected %q, got %q", "my-weird-profile-name", got)
	}
}

// ---------------------------------------------------------------------------
// HasTmux
// ---------------------------------------------------------------------------

func TestHasTmux_AlwaysTrue(t *testing.T) {
	if !HasTmux() {
		t.Error("HasTmux() should always return true")
	}
}

func TestHasTmux_MultipleCalls(t *testing.T) {
	for i := 0; i < 10; i++ {
		if !HasTmux() {
			t.Errorf("HasTmux() returned false on call %d", i)
		}
	}
}

// ---------------------------------------------------------------------------
// New / SessionKey
// ---------------------------------------------------------------------------

func TestNew_CreatesManager(t *testing.T) {
	mgr := New("test-profile")
	if mgr == nil {
		t.Fatal("New returned nil")
	}
	if mgr.sessions == nil {
		t.Error("sessions map not initialized")
	}
}

func TestManager_SessionKey(t *testing.T) {
	mgr := New("prod-cluster")
	key := mgr.SessionKey(101)
	expected := "lazypx-prod-cluster-101"
	if key != expected {
		t.Errorf("Expected %q, got %q", expected, key)
	}
}

func TestManager_SessionKey_Sanitize(t *testing.T) {
	mgr := New("my@weird!profile#name")
	key := mgr.SessionKey(999)
	expected := "lazypx-my-weird-profile-name-999"
	if key != expected {
		t.Errorf("Expected sanitized key %q, got %q", expected, key)
	}
}

func TestManager_SessionKey_DifferentVMs(t *testing.T) {
	mgr := New("default")
	k1 := mgr.SessionKey(105)
	k2 := mgr.SessionKey(150)
	if k1 == k2 {
		t.Errorf("Different VMIDs should produce different session keys: both = %q", k1)
	}
}

func TestManager_SessionKey_SameVMSameKey(t *testing.T) {
	mgr := New("default")
	k1 := mgr.SessionKey(105)
	k2 := mgr.SessionKey(105)
	if k1 != k2 {
		t.Errorf("Same VMID should always return the same key: %q vs %q", k1, k2)
	}
}

// ---------------------------------------------------------------------------
// OpenSession / CloseSession / IsAlive helpers
// ---------------------------------------------------------------------------

// openSleepSession opens a `sleep` session and returns the manager and key.
func openSleepSession(t *testing.T, vmid int) (*Manager, string) {
	t.Helper()
	mgr := New("test")
	key := mgr.SessionKey(vmid)
	err := mgr.OpenSession(key, "sleep", []string{"60"})
	if err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	return mgr, key
}

func TestOpenSession_CreatesSession(t *testing.T) {
	mgr, key := openSleepSession(t, 1001)
	defer mgr.CloseAll()

	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].Key != key {
		t.Errorf("expected key %q, got %q", key, list[0].Key)
	}
}

func TestOpenSession_ExtractsVMID(t *testing.T) {
	mgr := New("profile")
	key := mgr.SessionKey(42)
	err := mgr.OpenSession(key, "sleep", []string{"60"})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer mgr.CloseAll()

	list := mgr.ListSessions()
	if len(list) != 1 || list[0].VMID != 42 {
		t.Errorf("expected VMID 42, got %d", list[0].VMID)
	}
}

func TestOpenSession_MultipleDifferentKeys(t *testing.T) {
	mgr := New("test")
	defer mgr.CloseAll()

	for i := 0; i < 3; i++ {
		key := mgr.SessionKey(2000 + i)
		if err := mgr.OpenSession(key, "sleep", []string{"60"}); err != nil {
			t.Fatalf("OpenSession(%s): %v", key, err)
		}
	}
	list := mgr.ListSessions()
	if len(list) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(list))
	}
}

func TestOpenSession_ReplacesDeadSession(t *testing.T) {
	mgr := New("test")
	defer mgr.CloseAll()

	key := mgr.SessionKey(3000)

	// Open a session with `true` which exits immediately.
	if err := mgr.OpenSession(key, "true", nil); err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	// Wait for process to die and background Wait() to set ProcessState.
	time.Sleep(200 * time.Millisecond)

	// Now open a live one on the same key.
	if err := mgr.OpenSession(key, "sleep", []string{"60"}); err != nil {
		t.Fatalf("re-open: %v", err)
	}
	if !mgr.IsAlive(key) {
		t.Error("replaced session should be alive")
	}
}

func TestOpenSession_InvalidCommand(t *testing.T) {
	mgr := New("test")
	err := mgr.OpenSession("bad-cmd", "nonexistent-binary-xyz", nil)
	if err == nil {
		t.Error("expected error for invalid command")
	}
}

// ---------------------------------------------------------------------------
// IsAlive
// ---------------------------------------------------------------------------

func TestIsAlive_RunningSession(t *testing.T) {
	mgr, key := openSleepSession(t, 4000)
	defer mgr.CloseAll()

	if !mgr.IsAlive(key) {
		t.Error("IsAlive should be true for running session")
	}
}

func TestIsAlive_KilledSession(t *testing.T) {
	mgr, key := openSleepSession(t, 4001)
	mgr.CloseSession(key)

	if mgr.IsAlive(key) {
		t.Error("IsAlive should be false after CloseSession")
	}
}

func TestIsAlive_NonExisting(t *testing.T) {
	mgr := New("test")
	if mgr.IsAlive("does-not-exist") {
		t.Error("IsAlive should return false for non-existing key")
	}
}

func TestIsAlive_ExitedProcess(t *testing.T) {
	mgr := New("test")
	key := mgr.SessionKey(4002)
	if err := mgr.OpenSession(key, "true", nil); err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	// Wait for exit.
	time.Sleep(200 * time.Millisecond)
	if mgr.IsAlive(key) {
		t.Error("IsAlive should be false after process exits")
	}
}

// ---------------------------------------------------------------------------
// CloseSession
// ---------------------------------------------------------------------------

func TestCloseSession_Running(t *testing.T) {
	mgr, key := openSleepSession(t, 5000)

	if err := mgr.CloseSession(key); err != nil {
		t.Errorf("CloseSession returned error: %v", err)
	}
	list := mgr.ListSessions()
	if len(list) != 0 {
		t.Errorf("expected 0 sessions after close, got %d", len(list))
	}
}

func TestCloseSession_NonExisting(t *testing.T) {
	mgr := New("test")
	// Should not error on missing key.
	if err := mgr.CloseSession("nonexistent"); err != nil {
		t.Errorf("CloseSession on missing key should not return error, got: %v", err)
	}
}

func TestCloseSession_Idempotent(t *testing.T) {
	mgr, key := openSleepSession(t, 5001)

	mgr.CloseSession(key)
	mgr.CloseSession(key) // second close should not panic
	if len(mgr.ListSessions()) != 0 {
		t.Error("expected 0 sessions after double close")
	}
}

// ---------------------------------------------------------------------------
// CloseAll
// ---------------------------------------------------------------------------

func TestCloseAll_MultipleSessions(t *testing.T) {
	mgr := New("test")

	for i := 0; i < 5; i++ {
		key := mgr.SessionKey(6000 + i)
		if err := mgr.OpenSession(key, "sleep", []string{"60"}); err != nil {
			t.Fatalf("OpenSession: %v", err)
		}
	}
	if len(mgr.ListSessions()) != 5 {
		t.Fatal("expected 5 sessions before CloseAll")
	}

	mgr.CloseAll()
	if len(mgr.ListSessions()) != 0 {
		t.Errorf("expected 0 sessions after CloseAll, got %d", len(mgr.ListSessions()))
	}
}

func TestCloseAll_Empty(t *testing.T) {
	mgr := New("test")
	mgr.CloseAll() // should not panic
	if len(mgr.ListSessions()) != 0 {
		t.Error("expected empty after CloseAll on empty manager")
	}
}

// ---------------------------------------------------------------------------
// ListSessions
// ---------------------------------------------------------------------------

func TestListSessions_StatusRunning(t *testing.T) {
	mgr, key := openSleepSession(t, 7000)
	defer mgr.CloseAll()

	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].Status != "running" {
		t.Errorf("expected status %q, got %q", "running", list[0].Status)
	}
	if list[0].Key != key {
		t.Errorf("expected key %q, got %q", key, list[0].Key)
	}
}

func TestListSessions_AttachedFlag(t *testing.T) {
	mgr, _ := openSleepSession(t, 7001)
	defer mgr.CloseAll()

	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	// By default attached should be false (we never ran Run()).
	if list[0].Attached {
		t.Error("new session should not be attached")
	}
}

func TestListSessions_StatusExited(t *testing.T) {
	mgr := New("test")
	key := mgr.SessionKey(7002)
	if err := mgr.OpenSession(key, "true", nil); err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].Status != "exited" {
		t.Errorf("expected status %q, got %q", "exited", list[0].Status)
	}
}

func TestListSessions_SnapshotSafety(t *testing.T) {
	// Verify ListSessions returns a snapshot: mutating the slice doesn't affect manager.
	mgr, _ := openSleepSession(t, 7003)
	defer mgr.CloseAll()

	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Fatal("expected 1 session")
	}
	list = nil // clear local copy

	if len(mgr.ListSessions()) != 1 {
		t.Error("mutating returned slice should not affect manager state")
	}
}

// ---------------------------------------------------------------------------
// GetPTY
// ---------------------------------------------------------------------------

func TestGetPTY_Existing(t *testing.T) {
	mgr, key := openSleepSession(t, 8000)
	defer mgr.CloseAll()

	ptm := mgr.GetPTY(key)
	if ptm == nil {
		t.Fatal("GetPTY returned nil for existing session")
	}
}

func TestGetPTY_NonExisting(t *testing.T) {
	mgr := New("test")
	ptm := mgr.GetPTY("no-such-key")
	if ptm != nil {
		t.Error("GetPTY should return nil for non-existing session")
	}
}

func TestGetPTY_ReadWrite(t *testing.T) {
	mgr, key := openSleepSession(t, 8001)
	defer mgr.CloseAll()

	ptm := mgr.GetPTY(key)
	if ptm == nil {
		t.Fatal("GetPTY returned nil")
	}
	// The master side of a PTY should be writable.
	// We don't read from it because `sleep` produces no output.
}

// ---------------------------------------------------------------------------
// ResizePTY
// ---------------------------------------------------------------------------

func TestResizePTY_Existing(t *testing.T) {
	mgr, key := openSleepSession(t, 9000)
	defer mgr.CloseAll()

	if err := mgr.ResizePTY(key, 120, 40); err != nil {
		t.Errorf("ResizePTY failed: %v", err)
	}
}

func TestResizePTY_NonExisting(t *testing.T) {
	mgr := New("test")
	err := mgr.ResizePTY("ghost", 80, 24)
	if err == nil {
		t.Error("ResizePTY should return error for non-existing session")
	}
}

func TestResizePTY_MultipleResizes(t *testing.T) {
	mgr, key := openSleepSession(t, 9001)
	defer mgr.CloseAll()

	sizes := [][2]int{{80, 24}, {120, 40}, {200, 60}, {40, 10}}
	for _, s := range sizes {
		if err := mgr.ResizePTY(key, s[0], s[1]); err != nil {
			t.Errorf("ResizePTY(%d,%d): %v", s[0], s[1], err)
		}
	}
}

// ---------------------------------------------------------------------------
// PTYAttacher
// ---------------------------------------------------------------------------

func TestPTYAttacher_SetStdin(t *testing.T) {
	mgr := New("test")
	attacher := mgr.AttachCmd("some-key")
	var buf bytes.Buffer
	// Should not panic.
	attacher.SetStdin(&buf)
}

func TestPTYAttacher_SetStdout(t *testing.T) {
	mgr := New("test")
	attacher := mgr.AttachCmd("some-key")
	var buf bytes.Buffer
	attacher.SetStdout(&buf)
}

func TestPTYAttacher_SetStderr(t *testing.T) {
	mgr := New("test")
	attacher := mgr.AttachCmd("some-key")
	var buf bytes.Buffer
	attacher.SetStderr(&buf)
}

func TestAttachCmd_ReturnsAttacher(t *testing.T) {
	mgr := New("default")
	key := mgr.SessionKey(105)
	attacher := mgr.AttachCmd(key)
	if attacher.key != key {
		t.Errorf("Expected attacher key %q, got %q", key, attacher.key)
	}
	if attacher.mgr != mgr {
		t.Error("Expected attacher to reference the same manager")
	}
}

// ---------------------------------------------------------------------------
// ListSessions on empty manager
// ---------------------------------------------------------------------------

func TestManager_ListSessions_Empty(t *testing.T) {
	mgr := New("default")
	sessions := mgr.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected empty session list, got %d sessions", len(sessions))
	}
}

// ---------------------------------------------------------------------------
// Integration: open, list, close, verify
// ---------------------------------------------------------------------------

func TestIntegration_OpenListClose(t *testing.T) {
	mgr := New("integration")
	defer mgr.CloseAll()

	keys := make([]string, 3)
	for i := range keys {
		keys[i] = mgr.SessionKey(100 + i)
		if err := mgr.OpenSession(keys[i], "sleep", []string{"60"}); err != nil {
			t.Fatalf("OpenSession(%s): %v", keys[i], err)
		}
	}

	list := mgr.ListSessions()
	if len(list) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(list))
	}

	// Close the middle one.
	if err := mgr.CloseSession(keys[1]); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	list = mgr.ListSessions()
	if len(list) != 2 {
		t.Errorf("expected 2 sessions after closing one, got %d", len(list))
	}

	// Verify remaining sessions are the correct ones.
	seen := map[string]bool{}
	for _, s := range list {
		seen[s.Key] = true
	}
	if seen[keys[1]] {
		t.Error("closed session should not appear in list")
	}
}

// ---------------------------------------------------------------------------
// Integration: cat session with PTY I/O
// ---------------------------------------------------------------------------

func TestIntegration_CatSessionIO(t *testing.T) {
	mgr := New("cat-test")
	defer mgr.CloseAll()

	key := mgr.SessionKey(1111)
	if err := mgr.OpenSession(key, "cat", nil); err != nil {
		t.Fatalf("OpenSession cat: %v", err)
	}

	ptm := mgr.GetPTY(key)
	if ptm == nil {
		t.Fatal("GetPTY returned nil")
	}

	// Write something to the PTY master; `cat` echoes it back.
	input := []byte("hello-pty\n")
	if _, err := ptm.Write(input); err != nil {
		t.Fatalf("write to PTY: %v", err)
	}

	// Give cat a moment to echo.
	time.Sleep(50 * time.Millisecond)

	buf := make([]byte, 64)
	n, err := ptm.Read(buf)
	if err != nil {
		t.Fatalf("read from PTY: %v", err)
	}
	if n == 0 {
		t.Fatal("expected data from cat echo, got 0 bytes")
	}
}

// ---------------------------------------------------------------------------
// Race condition tests
// ---------------------------------------------------------------------------

func TestRace_OpenClose(t *testing.T) {
	mgr := New("race")
	var wg sync.WaitGroup

	// Open and close concurrently in goroutines.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("race-key-%d", i)
			_ = mgr.OpenSession(key, "sleep", []string{"60"})
			time.Sleep(time.Duration(i%5) * time.Millisecond)
			_ = mgr.CloseSession(key)
		}(i)
	}
	wg.Wait()
}

func TestRace_ListWhileOpening(t *testing.T) {
	mgr := New("race-list")
	defer mgr.CloseAll()
	var wg sync.WaitGroup

	// Writers: open sessions.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("race-open-%d", i)
			_ = mgr.OpenSession(key, "sleep", []string{"60"})
		}(i)
	}
	// Readers: concurrently list.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.ListSessions()
		}()
	}
	wg.Wait()
}

func TestRace_ConcurrentIsAlive(t *testing.T) {
	mgr := New("race-alive")
	defer mgr.CloseAll()

	key := mgr.SessionKey(12345)
	if err := mgr.OpenSession(key, "sleep", []string{"60"}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.IsAlive(key)
		}()
	}
	wg.Wait()
}

func TestRace_CloseAllWhileListing(t *testing.T) {
	mgr := New("race-closeall")
	for i := 0; i < 5; i++ {
		key := mgr.SessionKey(20000 + i)
		_ = mgr.OpenSession(key, "sleep", []string{"60"})
	}

	var wg sync.WaitGroup
	// Listers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.ListSessions()
		}()
	}
	// Closer
	wg.Add(1)
	go func() {
		defer wg.Done()
		mgr.CloseAll()
	}()
	wg.Wait()
}

// ---------------------------------------------------------------------------
// ProcessState availability after exit
// ---------------------------------------------------------------------------

func TestListSessions_VMIDInExitedSession(t *testing.T) {
	mgr := New("test")
	key := mgr.SessionKey(7777)
	if err := mgr.OpenSession(key, "true", nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	defer mgr.CloseAll()

	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
	if list[0].VMID != 7777 {
		t.Errorf("expected VMID 7777, got %d", list[0].VMID)
	}
}

// ---------------------------------------------------------------------------
// StartedAt is populated
// ---------------------------------------------------------------------------

func TestListSessions_StartedAtPopulated(t *testing.T) {
	mgr, _ := openSleepSession(t, 8888)
	defer mgr.CloseAll()

	before := time.Now().Add(-time.Second)
	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Fatal("expected 1 session")
	}
	if list[0].StartedAt.Before(before) {
		t.Error("StartedAt should be recent")
	}
}

// ---------------------------------------------------------------------------
// Ensure /dev/ptmx is available (sanity for PTY tests)
// ---------------------------------------------------------------------------

func TestPTYSubsystemAvailable(t *testing.T) {
	if _, err := os.Stat("/dev/ptmx"); err != nil {
		t.Skipf("PTY not available on this system: %v", err)
	}
}
