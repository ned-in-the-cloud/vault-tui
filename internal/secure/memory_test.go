package secure

import (
	"bytes"
	"testing"
)

func TestSecureStringRoundTrip(t *testing.T) {
	plaintext := []byte("hunter2")
	src := append([]byte(nil), plaintext...)
	s := NewSecureString(src)
	defer s.Destroy()

	// src should be wiped after construction.
	for i, b := range src {
		if b != 0 {
			t.Fatalf("source not wiped at %d: %v", i, src)
		}
	}

	var seen []byte
	if err := s.WithBytes(func(b []byte) error {
		seen = append(seen, b...)
		return nil
	}); err != nil {
		t.Fatalf("WithBytes: %v", err)
	}
	if !bytes.Equal(seen, plaintext) {
		t.Fatalf("got %q, want %q", seen, plaintext)
	}
}

func TestSecureStringEmpty(t *testing.T) {
	s := NewSecureString(nil)
	if !s.IsEmpty() {
		t.Fatal("empty SecureString reported non-empty")
	}
	if err := s.WithBytes(func(b []byte) error {
		if len(b) != 0 {
			t.Fatalf("expected empty bytes, got %v", b)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithBytes: %v", err)
	}
}

func TestSecureStringDestroyIsIdempotent(t *testing.T) {
	s := NewSecureString([]byte("x"))
	s.Destroy()
	s.Destroy() // should not panic
}

func TestPassphraseEncryptDecrypt(t *testing.T) {
	pw := []byte("correct horse battery staple")
	k, err := DeriveKeyFromPassphrase(pw, nil)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	defer k.Destroy()

	// pw must be wiped.
	for _, b := range pw {
		if b != 0 {
			t.Fatalf("passphrase not wiped: %v", pw)
		}
	}

	pt := []byte(`{"hello":"world"}`)
	ct, err := k.Encrypt(pt)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ct, pt) {
		t.Fatal("ciphertext contains plaintext")
	}

	got, err := k.Decrypt(ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("got %q, want %q", got, pt)
	}
}

func TestPassphraseWrongPassphraseFails(t *testing.T) {
	k1, err := DeriveKeyFromPassphrase([]byte("right"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer k1.Destroy()

	ct, err := k1.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	// Different salt -> different key, decrypt should fail with salt mismatch.
	k2, err := DeriveKeyFromPassphrase([]byte("wrong"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer k2.Destroy()

	if _, err := k2.Decrypt(ct); err == nil {
		t.Fatal("expected decrypt to fail with different salt")
	}
}
