package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Profile holds a single Proxmox connection profile.
type Profile struct {
	Name            string `mapstructure:"name"`
	Host            string `mapstructure:"host"`
	TokenID         string `mapstructure:"token_id"`
	TokenSecret     string `mapstructure:"token_secret"`
	TLSInsecure     bool   `mapstructure:"tls_insecure"`
	RefreshInterval int    `mapstructure:"refresh_interval"` // seconds, default 1
	Production      bool   `mapstructure:"production"`       // show red indicator in header
}

// Config is the top-level configuration structure.
type Config struct {
	DefaultProfile string    `mapstructure:"default_profile"`
	Profiles       []Profile `mapstructure:"profiles"`
	ActiveProfile  *Profile  // resolved at load time
}

var configDir string

func init() {
	home, _ := os.UserHomeDir()
	configDir = filepath.Join(home, ".config", "lazypx")
}

// ConfigDir returns the path to the lazypx config directory.
func ConfigDir() string {
	return configDir
}

// Load reads and parses the config file, resolving the active profile.
// If profile is empty, the default_profile name is used.
func Load(profile string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Sensible defaults
	viper.SetDefault("default_profile", "default")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// No config file — return empty config so the app can show a setup hint.
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Resolve active profile
	name := profile
	if name == "" {
		name = cfg.DefaultProfile
	}
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == name {
			p := cfg.Profiles[i]
			cfg.ActiveProfile = &p
			break
		}
	}

	// Apply defaults to active profile
	if cfg.ActiveProfile != nil && cfg.ActiveProfile.RefreshInterval == 0 {
		cfg.ActiveProfile.RefreshInterval = 1
	}

	return &cfg, nil
}

// ExampleConfig returns a YAML string the user can paste as a starting config.
func ExampleConfig() string {
	return `# lazypx configuration
# Save this to ~/.config/lazypx/config.yaml

default_profile: default

profiles:
  - name: default
    host: https://192.168.1.10:8006
    token_id: root@pam!lazypx
    token_secret: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    tls_insecure: false
    refresh_interval: 1
    production: false

  - name: prod
    host: https://pve-prod.example.com:8006
    token_id: root@pam!lazypx
    token_secret: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    tls_insecure: false
    refresh_interval: 15
    production: true
`
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() error {
	return os.MkdirAll(configDir, 0700)
}
