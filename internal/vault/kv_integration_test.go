//go:build integration

package vault_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/vault"
)

// authedClient logs in to the integration dev server with the test token.
func authedClient(t *testing.T, ctx context.Context) vault.Client {
	t.Helper()
	addr := os.Getenv("VAULT_TUI_TEST_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8210"
	}
	tok := os.Getenv("VAULT_TUI_TEST_TOKEN")
	if tok == "" {
		tok = "root-test-token"
	}
	c, err := vault.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.SetAddress(addr); err != nil {
		t.Fatalf("SetAddress: %v", err)
	}
	a := vault.NewTokenAuth([]byte(tok))
	defer a.Destroy()
	if _, err := c.Login(ctx, a); err != nil {
		t.Fatalf("Login: %v", err)
	}
	return c
}

func decodeString(t *testing.T, ss *secure.SecureString) string {
	t.Helper()
	var s string
	if err := ss.WithBytes(func(b []byte) error {
		return json.Unmarshal(b, &s)
	}); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

func TestIntegrationKVv1CRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := authedClient(t, ctx)

	mount := "kv-v1"
	mounts, err := c.ListMounts(ctx)
	if err != nil {
		t.Fatalf("ListMounts: %v", err)
	}
	if _, ok := mounts[mount+"/"]; !ok {
		t.Skipf("mount %q not present; run scripts/seed-dev.ps1 first", mount)
	}

	e := c.KVv1(mount)
	if e.Version() != 1 {
		t.Fatalf("Version = %d, want 1", e.Version())
	}

	path := "vault-tui-itest/v1secret"
	defer e.Delete(ctx, path)

	if err := e.Put(ctx, path, map[string]interface{}{
		"username": "alice",
		"password": "tacos",
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := e.Get(ctx, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer got.Destroy()
	if u := decodeString(t, got.Data["username"]); u != "alice" {
		t.Errorf("username = %q", u)
	}
	if p := decodeString(t, got.Data["password"]); p != "tacos" {
		t.Errorf("password = %q", p)
	}

	entries, err := e.List(ctx, "vault-tui-itest")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected list entries")
	}
}

func TestIntegrationKVv2CRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := authedClient(t, ctx)

	mount := "kv-v2"
	mounts, err := c.ListMounts(ctx)
	if err != nil {
		t.Fatalf("ListMounts: %v", err)
	}
	if _, ok := mounts[mount+"/"]; !ok {
		t.Skipf("mount %q not present; run scripts/seed-dev.ps1 first", mount)
	}

	e := c.KVv2(mount)
	if e.Version() != 2 {
		t.Fatalf("Version = %d", e.Version())
	}

	path := "vault-tui-itest/v2secret"
	defer e.DeleteAllVersions(ctx, path)

	// Two versions to exercise version metadata.
	if err := e.Put(ctx, path, map[string]interface{}{"k": "v1"}); err != nil {
		t.Fatalf("Put v1: %v", err)
	}
	if err := e.Put(ctx, path, map[string]interface{}{"k": "v2"}); err != nil {
		t.Fatalf("Put v2: %v", err)
	}

	got, err := e.Get(ctx, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer got.Destroy()
	if got.VersionMetadata == nil || got.VersionMetadata.Version != 2 {
		t.Errorf("version meta = %+v", got.VersionMetadata)
	}
	if v := decodeString(t, got.Data["k"]); v != "v2" {
		t.Errorf("k = %q", v)
	}

	v1, err := e.GetVersion(ctx, path, 1)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	defer v1.Destroy()
	if v := decodeString(t, v1.Data["k"]); v != "v1" {
		t.Errorf("v1 k = %q", v)
	}

	versions, err := e.GetVersionsList(ctx, path)
	if err != nil {
		t.Fatalf("GetVersionsList: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("versions = %d", len(versions))
	}

	md, err := e.GetMetadata(ctx, path)
	if err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}
	if md.CurrentVersion != 2 {
		t.Errorf("current = %d", md.CurrentVersion)
	}

	subkeys, err := e.GetSubkeys(ctx, path, 0, 0)
	if err != nil {
		t.Fatalf("GetSubkeys: %v", err)
	}
	sort.Strings(subkeys)
	if !equalStringSlices(subkeys, []string{"k"}) {
		t.Errorf("subkeys = %v", subkeys)
	}

	// PutFromSecure round-trip.
	in := map[string]*secure.SecureString{
		"k": secure.NewSecureString([]byte(`"secured"`)),
	}
	if err := vault.PutFromSecure(ctx, e, path, in); err != nil {
		t.Fatalf("PutFromSecure: %v", err)
	}
	got2, err := e.Get(ctx, path)
	if err != nil {
		t.Fatalf("Get after PutFromSecure: %v", err)
	}
	defer got2.Destroy()
	if v := decodeString(t, got2.Data["k"]); v != "secured" {
		t.Errorf("after secure put k = %q", v)
	}

	// Soft-delete v1 then undelete.
	if err := e.DeleteVersions(ctx, path, []int{1}); err != nil {
		t.Fatalf("DeleteVersions: %v", err)
	}
	if err := e.UndeleteVersions(ctx, path, []int{1}); err != nil {
		t.Fatalf("UndeleteVersions: %v", err)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// silence unused import warnings on builds where bytes is not yet needed
var _ = bytes.NewBuffer
