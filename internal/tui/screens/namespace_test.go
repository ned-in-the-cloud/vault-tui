package screens

import (
	"context"
	"io"
	"log/slog"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

func testNamespaceContext() *tui.AppContext {
	return &tui.AppContext{
		Vault:  &vault.MockClient{},
		Config: &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestNamespacePromptEnterUpdatesNamespaceAndReturnsCommand(t *testing.T) {
	ctx := testNamespaceContext()
	ctx.Vault.SetNamespace("team-a")

	screen := NewNamespacePrompt(ctx, tui.Theme{})
	prompt, ok := screen.(*namespacePrompt)
	if !ok {
		t.Fatalf("screen type = %T, want *namespacePrompt", screen)
	}
	prompt.input.SetValue("team-b")

	_, cmd := prompt.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	_ = cmd()
	if got := ctx.Vault.Namespace(); got != "team-b" {
		t.Fatalf("namespace = %q, want team-b", got)
	}
}

func TestNamespacePromptEnterWithoutChangeOnlyPops(t *testing.T) {
	ctx := testNamespaceContext()
	ctx.Vault.SetNamespace("team-a")

	screen := NewNamespacePrompt(ctx, tui.Theme{})
	prompt := screen.(*namespacePrompt)
	prompt.input.SetValue("team-a")

	_, cmd := prompt.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(tui.PopScreenMsg); !ok {
		t.Fatalf("message type = %T, want tui.PopScreenMsg", msg)
	}
	if got := ctx.Vault.Namespace(); got != "team-a" {
		t.Fatalf("namespace = %q, want team-a", got)
	}
}

func TestEngineListReloadsOnNamespaceChanged(t *testing.T) {
	mock := &vault.MockClient{}
	listCalls := 0
	ctx := &tui.AppContext{
		Vault:  mock,
		Config: &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mock.ListMountsFn = func(_ context.Context) (map[string]*vault.MountInfo, error) {
		listCalls++
		return map[string]*vault.MountInfo{}, nil
	}

	screen := NewEngineListScreen(ctx, tui.Theme{})
	engine, ok := screen.(*EngineListScreen)
	if !ok {
		t.Fatalf("screen type = %T, want *EngineListScreen", screen)
	}

	_, cmd := engine.Update(namespaceChangedMsg{Previous: "team-a", Current: "team-b"})
	if !engine.loading {
		t.Fatal("expected loading to be set on namespace change")
	}
	if cmd == nil {
		t.Fatal("expected reload command")
	}
	_ = cmd()
	if listCalls != 1 {
		t.Fatalf("ListMounts calls = %d, want 1", listCalls)
	}
}
