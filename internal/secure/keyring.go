package secure

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	keyring "github.com/zalando/go-keyring"
)

// PersistenceMode selects which token-storage tier to use.
type PersistenceMode string

const (
	// PersistenceAuto picks keyring if available, else passphrase, else none.
	PersistenceAuto PersistenceMode = "auto"
	// PersistenceKeyring forces use of the OS keyring.
	PersistenceKeyring PersistenceMode = "keyring"
	// PersistencePassphrase forces use of the encrypted file backend.
	PersistencePassphrase PersistenceMode = "passphrase"
	// PersistenceNone disables on-disk token storage.
	PersistenceNone PersistenceMode = "none"
)

// keyringService is the service name registered with the OS keyring.
const keyringService = "vault-tui"

// TokenStore persists per-identity Vault tokens. Tokens are keyed by
// address + auth method + identifier so that multiple identities per
// server do not overwrite each other.
type TokenStore interface {
	Mode() PersistenceMode
	Get(key string) (string, error)
	Set(key, token string) error
	Delete(key string) error
}

// ErrTokenNotFound is returned when a key has no stored token.
var ErrTokenNotFound = errors.New("secure: token not found")

// NewTokenStore picks a backend based on the requested mode and the
// supplied resources. passphraseKey may be nil, in which case the
// passphrase tier is unavailable; if mode is auto it will be skipped.
// tokensFilePath is the on-disk location for the encrypted file backend.
func NewTokenStore(mode PersistenceMode, passphraseKey *PassphraseKey, tokensFilePath string) (TokenStore, error) {
	switch mode {
	case PersistenceKeyring:
		if !keyringAvailable() {
			return nil, errors.New("secure: keyring not available on this platform")
		}
		return &keyringStore{}, nil
	case PersistencePassphrase:
		if passphraseKey == nil {
			return nil, errors.New("secure: passphrase mode selected but no passphrase key provided")
		}
		return newFileStore(passphraseKey, tokensFilePath)
	case PersistenceNone, "":
		return &nullStore{}, nil
	case PersistenceAuto:
		if keyringAvailable() {
			return &keyringStore{}, nil
		}
		if passphraseKey != nil {
			return newFileStore(passphraseKey, tokensFilePath)
		}
		return &nullStore{}, nil
	default:
		return nil, fmt.Errorf("secure: unknown persistence mode %q", mode)
	}
}

// TokenKey assembles the storage key for an address+method+identifier
// triple. Identifier may be empty (e.g. legacy callers) but should not be
// in normal usage.
func TokenKey(address, method, identifier string) string {
	return address + "|" + method + "|" + identifier
}

// keyringAvailable does a best-effort probe for whether the OS keyring is
// usable. On Linux without a Secret Service this will be false at runtime
// even though the platform would normally support it.
func keyringAvailable() bool {
	const probe = "vault-tui-probe"
	// Try to write+read+delete a tiny value. Any error means unavailable.
	if err := keyring.Set(keyringService, probe, "ok"); err != nil {
		return false
	}
	val, err := keyring.Get(keyringService, probe)
	_ = keyring.Delete(keyringService, probe)
	if err != nil || val != "ok" {
		return false
	}
	return true
}

// ---- keyring backend ----

type keyringStore struct{}

func (s *keyringStore) Mode() PersistenceMode { return PersistenceKeyring }

func (s *keyringStore) Get(key string) (string, error) {
	v, err := keyring.Get(keyringService, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrTokenNotFound
		}
		return "", fmt.Errorf("secure: keyring get: %w", err)
	}
	return v, nil
}

func (s *keyringStore) Set(key, token string) error {
	if err := keyring.Set(keyringService, key, token); err != nil {
		return fmt.Errorf("secure: keyring set: %w", err)
	}
	return nil
}

func (s *keyringStore) Delete(key string) error {
	if err := keyring.Delete(keyringService, key); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("secure: keyring delete: %w", err)
	}
	return nil
}

// ---- encrypted file backend ----

type fileStore struct {
	mu     sync.Mutex
	key    *PassphraseKey
	path   string
	cached map[string]string
	loaded bool
}

func newFileStore(key *PassphraseKey, path string) (*fileStore, error) {
	if path == "" {
		return nil, errors.New("secure: empty tokens file path")
	}
	return &fileStore{key: key, path: path, cached: map[string]string{}}, nil
}

func (s *fileStore) Mode() PersistenceMode { return PersistencePassphrase }

func (s *fileStore) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return "", err
	}
	v, ok := s.cached[key]
	if !ok {
		return "", ErrTokenNotFound
	}
	return v, nil
}

func (s *fileStore) Set(key, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return err
	}
	s.cached[key] = token
	return s.saveLocked()
}

func (s *fileStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return err
	}
	delete(s.cached, key)
	return s.saveLocked()
}

func (s *fileStore) loadLocked() error {
	if s.loaded {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.loaded = true
			return nil
		}
		return fmt.Errorf("secure: read tokens file: %w", err)
	}
	if len(data) == 0 {
		s.loaded = true
		return nil
	}
	pt, err := s.key.Decrypt(data)
	if err != nil {
		return fmt.Errorf("secure: decrypt tokens file: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(pt, &m); err != nil {
		// wipe pt then return error
		for i := range pt {
			pt[i] = 0
		}
		return fmt.Errorf("secure: decode tokens file: %w", err)
	}
	for i := range pt {
		pt[i] = 0
	}
	s.cached = m
	s.loaded = true
	return nil
}

func (s *fileStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("secure: create tokens dir: %w", err)
	}
	pt, err := json.Marshal(s.cached)
	if err != nil {
		return fmt.Errorf("secure: encode tokens file: %w", err)
	}
	defer func() {
		for i := range pt {
			pt[i] = 0
		}
	}()
	ct, err := s.key.Encrypt(pt)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, ct, 0o600); err != nil {
		return fmt.Errorf("secure: write tokens file: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmp, 0o600)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("secure: rename tokens file: %w", err)
	}
	return nil
}

// ---- null backend ----

type nullStore struct{}

func (n *nullStore) Mode() PersistenceMode      { return PersistenceNone }
func (n *nullStore) Get(string) (string, error) { return "", ErrTokenNotFound }
func (n *nullStore) Set(string, string) error   { return nil }
func (n *nullStore) Delete(string) error        { return nil }
