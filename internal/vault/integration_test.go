//go:build integration

package vault_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ned1313/vault-tui/internal/vault"
)

// TestIntegrationDevServer exercises the full connect+auth+lookup flow
// against a running `vault server -dev` instance. Run with:
//
//	go test -tags=integration ./internal/vault/...
//
// Required env:
//
//	VAULT_TUI_TEST_ADDR  (default http://127.0.0.1:8210)
//	VAULT_TUI_TEST_TOKEN (default root-test-token)
func TestIntegrationDevServer(t *testing.T) {
	addr := os.Getenv("VAULT_TUI_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8210"
	}
	tok := os.Getenv("VAULT_TUI_TEST_TOKEN")
	if tok == "" {
		tok = "root-test-token"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := vault.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.SetAddress(addr); err != nil {
		t.Fatalf("SetAddress: %v", err)
	}

	health, err := client.Health(ctx)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !health.Initialized {
		t.Fatalf("expected initialized, got %+v", health)
	}
	if health.Sealed {
		t.Fatalf("expected unsealed dev server, got %+v", health)
	}
	t.Logf("vault %s initialized=%v sealed=%v", health.Version, health.Initialized, health.Sealed)

	auth := vault.NewTokenAuth([]byte(tok))
	defer auth.Destroy()
	tokInfo, err := client.Login(ctx, auth)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokInfo == nil || tokInfo.ID == "" {
		t.Fatal("empty token info")
	}
	t.Logf("logged in as %s (accessor=%s policies=%v)",
		tokInfo.DisplayName, tokInfo.Accessor, tokInfo.Policies)

	// Lookup token to verify subsequent authenticated calls work.
	got, err := client.LookupToken(ctx)
	if err != nil {
		t.Fatalf("LookupToken: %v", err)
	}
	if got.Accessor == "" {
		t.Fatalf("missing accessor: %+v", got)
	}

	// List mounts (sys/mounts) — also requires authenticated client.
	mounts, err := client.ListMounts(ctx)
	if err != nil {
		t.Fatalf("ListMounts: %v", err)
	}
	if len(mounts) == 0 {
		t.Fatal("expected at least one mount")
	}
	t.Logf("mounts: %d", len(mounts))
}

// TestIntegrationUserPass exercises the userpass login path end-to-end.
// Requires `vault auth enable userpass` and a user `alice` with password
// `hunter2`. Run with -tags=integration.
func TestIntegrationUserPass(t *testing.T) {
	addr := os.Getenv("VAULT_TUI_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8210"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := vault.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.SetAddress(addr); err != nil {
		t.Fatalf("SetAddress: %v", err)
	}

	auth := vault.NewUserPassAuth("userpass", "alice", []byte("hunter2"))
	defer auth.Destroy()
	tokInfo, err := client.Login(ctx, auth)
	if err != nil {
		t.Fatalf("UserPass Login: %v", err)
	}
	if tokInfo.ID == "" {
		t.Fatal("empty token id")
	}
	t.Logf("userpass logged in as %s policies=%v ttl=%s",
		tokInfo.DisplayName, tokInfo.Policies, tokInfo.TTL)

	if _, err := client.LookupToken(ctx); err != nil {
		t.Fatalf("LookupToken after userpass: %v", err)
	}
}
