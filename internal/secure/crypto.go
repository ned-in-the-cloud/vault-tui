package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

// PassphraseKey holds an Argon2id-derived 32-byte key inside a memguard
// enclave so that it can be reused across multiple encrypt/decrypt calls
// without sitting in plaintext memory.
type PassphraseKey struct {
	salt []byte
	key  *SecureString
}

// Argon2 parameters. These are the OWASP-recommended defaults for a
// password verification context; we use Argon2id for symmetric key
// derivation against an interactive passphrase.
const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024 // 64 MiB
	argon2Threads = 4
	argon2KeyLen  = 32
	saltLen       = 16
)

// ErrCiphertextTooShort is returned when Decrypt receives data that cannot
// possibly contain a valid nonce + ciphertext + tag.
var ErrCiphertextTooShort = errors.New("secure: ciphertext too short")

// DeriveKeyFromPassphrase runs Argon2id against the supplied passphrase
// and salt. If salt is nil a fresh 16-byte salt is generated. The
// passphrase is wiped before this function returns. The caller is
// responsible for persisting the returned salt alongside any ciphertext.
func DeriveKeyFromPassphrase(passphrase []byte, salt []byte) (*PassphraseKey, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("secure: empty passphrase")
	}
	if salt == nil {
		salt = make([]byte, saltLen)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return nil, fmt.Errorf("secure: generate salt: %w", err)
		}
	} else if len(salt) < 8 {
		return nil, errors.New("secure: salt too short")
	}

	key := argon2.IDKey(passphrase, salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	// wipe passphrase
	for i := range passphrase {
		passphrase[i] = 0
	}
	return &PassphraseKey{
		salt: append([]byte(nil), salt...),
		key:  NewSecureString(key),
	}, nil
}

// Salt returns the salt associated with this derived key.
func (k *PassphraseKey) Salt() []byte {
	return append([]byte(nil), k.salt...)
}

// Destroy zeroes the cached key.
func (k *PassphraseKey) Destroy() {
	if k == nil {
		return
	}
	k.key.Destroy()
	for i := range k.salt {
		k.salt[i] = 0
	}
	k.salt = nil
}

// Encrypt seals plaintext under AES-256-GCM. The output layout is
// salt || nonce || ciphertext || tag. The plaintext slice is not modified.
func (k *PassphraseKey) Encrypt(plaintext []byte) ([]byte, error) {
	if k == nil || k.key.IsEmpty() {
		return nil, errors.New("secure: nil passphrase key")
	}
	var out []byte
	err := k.key.WithBytes(func(rawKey []byte) error {
		block, err := aes.NewCipher(rawKey)
		if err != nil {
			return fmt.Errorf("secure: aes new cipher: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return fmt.Errorf("secure: gcm: %w", err)
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return fmt.Errorf("secure: nonce: %w", err)
		}
		ct := gcm.Seal(nil, nonce, plaintext, k.salt)
		buf := make([]byte, 0, len(k.salt)+len(nonce)+len(ct))
		buf = append(buf, k.salt...)
		buf = append(buf, nonce...)
		buf = append(buf, ct...)
		out = buf
		return nil
	})
	return out, err
}

// Decrypt opens a buffer produced by Encrypt. The leading salt is verified
// against the key's salt to catch passphrase/salt mismatches early.
func (k *PassphraseKey) Decrypt(blob []byte) ([]byte, error) {
	if k == nil || k.key.IsEmpty() {
		return nil, errors.New("secure: nil passphrase key")
	}
	if len(blob) < saltLen+12+16 {
		return nil, ErrCiphertextTooShort
	}
	gotSalt := blob[:saltLen]
	if !bytesEqual(gotSalt, k.salt) {
		return nil, errors.New("secure: salt mismatch (wrong passphrase?)")
	}
	var out []byte
	err := k.key.WithBytes(func(rawKey []byte) error {
		block, err := aes.NewCipher(rawKey)
		if err != nil {
			return fmt.Errorf("secure: aes new cipher: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return fmt.Errorf("secure: gcm: %w", err)
		}
		ns := gcm.NonceSize()
		if len(blob) < saltLen+ns {
			return ErrCiphertextTooShort
		}
		nonce := blob[saltLen : saltLen+ns]
		ct := blob[saltLen+ns:]
		pt, err := gcm.Open(nil, nonce, ct, k.salt)
		if err != nil {
			return fmt.Errorf("secure: gcm open: %w", err)
		}
		out = pt
		return nil
	})
	return out, err
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
