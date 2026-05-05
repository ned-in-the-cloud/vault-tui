package screens

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

func testSecretListScreen() *SecretListScreen {
	ctx := &tui.AppContext{
		Config: &config.Config{},
		Vault:  &vault.MockClient{},
	}
	return NewSecretListScreen(ctx, tui.DefaultTheme(), &vault.MountInfo{Path: "secret/"}, 2, "team").(*SecretListScreen)
}

func TestSecretListKeyPShowsDirectPathPrompt(t *testing.T) {
	s := testSecretListScreen()
	_, cmd := s.handleKey("p")
	if cmd == nil {
		t.Fatalf("expected command")
	}
	msg := cmd()
	push, ok := msg.(tui.PushScreenMsg)
	if !ok {
		t.Fatalf("expected PushScreenMsg, got %T", msg)
	}
	if push.Screen == nil || push.Screen.Title() != "Direct path" {
		t.Fatalf("expected direct path prompt screen")
	}
}

func TestSecretListPermissionDeniedTriggersRecoveryFlow(t *testing.T) {
	s := testSecretListScreen()
	s.keys = []string{"a/", "b"}
	s.filtered = []string{"a/", "b"}
	s.canCreate = true

	_, cmd := s.Update(keysLoadedMsg{err: vault.ErrPermissionDenied})
	if cmd == nil {
		t.Fatalf("expected recovery command")
	}
	if s.loading {
		t.Fatalf("expected loading false")
	}
	if s.canCreate {
		t.Fatalf("expected create capability reset on denied load")
	}
	if len(s.keys) != 0 || len(s.filtered) != 0 {
		t.Fatalf("expected key lists cleared on denied load")
	}
	if msg := cmd(); msg == nil {
		t.Fatalf("expected non-nil recovery message")
	}
}

func TestSecretDirectPathPromptEnterPushesPathScreen(t *testing.T) {
	ctx := &tui.AppContext{Config: &config.Config{}, Vault: &vault.MockClient{}}
	s := NewSecretDirectPathPrompt(ctx, tui.DefaultTheme(), &vault.MountInfo{Path: "secret/"}, 2, "team")
	prompt := s.(*secretDirectPathPrompt)
	prompt.input.SetValue("team/prod/api")

	_, cmd := prompt.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected command")
	}
	if msg := cmd(); msg == nil {
		t.Fatalf("expected navigation message")
	}
}

func TestSecretDirectPathPromptEscPops(t *testing.T) {
	ctx := &tui.AppContext{Config: &config.Config{}, Vault: &vault.MockClient{}}
	s := NewSecretDirectPathPrompt(ctx, tui.DefaultTheme(), &vault.MountInfo{Path: "secret/"}, 2, "team")
	prompt := s.(*secretDirectPathPrompt)

	_, cmd := prompt.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatalf("expected command")
	}
	if _, ok := cmd().(tui.PopScreenMsg); !ok {
		t.Fatalf("expected PopScreenMsg")
	}
}
