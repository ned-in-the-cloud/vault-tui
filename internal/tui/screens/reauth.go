package screens

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

// pushReauthIfInvalidToken returns a command that pushes the re-auth
// screen when the supplied error indicates an expired/invalid token.
// Returns nil for all non-token errors.
func pushReauthIfInvalidToken(ctx *tui.AppContext, theme tui.Theme, err error) tea.Cmd {
	if errors.Is(err, vault.ErrInvalidToken) {
		reauth := func() tea.Msg { return tui.PushScreenMsg{Screen: NewReauthScreen(ctx, theme)} }
		return tea.Batch(
			reauth,
			tui.ShowAppError(err, tui.ErrorAction{
				Key:   "a",
				Label: "re-authenticate",
				Cmd:   reauth,
			}),
		)
	}
	if errors.Is(err, vault.ErrConnectionFailed) {
		reconnect := func() tea.Msg {
			return tui.ReplaceScreenMsg{Screen: NewConnectScreen(ctx, theme)}
		}
		return tui.ShowAppError(err, tui.ErrorAction{
			Key:   "r",
			Label: "reconnect",
			Cmd:   reconnect,
		})
	}
	if err != nil {
		return tui.ShowAppError(err)
	}
	return nil
}
