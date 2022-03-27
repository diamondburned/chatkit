package secret

import (
	"context"
	"errors"

	"github.com/diamondburned/gotkit/app"
	"github.com/zalando/go-keyring"
)

// Keyring is an implementation of a secret driver using the system's keyring
// driver.
type Keyring struct {
	id string
}

var ErrUnsupportedPlatform = keyring.ErrUnsupportedPlatform

var _ Driver = (*Keyring)(nil)

// KeyringDriver creates a new keyring driver.
func KeyringDriver(ctx context.Context) *Keyring {
	return &Keyring{
		id: app.FromContext(ctx).IDDot("secrets"),
	}
}

// IsAvailable returns true if the keyring API is available.
func (k *Keyring) IsAvailable() bool {
	return keyring.Set(k.id, "__secret_available_000", "") == nil
}

// Set sets the key.
func (k *Keyring) Set(key string, value []byte) error {
	return keyring.Set(k.id, key, string(value))
}

// Get gets the key.
func (k *Keyring) Get(key string) ([]byte, error) {
	v, err := keyring.Get(k.id, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return []byte(v), nil
}
