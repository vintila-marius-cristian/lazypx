package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDir(t *testing.T) {
	dir := ConfigDir()
	if dir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}
	if !strings.HasSuffix(dir, filepath.Join(".config", "lazypx")) {
		t.Errorf("ConfigDir() = %q, expected to end with %q", dir, filepath.Join(".config", "lazypx"))
	}
}

func TestExampleConfig(t *testing.T) {
	yaml := ExampleConfig()
	if yaml == "" {
		t.Fatal("ExampleConfig() returned empty string")
	}

	// Verify key fields are present
	expected := []string{
		"default_profile:",
		"profiles:",
		"name: default",
		"host: https://192.168.1.10:8006",
		"token_id:",
		"tls_insecure:",
		"refresh_interval:",
		"production:",
	}
	for _, field := range expected {
		if !strings.Contains(yaml, field) {
			t.Errorf("ExampleConfig() missing expected field %q", field)
		}
	}

	// Verify at least two profiles
	if strings.Count(yaml, "- name: ") < 2 {
		t.Error("ExampleConfig() should contain at least two profile examples")
	}
}

func TestEnsureConfigDir(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Re-initialize configDir for this test
	origDir := configDir
	configDir = filepath.Join(tmpHome, ".config", "lazypx")
	defer func() { configDir = origDir }()

	if err := EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir() error: %v", err)
	}

	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("ConfigDir path is not a directory")
	}

	// Idempotent: calling again should not fail
	if err := EnsureConfigDir(); err != nil {
		t.Errorf("EnsureConfigDir() not idempotent: %v", err)
	}
}

func TestLoadNoConfigFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir := configDir
	configDir = filepath.Join(tmpHome, ".config", "lazypx")
	os.MkdirAll(configDir, 0700)
	defer func() { configDir = origDir }()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() with no config file should not error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.DefaultProfile != "" {
		t.Errorf("expected empty DefaultProfile, got %q", cfg.DefaultProfile)
	}
	if cfg.ActiveProfile != nil {
		t.Error("expected nil ActiveProfile when no config file exists")
	}
}

func TestLoadWithConfigFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir := configDir
	configDir = filepath.Join(tmpHome, ".config", "lazypx")
	os.MkdirAll(configDir, 0700)
	defer func() { configDir = origDir }()

	yaml := `
default_profile: prod

profiles:
  - name: default
    host: https://192.168.1.10:8006
    token_id: root@pam!lazypx
    token_secret: ""
    tls_insecure: false
    refresh_interval: 30
    production: false
  - name: prod
    host: https://pve-prod.example.com:8006
    token_id: root@pam!prod
    token_secret: ""
    tls_insecure: true
    refresh_interval: 60
    production: true
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatalf("writing config.yaml: %v", err)
	}

	t.Run("default profile", func(t *testing.T) {
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.DefaultProfile != "prod" {
			t.Errorf("expected default_profile=prod, got %q", cfg.DefaultProfile)
		}
		if cfg.ActiveProfile == nil {
			t.Fatal("ActiveProfile is nil")
		}
		if cfg.ActiveProfile.Name != "prod" {
			t.Errorf("expected active profile name=prod, got %q", cfg.ActiveProfile.Name)
		}
		if cfg.ActiveProfile.Host != "https://pve-prod.example.com:8006" {
			t.Errorf("unexpected host: %q", cfg.ActiveProfile.Host)
		}
		if cfg.ActiveProfile.RefreshInterval != 60 {
			t.Errorf("expected refresh_interval=60, got %d", cfg.ActiveProfile.RefreshInterval)
		}
		if !cfg.ActiveProfile.TLSInsecure {
			t.Error("expected TLSInsecure=true for prod")
		}
	})

	t.Run("explicit profile", func(t *testing.T) {
		cfg, err := Load("default")
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.ActiveProfile == nil {
			t.Fatal("ActiveProfile is nil")
		}
		if cfg.ActiveProfile.Name != "default" {
			t.Errorf("expected active profile name=default, got %q", cfg.ActiveProfile.Name)
		}
		if cfg.ActiveProfile.RefreshInterval != 30 {
			t.Errorf("expected refresh_interval=30, got %d", cfg.ActiveProfile.RefreshInterval)
		}
	})

	t.Run("nonexistent profile", func(t *testing.T) {
		cfg, err := Load("staging")
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.ActiveProfile != nil {
			t.Error("expected nil ActiveProfile for nonexistent profile name")
		}
	})
}

func TestLoadRefreshIntervalDefault(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir := configDir
	configDir = filepath.Join(tmpHome, ".config", "lazypx")
	os.MkdirAll(configDir, 0700)
	defer func() { configDir = origDir }()

	// Profile without refresh_interval should default to 1
	yaml := `
default_profile: default
profiles:
  - name: default
    host: https://localhost:8006
    token_id: root@pam!test
`
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0600)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ActiveProfile == nil {
		t.Fatal("ActiveProfile is nil")
	}
	if cfg.ActiveProfile.RefreshInterval != 1 {
		t.Errorf("expected default refresh_interval=1, got %d", cfg.ActiveProfile.RefreshInterval)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir := configDir
	configDir = filepath.Join(tmpHome, ".config", "lazypx")
	os.MkdirAll(configDir, 0700)
	defer func() { configDir = origDir }()

	// Write invalid YAML that viper will reject
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(":\n  : bad"), 0600)

	_, err := Load("")
	if err == nil {
		t.Error("expected error for invalid YAML config, got nil")
	}
}

func TestSSHLoadMissingFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	os.MkdirAll(filepath.Join(tmpHome, ".config", "lazypx"), 0700)

	hosts, err := LoadSSH()
	if err != nil {
		t.Fatalf("LoadSSH() with missing file should not error, got: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected empty map, got %d entries", len(hosts))
	}
}

func TestSSHLoadInvalidYAML(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".config", "lazypx")
	os.MkdirAll(configDir, 0700)
	os.WriteFile(filepath.Join(configDir, "ssh.yaml"), []byte("not: valid: yaml: [[["), 0600)

	_, err := LoadSSH()
	if err == nil {
		t.Error("expected error for invalid ssh.yaml, got nil")
	}
}

func TestSSHLoadEmptyFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".config", "lazypx")
	os.MkdirAll(configDir, 0700)
	os.WriteFile(filepath.Join(configDir, "ssh.yaml"), []byte(""), 0600)

	hosts, err := LoadSSH()
	if err != nil {
		t.Fatalf("LoadSSH() with empty file error: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected empty map for empty file, got %d", len(hosts))
	}
}

func TestSSHLoadPermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test as root")
	}
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".config", "lazypx")
	os.MkdirAll(configDir, 0700)
	path := filepath.Join(configDir, "ssh.yaml")
	os.WriteFile(path, []byte("- id: 1\n  user: root\n  host: 1.2.3.4"), 0000)

	_, err := LoadSSH()
	if err == nil {
		t.Error("expected error for unreadable ssh.yaml, got nil")
	}
}

func TestKeyringKey(t *testing.T) {
	tests := []struct {
		profile string
		want    string
	}{
		{"default", "lazypx:token_secret:default"},
		{"prod", "lazypx:token_secret:prod"},
		{"", "lazypx:token_secret:"},
		{"my@profile", "lazypx:token_secret:my@profile"},
	}
	for _, tt := range tests {
		got := keyringKey(tt.profile)
		if got != tt.want {
			t.Errorf("keyringKey(%q) = %q, want %q", tt.profile, got, tt.want)
		}
	}
}

func TestStoreSecret(t *testing.T) {
	err := StoreSecret("test-store-profile", "my-secret-token")
	if err != nil {
		t.Fatalf("StoreSecret() error: %v", err)
	}
	// Clean up
	_ = DeleteSecret("test-store-profile")
}

func TestLoadSecretSuccess(t *testing.T) {
	profile := "test-load-profile"
	expected := "super-secret-value"

	// Store first
	if err := StoreSecret(profile, expected); err != nil {
		t.Fatalf("StoreSecret() error: %v", err)
	}
	defer DeleteSecret(profile)

	got, err := LoadSecret(profile)
	if err != nil {
		t.Fatalf("LoadSecret() error: %v", err)
	}
	if got != expected {
		t.Errorf("LoadSecret() = %q, want %q", got, expected)
	}
}

func TestLoadSecretNotFound(t *testing.T) {
	// Use a profile that has never been stored
	got, err := LoadSecret("nonexistent-profile-12345")
	if err != nil {
		t.Fatalf("LoadSecret() for missing key should return empty string without error, got: %v", err)
	}
	if got != "" {
		t.Errorf("LoadSecret() for missing key = %q, want empty string", got)
	}
}

func TestDeleteSecretNotFound(t *testing.T) {
	// Deleting a nonexistent secret should not error
	err := DeleteSecret("nonexistent-profile-67890")
	if err != nil {
		t.Fatalf("DeleteSecret() for missing key should not error, got: %v", err)
	}
}

func TestDeleteSecretSuccess(t *testing.T) {
	profile := "test-delete-profile"
	if err := StoreSecret(profile, "value-to-delete"); err != nil {
		t.Fatalf("StoreSecret() error: %v", err)
	}

	err := DeleteSecret(profile)
	if err != nil {
		t.Fatalf("DeleteSecret() error: %v", err)
	}

	// Verify it's gone
	got, err := LoadSecret(profile)
	if err != nil {
		t.Fatalf("LoadSecret() after delete error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty after delete, got %q", got)
	}
}
