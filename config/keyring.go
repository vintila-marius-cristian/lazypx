package config

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "lazypx"

// keyringKey returns the keyring storage key for a given profile name.
func keyringKey(profile string) string {
	return fmt.Sprintf("lazypx:token_secret:%s", profile)
}

// StoreSecret saves a token secret in the OS keychain under the given profile name.
// This overwrites any existing secret for this profile.
func StoreSecret(profile, secret string) error {
	if err := keyring.Set(keyringService, keyringKey(profile), secret); err != nil {
		return fmt.Errorf("keyring store: %w", err)
	}
	return nil
}

// LoadSecret retrieves a token secret from the OS keychain.
// Returns ("", nil) if no secret is stored for this profile.
func LoadSecret(profile string) (string, error) {
	secret, err := keyring.Get(keyringService, keyringKey(profile))
	if err != nil {
		// If the secret simply doesn't exist, return empty without error.
		if errors.Is(err, keyring.ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("keyring load: %w", err)
	}
	return secret, nil
}

// DeleteSecret removes a stored secret from the OS keychain.
func DeleteSecret(profile string) error {
	if err := keyring.Delete(keyringService, keyringKey(profile)); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("keyring delete: %w", err)
	}
	return nil
}
