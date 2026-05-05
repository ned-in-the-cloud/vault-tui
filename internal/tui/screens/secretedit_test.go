package screens

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

type fakeEditKVv2 struct {
	putErr error
}

func (f *fakeEditKVv2) Version() int  { return 2 }
func (f *fakeEditKVv2) Mount() string { return "secret" }
func (f *fakeEditKVv2) List(context.Context, string) ([]vault.KVEntry, error) {
	return nil, nil
}
func (f *fakeEditKVv2) Get(context.Context, string) (*vault.KVSecret, error) { return nil, nil }
func (f *fakeEditKVv2) Put(context.Context, string, map[string]interface{}) error {
	return nil
}
func (f *fakeEditKVv2) Delete(context.Context, string) error { return nil }
func (f *fakeEditKVv2) GetVersion(context.Context, string, int) (*vault.KVSecret, error) {
	return nil, nil
}
func (f *fakeEditKVv2) GetVersionsList(context.Context, string) ([]vault.KVVersionMeta, error) {
	return nil, nil
}
func (f *fakeEditKVv2) GetMetadata(context.Context, string) (*vault.KVMetadata, error) {
	return nil, nil
}
func (f *fakeEditKVv2) GetSubkeys(context.Context, string, int, int) ([]string, error) {
	return nil, nil
}
func (f *fakeEditKVv2) PutCAS(context.Context, string, map[string]interface{}, *int) error {
	return f.putErr
}
func (f *fakeEditKVv2) Patch(context.Context, string, map[string]interface{}, *int) error {
	return f.putErr
}
func (f *fakeEditKVv2) DeleteVersions(context.Context, string, []int) error   { return nil }
func (f *fakeEditKVv2) UndeleteVersions(context.Context, string, []int) error { return nil }
func (f *fakeEditKVv2) DestroyVersions(context.Context, string, []int) error  { return nil }
func (f *fakeEditKVv2) DeleteAllVersions(context.Context, string) error       { return nil }

func editTestContext(kv vault.KVv2Engine) *tui.AppContext {
	cfg := &config.Config{}
	ctx := &tui.AppContext{
		Config:     cfg,
		ConfigPath: filepath.Join(".", "test-config.json"),
		Tokens:     stubTokenStore{},
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Vault: &vault.MockClient{KVv2Fn: func(string) vault.KVv2Engine {
			return kv
		}},
	}
	ctx.Vault.SetAddress("https://vault.example")
	return ctx
}

func TestSecretEditPreservesValuesOnInvalidTokenSaveFailure(t *testing.T) {
	ctx := editTestContext(&fakeEditKVv2{putErr: vault.ErrInvalidToken})
	s := NewSecretEditScreen(ctx, tui.DefaultTheme(), &vault.MountInfo{Path: "secret/"}, 2, "app/config", nil, false)
	edit, ok := s.(*SecretEditScreen)
	if !ok {
		t.Fatalf("screen type = %T", s)
	}

	edit.pairs[0].key.SetValue("username")
	edit.pairs[0].value.SetValue("alice")

	_, submitCmd := edit.submit()
	if submitCmd == nil {
		t.Fatalf("expected submit cmd")
	}
	submitMsg := submitCmd()
	op, ok := submitMsg.(opCompletedMsg)
	if !ok {
		t.Fatalf("expected opCompletedMsg, got %T", submitMsg)
	}
	if op.err == nil {
		t.Fatalf("expected save error")
	}

	_, updateCmd := edit.Update(op)
	if updateCmd == nil {
		t.Fatalf("expected reauth cmd")
	}
	if got := edit.pairs[0].value.Value(); got != "alice" {
		t.Fatalf("value lost across invalid-token flow: got %q", got)
	}

	msg := updateCmd()
	if msg == nil {
		t.Fatalf("expected non-nil reauth message")
	}
}
