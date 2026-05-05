package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/vault"
)

type testScreen struct {
	seenTokenMsg bool
}

func (s *testScreen) Init() tea.Cmd { return nil }
func (s *testScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if _, ok := msg.(TokenInfoMsg); ok {
		s.seenTokenMsg = true
	}
	return s, nil
}
func (s *testScreen) View() string  { return "" }
func (s *testScreen) Title() string { return "test" }

func TestTokenInfoMsgUpdatesRootAndForwardsToScreen(t *testing.T) {
	scr := &testScreen{}
	ctx := &AppContext{
		Config: &config.Config{AutoRenewToken: false},
		Vault:  &vault.MockClient{},
	}
	model := NewRoot(ctx, scr).(*rootModel)
	want := &vault.TokenInfo{Accessor: "acc-123"}

	_, _ = model.Update(TokenInfoMsg{Info: want})

	if model.tokenInfo != want {
		t.Fatalf("root tokenInfo not updated")
	}
	if !scr.seenTokenMsg {
		t.Fatalf("expected screen to receive TokenInfoMsg")
	}
}

func TestAutoRenewChangedCmdMessageType(t *testing.T) {
	cmd := AutoRenewChanged()
	if cmd == nil {
		t.Fatalf("expected non-nil command")
	}
	if _, ok := cmd().(autoRenewChangedMsg); !ok {
		t.Fatalf("expected autoRenewChangedMsg")
	}
}

func TestTokenLookupMsgProducesTokenInfoMsg(t *testing.T) {
	scr := &testScreen{}
	ctx := &AppContext{
		Config: &config.Config{AutoRenewToken: false},
		Vault:  &vault.MockClient{},
	}
	model := NewRoot(ctx, scr).(*rootModel)
	info := &vault.TokenInfo{Accessor: "acc-456"}

	_, cmd := model.Update(tokenLookupMsg{info: info})
	if cmd == nil {
		t.Fatalf("expected command")
	}
	msg := cmd()
	ti, ok := msg.(TokenInfoMsg)
	if !ok {
		t.Fatalf("expected TokenInfoMsg, got %T", msg)
	}
	if ti.Info != info {
		t.Fatalf("unexpected token info payload")
	}
}
