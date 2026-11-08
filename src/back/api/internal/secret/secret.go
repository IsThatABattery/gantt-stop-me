// Package secret encrypts stored credentials (GitHub tokens, provider keys) with AES-256-GCM.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// Box seals and opens values with one key. Each sealed value carries its own random nonce.
type Box struct{ aead cipher.AEAD }

// New returns a Box for a 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, errors.New("secret: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead}, nil
}

// Seal encrypts plain.
func (b *Box) Seal(plain []byte) []byte { return b.aead.Seal(nil, nil, plain, nil) }

// Open decrypts a value from Seal, failing if it was altered or sealed with another key.
func (b *Box) Open(sealed []byte) ([]byte, error) { return b.aead.Open(nil, nil, sealed, nil) }
