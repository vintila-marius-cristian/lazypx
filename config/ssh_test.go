package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSSH(t *testing.T) {
	// Create a temporary directory to act as home
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".config", "lazypx")
	err := os.MkdirAll(configDir, 0700)
	if err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	yamlContent := []byte(`
- id: 100
  user: root
  host: 192.168.1.100
  port: 2222
  password: my-secret-password
- id: 101
  user: admin
  host: 192.168.1.101
  identity_file: /path/to/key
`)

	err = os.WriteFile(filepath.Join(configDir, "ssh.yaml"), yamlContent, 0600)
	if err != nil {
		t.Fatalf("failed to write ssh.yaml: %v", err)
	}

	hosts, err := LoadSSH()
	if err != nil {
		t.Fatalf("LoadSSH failed: %v", err)
	}

	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(hosts))
	}

	h100, ok := hosts[100]
	if !ok {
		t.Fatalf("missing host 100")
	}
	if h100.User != "root" || h100.Host != "192.168.1.100" || h100.Port != 2222 || h100.Password != "my-secret-password" {
		t.Errorf("host 100 parsed incorrectly: %+v", h100)
	}

	h101, ok := hosts[101]
	if !ok {
		t.Fatalf("missing host 101")
	}
	if h101.User != "admin" || h101.IdentityFile != "/path/to/key" || h101.Password != "" {
		t.Errorf("host 101 parsed incorrectly: %+v", h101)
	}
}
