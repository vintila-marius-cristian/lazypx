package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"lazypx/api"
	"lazypx/config"
)

// ── Root command structure ─────────────────────────────────────────────────

func TestRoot_ReturnsCommand(t *testing.T) {
	cmd := Root()
	if cmd == nil {
		t.Fatal("Root() returned nil")
	}
	if cmd.Use != "lazypx" {
		t.Errorf("expected Use='lazypx', got %q", cmd.Use)
	}
}

func TestRoot_HasSubcommands(t *testing.T) {
	cmd := Root()
	expected := []string{"vm", "node", "cluster", "snapshot", "access", "init-config", "version", "ssh"}
	for _, name := range expected {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestRoot_HasProfileFlag(t *testing.T) {
	cmd := Root()
	flag := cmd.PersistentFlags().Lookup("profile")
	if flag == nil {
		t.Fatal("expected --profile persistent flag")
	}
	if flag.Shorthand != "p" {
		t.Errorf("expected shorthand 'p', got %q", flag.Shorthand)
	}
}

// ── Version command ────────────────────────────────────────────────────────

func TestVersionCmd_PrintsVersion(t *testing.T) {
	// Reset global state
	cfgGlobal = nil

	cmd := newVersionCmd()
	if cmd.Use != "version" {
		t.Errorf("expected Use='version', got %q", cmd.Use)
	}

	// Capture stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd.Run(cmd, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "lazypx") {
		t.Error("version output should contain 'lazypx'")
	}
	if !strings.Contains(output, config.Version) {
		t.Errorf("version output should contain %q", config.Version)
	}
}

// ── clientFromConfig ───────────────────────────────────────────────────────

func TestClientFromConfig_NilConfig(t *testing.T) {
	cfgGlobal = nil
	_, err := clientFromConfig(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "no profile configured") {
		t.Errorf("expected 'no profile configured' error, got: %v", err)
	}
}

func TestClientFromConfig_NoActiveProfile(t *testing.T) {
	cfgGlobal = nil
	cfg := &config.Config{
		DefaultProfile: "default",
		Profiles:       []config.Profile{},
	}
	_, err := clientFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error when no active profile")
	}
	if !strings.Contains(err.Error(), "no profile configured") {
		t.Errorf("expected 'no profile configured' error, got: %v", err)
	}
}

func TestClientFromConfig_ValidConfig(t *testing.T) {
	cfgGlobal = nil
	cfg := &config.Config{
		ActiveProfile: &config.Profile{
			Name:        "test",
			Host:        "https://localhost:8006",
			TokenID:     "root@pam!test",
			TokenSecret: "secret",
			TLSInsecure: true,
		},
	}
	client, err := clientFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("client should not be nil")
	}
}

func TestClientFromConfig_GlobalConfigOverride(t *testing.T) {
	cfgGlobal = &config.Config{
		ActiveProfile: &config.Profile{
			Name:        "global",
			Host:        "https://global:8006",
			TokenID:     "root@pam!global",
			TokenSecret: "secret",
			TLSInsecure: true,
		},
	}
	// Pass nil cfg but cfgGlobal has an active profile
	client, err := clientFromConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("client should not be nil from global config")
	}
	cfgGlobal = nil // cleanup
}

// ── init-config command ────────────────────────────────────────────────────

func TestInitConfigCmd_Output(t *testing.T) {
	cfgGlobal = nil

	cmd := newInitConfigCmd()
	if cmd.Use != "init-config" {
		t.Errorf("expected Use='init-config', got %q", cmd.Use)
	}

	// Capture stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd.Run(cmd, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "lazypx configuration") {
		t.Error("init-config should output example config header")
	}
	if !strings.Contains(output, "default_profile") {
		t.Error("init-config should output default_profile field")
	}
	if !strings.Contains(output, "token_id") {
		t.Error("init-config should output token_id field")
	}
	if !strings.Contains(output, "token_secret") {
		t.Error("init-config should output token_secret field")
	}
}

// ── printVMs ───────────────────────────────────────────────────────────────

func TestPrintVMs_TableFormat(t *testing.T) {
	vms := []api.VMStatus{
		{VMID: 100, Name: "web", Status: "running", Node: "pve1", CPU: 0.05, MemUsed: 1073741824, MemTotal: 4294967296},
		{VMID: 101, Name: "db", Status: "stopped", Node: "pve1", CPU: 0, MemUsed: 0, MemTotal: 8589934592},
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := printVMs(vms, "table")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "VMID") {
		t.Error("table should contain header VMID")
	}
	if !strings.Contains(output, "100") {
		t.Error("table should contain VMID 100")
	}
	if !strings.Contains(output, "web") {
		t.Error("table should contain VM name 'web'")
	}
	if !strings.Contains(output, "running") {
		t.Error("table should contain status 'running'")
	}
	if !strings.Contains(output, "db") {
		t.Error("table should contain VM name 'db'")
	}
}

func TestPrintVMs_JSONFormat(t *testing.T) {
	vms := []api.VMStatus{
		{VMID: 100, Name: "web", Status: "running", Node: "pve1"},
	}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := printVMs(vms, "json")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify it's valid JSON
	var decoded []api.VMStatus
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(decoded))
	}
	if decoded[0].VMID != 100 {
		t.Errorf("expected VMID 100, got %d", decoded[0].VMID)
	}
}

func TestPrintVMs_EmptyList(t *testing.T) {
	vms := []api.VMStatus{}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := printVMs(vms, "table")

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should still have header
	if !strings.Contains(output, "VMID") {
		t.Error("empty table should still contain header")
	}
}

// ── printJSON ──────────────────────────────────────────────────────────────

func TestPrintJSON_Encoding(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	data := testStruct{Name: "hello", Value: 42}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := printJSON(data)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	var decoded testStruct
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Name != "hello" {
		t.Errorf("expected name='hello', got %q", decoded.Name)
	}
	if decoded.Value != 42 {
		t.Errorf("expected value=42, got %d", decoded.Value)
	}
}

func TestPrintJSON_Slice(t *testing.T) {
	data := []string{"a", "b", "c"}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := printJSON(data)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	var decoded []string
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded) != 3 {
		t.Errorf("expected 3 items, got %d", len(decoded))
	}
}

func TestPrintJSON_Indented(t *testing.T) {
	data := map[string]int{"x": 1}

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	printJSON(data)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "  ") {
		t.Error("JSON should be indented with spaces")
	}
}

// ── formatBytesCmd ─────────────────────────────────────────────────────────

func TestFormatBytesCmd(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1048576, "1.0MB"},
		{1073741824, "1.0GB"},
	}
	for _, tt := range tests {
		result := formatBytesCmd(tt.input)
		if result != tt.expected {
			t.Errorf("formatBytesCmd(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ── newVMCmd ───────────────────────────────────────────────────────────────

func TestNewVMCmd_Structure(t *testing.T) {
	cmd := newVMCmd(nil)
	if cmd.Use != "vm" {
		t.Errorf("expected Use='vm', got %q", cmd.Use)
	}
	expectedSubs := []string{"list", "start", "stop", "reboot"}
	for _, name := range expectedSubs {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("vm missing subcommand: %s", name)
		}
	}
}

func TestNewNodeCmd_Structure(t *testing.T) {
	cmd := newNodeCmd(nil)
	if cmd.Use != "node" {
		t.Errorf("expected Use='node', got %q", cmd.Use)
	}
	expectedSubs := []string{"list", "status"}
	for _, name := range expectedSubs {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("node missing subcommand: %s", name)
		}
	}
}

func TestNewClusterCmd_Structure(t *testing.T) {
	cmd := newClusterCmd(nil)
	if cmd.Use != "cluster" {
		t.Errorf("expected Use='cluster', got %q", cmd.Use)
	}
	expectedSubs := []string{"status", "resources"}
	for _, name := range expectedSubs {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("cluster missing subcommand: %s", name)
		}
	}
}

func TestNewSnapshotCmd_Structure(t *testing.T) {
	cmd := newSnapshotCmd(nil)
	if cmd.Use != "snapshot" {
		t.Errorf("expected Use='snapshot', got %q", cmd.Use)
	}
	expectedSubs := []string{"list", "create", "delete", "rollback"}
	for _, name := range expectedSubs {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("snapshot missing subcommand: %s", name)
		}
	}
}

func TestNewAccessCmd_Structure(t *testing.T) {
	cmd := newAccessCmd(nil)
	if cmd.Use != "access" {
		t.Errorf("expected Use='access', got %q", cmd.Use)
	}
	expectedSubs := []string{"user", "group", "role", "acl"}
	for _, name := range expectedSubs {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("access missing subcommand: %s", name)
		}
	}
}

func TestNewSSHCmd_Structure(t *testing.T) {
	cmd := newSSHCmd()
	if cmd.Use != "ssh <vmid|name>" {
		t.Errorf("expected Use='ssh <vmid|name>', got %q", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"100"}); err != nil {
		t.Errorf("ssh should accept exactly 1 arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("ssh should reject 0 args")
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("ssh should reject 2 args")
	}
}
