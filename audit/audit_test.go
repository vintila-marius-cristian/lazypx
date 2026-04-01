package audit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazypx/audit"
)

// reset closes the global audit file so each test starts clean.
func reset() { audit.Close() }

func TestInit_CreatesDirAndFile(t *testing.T) {
	reset()
	dir := filepath.Join(t.TempDir(), "subdir")
	if err := audit.Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer reset()

	// Dir should exist
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	// Log file should exist
	if audit.LogPath() == "" {
		t.Fatal("LogPath should not be empty after Init")
	}
	_, err = os.Stat(audit.LogPath())
	if err != nil {
		t.Fatalf("audit.log not created: %v", err)
	}
}

func TestInit_PreservesExistingLog(t *testing.T) {
	reset()
	dir := t.TempDir()

	// Write first log
	if err := audit.Init(dir); err != nil {
		t.Fatal(err)
	}
	audit.Log("p1", "root@pam", "START", "vm:100")
	audit.Close()

	content1, _ := os.ReadFile(filepath.Join(dir, "audit.log"))

	// Init again — should append, not truncate
	if err := audit.Init(dir); err != nil {
		t.Fatal(err)
	}
	audit.Log("p1", "root@pam", "STOP", "vm:100")
	audit.Close()

	content2, _ := os.ReadFile(filepath.Join(dir, "audit.log"))
	if len(content2) <= len(content1) {
		t.Error("second Init should append, not truncate")
	}
}

func TestLog_WritesCorrectFormat(t *testing.T) {
	reset()
	dir := t.TempDir()
	if err := audit.Init(dir); err != nil {
		t.Fatal(err)
	}
	defer reset()

	audit.Log("prod", "root@pam", "DELETE", "vm:200")

	data, err := os.ReadFile(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	if !strings.Contains(line, "[prod]") {
		t.Errorf("missing profile: %s", line)
	}
	if !strings.Contains(line, "[root@pam]") {
		t.Errorf("missing user: %s", line)
	}
	if !strings.Contains(line, "DELETE vm:200") {
		t.Errorf("missing action+resource: %s", line)
	}
}

func TestLog_MultipleEntries(t *testing.T) {
	reset()
	dir := t.TempDir()
	if err := audit.Init(dir); err != nil {
		t.Fatal(err)
	}
	defer reset()

	audit.Log("p", "u", "START", "vm:1")
	audit.Log("p", "u", "STOP", "vm:1")
	audit.Log("p", "u", "DELETE", "vm:1")

	data, _ := os.ReadFile(filepath.Join(dir, "audit.log"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %s", len(lines), string(data))
	}
}

func TestLog_NoOpBeforeInit(t *testing.T) {
	reset()
	// Don't call Init — Log should silently do nothing
	audit.Log("p", "u", "START", "vm:1")
	// No panic = pass
}

func TestLogPath_EmptyBeforeInit(t *testing.T) {
	reset()
	if audit.LogPath() != "" {
		t.Error("LogPath should be empty before Init")
	}
}

func TestLogPath_NonEmptyAfterInit(t *testing.T) {
	reset()
	dir := t.TempDir()
	audit.Init(dir)
	defer reset()

	if !strings.HasSuffix(audit.LogPath(), "audit.log") {
		t.Errorf("LogPath = %q, want suffix audit.log", audit.LogPath())
	}
}

func TestClose_MultipleCallsSafe(t *testing.T) {
	reset()
	dir := t.TempDir()
	audit.Init(dir)
	audit.Close()
	audit.Close() // should not panic
	audit.Close()
}

func TestInit_InvalidDir(t *testing.T) {
	reset()
	err := audit.Init("/dev/null/impossible/path")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
	// Close not needed since Init failed, but safe to call
	reset()
}

func TestInit_LogPathIsDir(t *testing.T) {
	reset()
	// Create a directory named "audit.log" so OpenFile fails
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "audit.log"), 0700)
	err := audit.Init(dir)
	if err == nil {
		t.Fatal("expected error when audit.log is a directory")
		reset()
	}
}

func TestLog_FilePermissions(t *testing.T) {
	reset()
	dir := t.TempDir()
	audit.Init(dir)
	defer reset()

	audit.Log("p", "u", "START", "vm:1")

	info, _ := os.Stat(filepath.Join(dir, "audit.log"))
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("audit.log permissions = %o, want 0600", perm)
	}
}
