package screens

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/tui/components"
)

func TestPushConfirmBypassesNormalWhenSkipped(t *testing.T) {
	cfg := &config.Config{}
	cfg.ConfirmationPrefs.SkipDeleteVersion = true
	ctx := &tui.AppContext{Config: cfg}
	called := false
	cmd := PushConfirm(ctx, tui.Theme{}, ConfirmRequest{
		Level:    components.DangerNormal,
		Category: ConfirmDeleteVersion,
		OnConfirm: func() tea.Msg {
			called = true
			return nil
		},
	})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd")
	}
	cmd()
	if !called {
		t.Errorf("expected OnConfirm to run when category is skipped")
	}
}

func TestPushConfirmDoesNotBypassDestructive(t *testing.T) {
	cfg := &config.Config{}
	cfg.ConfirmationPrefs.SkipDestroyVersion = true
	ctx := &tui.AppContext{Config: cfg}
	called := false
	cmd := PushConfirm(ctx, tui.Theme{}, ConfirmRequest{
		Level:    components.DangerDestructive,
		Category: ConfirmDestroyVersion,
		OnConfirm: func() tea.Msg {
			called = true
			return nil
		},
	})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd")
	}
	msg := cmd()
	if called {
		t.Errorf("destructive confirm must not be auto-skipped")
	}
	if _, ok := msg.(tui.PushScreenMsg); !ok {
		t.Errorf("expected PushScreenMsg, got %T", msg)
	}
}

func TestPushConfirmShowsDialogWhenNotSkipped(t *testing.T) {
	cfg := &config.Config{}
	ctx := &tui.AppContext{Config: cfg}
	cmd := PushConfirm(ctx, tui.Theme{}, ConfirmRequest{
		Level:    components.DangerNormal,
		Category: ConfirmDeleteVersion,
		OnConfirm: func() tea.Msg { return nil },
	})
	msg := cmd()
	if _, ok := msg.(tui.PushScreenMsg); !ok {
		t.Errorf("expected PushScreenMsg, got %T", msg)
	}
}
