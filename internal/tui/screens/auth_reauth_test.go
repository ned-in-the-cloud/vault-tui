package screens

import (
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

type stubTokenStore struct{}

func (stubTokenStore) Mode() secure.PersistenceMode { return secure.PersistenceNone }
func (stubTokenStore) Get(string) (string, error)   { return "", secure.ErrTokenNotFound }
func (stubTokenStore) Set(string, string) error     { return nil }
func (stubTokenStore) Delete(string) error          { return nil }

func testAuthContext(t *testing.T) *tui.AppContext {
	t.Helper()
	cfg := &config.Config{}
	cfg.ServerHistory = []config.ServerEntry{{Address: "https://vault.example", Namespace: ""}}
	ctx := &tui.AppContext{
		Vault:      &vault.MockClient{},
		Tokens:     stubTokenStore{},
		Config:     cfg,
		ConfigPath: filepath.Join(t.TempDir(), "config.json"),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	ctx.Vault.SetAddress("https://vault.example")
	return ctx
}

func TestAuthSuccessReplacesScreenInNormalMode(t *testing.T) {
	ctx := testAuthContext(t)
	s := NewAuthScreen(ctx, tui.Theme{}, nil)
	auth, ok := s.(*AuthScreen)
	if !ok {
		t.Fatalf("screen type = %T, want *AuthScreen", s)
	}

	_, cmd := auth.Update(authSuccessMsg{
		methodID:   config.AuthMethodToken,
		methodPath: "token",
		identifier: "tester",
		tokenInfo:  &vault.TokenInfo{ID: "token", DisplayName: "tester"},
	})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(tui.ReplaceScreenMsg); !ok {
		t.Fatalf("message type = %T, want tui.ReplaceScreenMsg", msg)
	}
}

func TestAuthSuccessPopsScreenInReauthMode(t *testing.T) {
	ctx := testAuthContext(t)
	s := NewReauthScreen(ctx, tui.Theme{})
	auth := s.(*AuthScreen)

	_, cmd := auth.Update(authSuccessMsg{
		methodID:   config.AuthMethodToken,
		methodPath: "token",
		identifier: "tester",
		tokenInfo:  &vault.TokenInfo{ID: "token", DisplayName: "tester"},
	})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(tui.ReplaceScreenMsg); ok {
		t.Fatalf("message type = %T, did not expect tui.ReplaceScreenMsg", msg)
	}
	if reflect.TypeOf(msg).String() != "tea.sequenceMsg" {
		t.Fatalf("message type = %T, want tea.sequenceMsg", msg)
	}
}
