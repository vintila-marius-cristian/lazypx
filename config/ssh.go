package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SSHHost represents an entry in ssh.yaml.
type SSHHost struct {
	ID           int    `yaml:"id"`
	User         string `yaml:"user"`
	Host         string `yaml:"host"` // or IP
	Port         int    `yaml:"port,omitempty"`
	Password     string `yaml:"password,omitempty"`
	IdentityFile string `yaml:"identity_file,omitempty"`
}

// LoadSSH reads ~/.config/lazypx/ssh.yaml and returns a map of VMID -> SSHHost.
func LoadSSH() (map[int]SSHHost, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "lazypx", "ssh.yaml")

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[int]SSHHost), nil
		}
		return nil, fmt.Errorf("reading ssh.yaml: %w", err)
	}

	var list []SSHHost
	if err := yaml.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("parsing ssh.yaml: %w", err)
	}

	out := make(map[int]SSHHost)
	for _, h := range list {
		out[h.ID] = h
	}
	return out, nil
}
