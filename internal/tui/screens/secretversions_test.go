package screens

import (
	"context"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

type fakeVersionsKVv2 struct {
	getVersionResult   *vault.KVSecret
	getVersionErr      error
	getVersionCalled   int
	getVersionArg      int
	deleteAllCalled    bool
	destroyCalled      bool
	destroyArgVersions []int
	putCASCalled       bool
	putCASPath         string
	putCASData         map[string]interface{}
}

func (f *fakeVersionsKVv2) Version() int  { return 2 }
func (f *fakeVersionsKVv2) Mount() string { return "secret" }
func (f *fakeVersionsKVv2) List(context.Context, string) ([]vault.KVEntry, error) {
	return nil, nil
}
func (f *fakeVersionsKVv2) Get(context.Context, string) (*vault.KVSecret, error) {
	return nil, nil
}
func (f *fakeVersionsKVv2) Put(context.Context, string, map[string]interface{}) error { return nil }
func (f *fakeVersionsKVv2) Delete(context.Context, string) error                      { return nil }
func (f *fakeVersionsKVv2) GetVersion(_ context.Context, _ string, version int) (*vault.KVSecret, error) {
	f.getVersionCalled++
	f.getVersionArg = version
	return f.getVersionResult, f.getVersionErr
}
func (f *fakeVersionsKVv2) GetVersionsList(context.Context, string) ([]vault.KVVersionMeta, error) {
	return nil, nil
}
func (f *fakeVersionsKVv2) GetMetadata(context.Context, string) (*vault.KVMetadata, error) {
	return nil, nil
}
func (f *fakeVersionsKVv2) GetSubkeys(context.Context, string, int, int) ([]string, error) {
	return nil, nil
}
func (f *fakeVersionsKVv2) PutCAS(_ context.Context, path string, data map[string]interface{}, _ *int) error {
	f.putCASCalled = true
	f.putCASPath = path
	f.putCASData = data
	return nil
}
func (f *fakeVersionsKVv2) Patch(context.Context, string, map[string]interface{}, *int) error {
	return nil
}
func (f *fakeVersionsKVv2) DeleteVersions(context.Context, string, []int) error   { return nil }
func (f *fakeVersionsKVv2) UndeleteVersions(context.Context, string, []int) error { return nil }
func (f *fakeVersionsKVv2) DestroyVersions(_ context.Context, _ string, versions []int) error {
	f.destroyCalled = true
	f.destroyArgVersions = append([]int(nil), versions...)
	return nil
}
func (f *fakeVersionsKVv2) DeleteAllVersions(context.Context, string) error {
	f.deleteAllCalled = true
	return nil
}

func testVersionsScreen(kv vault.KVv2Engine, cfg *config.Config) *SecretVersionsScreen {
	if cfg == nil {
		cfg = &config.Config{}
	}
	ctx := &tui.AppContext{
		Config: cfg,
		Vault: &vault.MockClient{KVv2Fn: func(string) vault.KVv2Engine {
			return kv
		}},
	}
	return &SecretVersionsScreen{
		ctx:   ctx,
		theme: tui.DefaultTheme(),
		mount: &vault.MountInfo{Path: "secret/"},
		path:  "app/config",
	}
}

func TestSecretVersionsRollbackCmdCopiesSelectedVersion(t *testing.T) {
	sec := &vault.KVSecret{Data: map[string]*secure.SecureString{
		"username": secure.NewSecureString([]byte(`"alice"`)),
		"port":     secure.NewSecureString([]byte(`5432`)),
	}}
	fakeKV := &fakeVersionsKVv2{getVersionResult: sec}
	s := testVersionsScreen(fakeKV, nil)

	msg := s.rollbackCmd(7)()
	if fakeKV.getVersionCalled != 1 || fakeKV.getVersionArg != 7 {
		t.Fatalf("GetVersion calls = %d arg=%d", fakeKV.getVersionCalled, fakeKV.getVersionArg)
	}
	if !fakeKV.putCASCalled {
		t.Fatalf("expected PutCAS to be called")
	}
	if fakeKV.putCASPath != "app/config" {
		t.Fatalf("PutCAS path = %q", fakeKV.putCASPath)
	}
	if got := fakeKV.putCASData["username"]; got != "alice" {
		t.Fatalf("username = %v", got)
	}
	if got := fakeKV.putCASData["port"]; got != float64(5432) {
		t.Fatalf("port = %v", got)
	}
	if sec.Data["username"] != nil && !sec.Data["username"].IsEmpty() {
		t.Fatalf("expected secure value to be destroyed")
	}
	if done, ok := msg.(opCompletedMsg); !ok || done.err != nil {
		t.Fatalf("expected successful opCompletedMsg, got %T (%+v)", msg, msg)
	}
}

func TestSecretVersionsDeleteAllBypassWhenSkipped(t *testing.T) {
	cfg := &config.Config{}
	cfg.ConfirmationPrefs.SkipDeleteAll = true
	fakeKV := &fakeVersionsKVv2{}
	s := testVersionsScreen(fakeKV, cfg)

	_, cmd := s.handleKey("a")
	if cmd == nil {
		t.Fatalf("expected command")
	}
	msg := cmd()
	if !fakeKV.deleteAllCalled {
		t.Fatalf("expected DeleteAllVersions call")
	}
	if done, ok := msg.(opCompletedMsg); !ok || done.err != nil {
		t.Fatalf("expected successful opCompletedMsg, got %T", msg)
	}
}

func TestSecretVersionsDestroyAllAlwaysPrompts(t *testing.T) {
	cfg := &config.Config{}
	cfg.ConfirmationPrefs.SkipDestroyAll = true
	fakeKV := &fakeVersionsKVv2{}
	s := testVersionsScreen(fakeKV, cfg)

	_, cmd := s.handleKey("A")
	if cmd == nil {
		t.Fatalf("expected command")
	}
	msg := cmd()
	if _, ok := msg.(tui.PushScreenMsg); !ok {
		t.Fatalf("expected PushScreenMsg for destructive flow, got %T", msg)
	}
}

func TestSecretVersionsDestroyAllCmdUsesNonDestroyedVersions(t *testing.T) {
	fakeKV := &fakeVersionsKVv2{}
	s := testVersionsScreen(fakeKV, nil)
	s.versions = []vault.KVVersionMeta{
		{Version: 9, Destroyed: true},
		{Version: 8, Destroyed: false},
		{Version: 7, Destroyed: false},
	}

	msg := s.destroyAllCmd()()
	if !fakeKV.destroyCalled {
		t.Fatalf("expected DestroyVersions call")
	}
	want := []int{8, 7}
	if !reflect.DeepEqual(fakeKV.destroyArgVersions, want) {
		t.Fatalf("destroy versions = %v, want %v", fakeKV.destroyArgVersions, want)
	}
	if done, ok := msg.(opCompletedMsg); !ok || done.err != nil {
		t.Fatalf("expected successful opCompletedMsg, got %T", msg)
	}
}

func TestSecretVersionsRollbackDisallowsDestroyedSelection(t *testing.T) {
	s := testVersionsScreen(&fakeVersionsKVv2{}, nil)
	s.versions = []vault.KVVersionMeta{{Version: 3, Destroyed: true}}
	s.idx = 0

	_, cmd := s.handleKey("r")
	if cmd != nil {
		t.Fatalf("expected no command for destroyed rollback")
	}
	if s.statusMsg == "" {
		t.Fatalf("expected status message")
	}
}

var _ tea.Msg = opCompletedMsg{}
