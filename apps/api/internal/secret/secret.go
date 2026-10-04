// Package secret encrypts small values at rest, such as bank account numbers, with AES-256-GCM.
// Each value is bound to the row it belongs to, so a ciphertext copied to another row will not
// decrypt.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// Box seals and opens values with one key.
type Box struct {
	aead cipher.AEAD
}

// New returns a Box for a 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secret key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// FromPassphrase derives a key from text. For development and tests only; production keys are
// 32 random bytes.
func FromPassphrase(passphrase string) *Box {
	key := sha256.Sum256([]byte(passphrase))
	b, _ := New(key[:])
	return b
}

// Seal encrypts plaintext for the record identified by context.
func (b *Box) Seal(plaintext, context []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, plaintext, context), nil
}

// ErrOpen means a value could not be decrypted: the wrong key, the wrong record, or tampering.
var ErrOpen = errors.New("cannot decrypt value")

// Open decrypts a value sealed for the record identified by context.
func (b *Box) Open(sealed, context []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, ErrOpen
	}
	plain, err := b.aead.Open(nil, sealed[:n], sealed[n:], context)
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}
