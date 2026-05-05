package screens

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

// TokenDetailScreen shows the full token info table and exposes
// hotkeys for renew (`r`) and auto-renew toggle (`a`).
type TokenDetailScreen struct {
	ctx      *tui.AppContext
	theme    tui.Theme
	token    *vault.TokenInfo
	renewing bool
	lastErr  error
	lastMsg  string
}

// NewTokenDetailScreen constructs the token detail screen.
func NewTokenDetailScreen(ctx *tui.AppContext, theme tui.Theme, token *vault.TokenInfo) tui.Screen {
	return &TokenDetailScreen{ctx: ctx, theme: theme, token: token}
}

func (s *TokenDetailScreen) Title() string { return "Token" }

func (s *TokenDetailScreen) HelpHint() string {
	return "r renew  a auto-renew  esc back"
}

func (s *TokenDetailScreen) Init() tea.Cmd {
	return tea.Batch(tickCmd(), s.publishToken())
}

func (s *TokenDetailScreen) publishToken() tea.Cmd {
	tok := s.token
	return func() tea.Msg { return tui.TokenInfoMsg{Info: tok} }
}

func (s *TokenDetailScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
		switch m.String() {
		case tui.KeyRenew:
			if s.token == nil || !s.token.Renewable {
				s.lastErr = fmt.Errorf("token is not renewable")
				return s, nil
			}
			return s, s.renewCmd()
		case tui.KeyAutoRenew:
			s.ctx.Config.AutoRenewToken = !s.ctx.Config.AutoRenewToken
			s.ctx.SaveConfig()
			s.lastMsg = fmt.Sprintf("auto-renew %v", s.ctx.Config.AutoRenewToken)
			return s, tui.AutoRenewChanged()
		}
	case dashboardTickMsg:
		return s, tickCmd()
	case tokenRenewedMsg:
		s.renewing = false
		if m.err != nil {
			s.lastErr = fmt.Errorf("renew: %w", m.err)
			if cmd := pushReauthIfInvalidToken(s.ctx, s.theme, m.err); cmd != nil {
				return s, cmd
			}
			return s, nil
		}
		s.token = m.info
		s.lastErr = nil
		s.lastMsg = "token renewed"
		return s, s.publishToken()
	case tui.TokenInfoMsg:
		s.token = m.Info
		return s, nil
	}
	return s, nil
}

func (s *TokenDetailScreen) renewCmd() tea.Cmd {
	if s.renewing {
		return nil
	}
	s.renewing = true
	vc := s.ctx.Vault
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ti, err := vc.RenewToken(ctx, 0)
		return tokenRenewedMsg{info: ti, err: err}
	}
}

func (s *TokenDetailScreen) View() string {
	var b strings.Builder
	b.WriteString(s.theme.Subtitle.Render("Token details"))
	b.WriteString("\n\n")
	if s.token == nil {
		b.WriteString("(no token info)")
		return b.String()
	}
	t := s.token
	b.WriteString(kv("display_name", t.DisplayName))
	b.WriteString(kv("accessor", t.Accessor))
	b.WriteString(kv("entity_id", t.EntityID))
	b.WriteString(kv("policies", strings.Join(t.Policies, ", ")))
	b.WriteString(kv("ttl", formatDuration(remaining(t))))
	b.WriteString(kv("ttl (issued)", formatDuration(t.TTL)))
	if !t.ExpireTime.IsZero() {
		b.WriteString(kv("expires", t.ExpireTime.Format(time.RFC3339)))
	}
	b.WriteString(kv("creation_ttl", formatDuration(t.CreationTTL)))
	b.WriteString(kv("renewable", fmt.Sprint(t.Renewable)))
	b.WriteString(kv("orphan", fmt.Sprint(t.Orphan)))
	if t.NumUses != 0 {
		b.WriteString(kv("num_uses", fmt.Sprint(t.NumUses)))
	}
	b.WriteString(kv("auto-renew", fmt.Sprint(s.ctx.Config.AutoRenewToken)))

	if s.renewing {
		b.WriteString("\n" + s.theme.Hint.Render("renewing..."))
	}
	if s.lastMsg != "" {
		b.WriteString("\n" + s.theme.Hint.Render(s.lastMsg))
	}
	if s.lastErr != nil {
		b.WriteString("\n" + s.theme.Error.Render(s.lastErr.Error()))
	}
	return b.String()
}
