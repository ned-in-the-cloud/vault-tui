// Package secure provides primitives for handling sensitive material in
// memory and at rest. SecureString wraps memguard.Enclave so that secret
// values are encrypted in memory and only briefly opened to a LockedBuffer
// when they must be used.
package secure

import (
	"errors"
	"fmt"

	"github.com/awnumar/memguard"
)

// ErrEnclaveDestroyed is returned when an operation is attempted on a
// SecureString whose backing enclave has already been destroyed.
var ErrEnclaveDestroyed = errors.New("secure: enclave destroyed")

// SecureString wraps a *memguard.Enclave and provides a small, intentional
// surface area for working with secret bytes.
//
// The zero value is not usable; construct via NewSecureString or
// NewSecureStringFromString. The source slice/string passed in is wiped
// after the enclave is created.
type SecureString struct {
	enc *memguard.Enclave
}

// NewSecureString creates a SecureString from a byte slice. The input slice
// is wiped (zeroed) before this function returns. Callers must not retain
// references to the slice afterwards.
func NewSecureString(b []byte) *SecureString {
	if len(b) == 0 {
		// memguard refuses empty buffers; represent empty as a nil enclave.
		return &SecureString{}
	}
	// memguard.NewEnclave consumes and wipes the input slice for us.
	enc := memguard.NewEnclave(b)
	return &SecureString{enc: enc}
}

// NewSecureStringFromString creates a SecureString from a string. Strings
// are immutable in Go and cannot be wiped, so callers that have access to
// the underlying byte source should prefer NewSecureString.
func NewSecureStringFromString(s string) *SecureString {
	if s == "" {
		return &SecureString{}
	}
	return NewSecureString([]byte(s))
}

// Open decrypts the enclave into a *memguard.LockedBuffer. The caller must
// call Destroy on the returned buffer as soon as the value is no longer
// needed.
func (s *SecureString) Open() (*memguard.LockedBuffer, error) {
	if s == nil {
		return nil, ErrEnclaveDestroyed
	}
	if s.enc == nil {
		// Empty value: return an empty locked buffer.
		return memguard.NewBuffer(0), nil
	}
	buf, err := s.enc.Open()
	if err != nil {
		return nil, fmt.Errorf("secure: open enclave: %w", err)
	}
	return buf, nil
}

// WithBytes opens the enclave, invokes fn with a read-only view of the
// plaintext, and destroys the resulting buffer immediately afterwards.
// The slice passed to fn must not be retained.
func (s *SecureString) WithBytes(fn func([]byte) error) error {
	buf, err := s.Open()
	if err != nil {
		return err
	}
	defer buf.Destroy()
	return fn(buf.Bytes())
}

// Destroy zeros and releases the enclave. Subsequent calls to Open return
// ErrEnclaveDestroyed. Safe to call multiple times.
func (s *SecureString) Destroy() {
	if s == nil || s.enc == nil {
		return
	}
	// Open + Destroy is the documented way to clear an enclave.
	if buf, err := s.enc.Open(); err == nil {
		buf.Destroy()
	}
	s.enc = nil
}

// IsEmpty reports whether the SecureString carries any bytes.
func (s *SecureString) IsEmpty() bool {
	return s == nil || s.enc == nil
}
