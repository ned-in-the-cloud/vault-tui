package screens

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/tui"
)

// refreshTokenInfoCmd looks up the active token via the shared app
// context and republishes its info as a TokenInfoMsg so the status
// bar's TTL display refreshes. Returns nil on error so callers can
// safely tea.Batch it alongside other refresh commands.
func refreshTokenInfoCmd(appCtx *tui.AppContext) tea.Cmd {
	if appCtx == nil || appCtx.Vault == nil {
		return nil
	}
	vc := appCtx.Vault
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ti, err := vc.LookupToken(ctx)
		if err != nil {
			return nil
		}
		return tui.TokenInfoMsg{Info: ti}
	}
}
