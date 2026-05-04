package secure

import (
	"path/filepath"
	"testing"
)

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	tokensPath := filepath.Join(dir, "tokens.enc")

	k, err := DeriveKeyFromPassphrase([]byte("phrase"), nil)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	defer k.Destroy()

	store, err := NewTokenStore(PersistencePassphrase, k, tokensPath)
	if err != nil {
		t.Fatalf("NewTokenStore: %v", err)
	}

	if _, err := store.Get("missing"); err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}

	if err := store.Set("a|token|alice", "hvs.AAAA"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := store.Get("a|token|alice")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "hvs.AAAA" {
		t.Fatalf("got %q", got)
	}

	// New store with same key should read the same value.
	k2, err := DeriveKeyFromPassphrase([]byte("phrase"), k.Salt())
	if err != nil {
		t.Fatalf("re-derive: %v", err)
	}
	defer k2.Destroy()
	store2, err := NewTokenStore(PersistencePassphrase, k2, tokensPath)
	if err != nil {
		t.Fatal(err)
	}
	got2, err := store2.Get("a|token|alice")
	if err != nil {
		t.Fatalf("re-get: %v", err)
	}
	if got2 != "hvs.AAAA" {
		t.Fatalf("re-get got %q", got2)
	}

	if err := store2.Delete("a|token|alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := store2.Get("a|token|alice"); err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestNullStore(t *testing.T) {
	s, err := NewTokenStore(PersistenceNone, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Mode() != PersistenceNone {
		t.Fatalf("mode: %s", s.Mode())
	}
	if err := s.Set("k", "v"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := s.Get("k"); err != ErrTokenNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestTokenKeyFormat(t *testing.T) {
	got := TokenKey("https://vault:8200", "userpass", "alice")
	want := "https://vault:8200|userpass|alice"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
