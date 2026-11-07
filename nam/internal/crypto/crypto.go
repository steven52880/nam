// Package crypto encrypts upstream credentials with AES-256-GCM.
//
// The master key comes from NAM_SECRET and is never stored in nam.db, so a
// database backup alone cannot reveal credentials.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

const version byte = 1

// ErrNoSecret is returned when NAM_SECRET is empty.
var ErrNoSecret = errors.New("crypto: NAM_SECRET is empty")

// Box seals and opens credential blobs.
type Box struct {
	aead cipher.AEAD
}

// New derives a 256-bit key from the secret.
func New(secret string) (*Box, error) {
	if secret == "" {
		return nil, ErrNoSecret
	}
	sum := sha256.Sum256([]byte("nam-credential-v1\x00" + secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal returns version || nonce || ciphertext. Empty plaintext yields nil.
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+b.aead.Overhead())
	out = append(out, version)
	out = append(out, nonce...)
	return b.aead.Seal(out, nonce, plaintext, []byte{version}), nil
}

// Open reverses Seal. Nil input yields nil.
func (b *Box) Open(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, nil
	}
	ns := b.aead.NonceSize()
	if len(blob) < 1+ns+b.aead.Overhead() {
		return nil, errors.New("crypto: blob too short")
	}
	if blob[0] != version {
		return nil, fmt.Errorf("crypto: unknown version %d", blob[0])
	}
	nonce, ct := blob[1:1+ns], blob[1+ns:]
	pt, err := b.aead.Open(nil, nonce, ct, []byte{version})
	if err != nil {
		return nil, errors.New("crypto: decrypt failed (wrong NAM_SECRET?)")
	}
	return pt, nil
}
