package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/ned1313/vault-tui/internal/vault"
)

// StatusBarState is the persistent footer rendered below every screen.
// It surfaces server info, the active namespace, a TTL countdown, and a
// short policy summary so the user always knows their security context.
type StatusBarState struct {
	Address   string
	Namespace string
	Token     *vault.TokenInfo
	AutoRenew bool
	HelpHint  string
	Width     int
}

// Render draws the status bar styled with the theme's StatusBar style.
func (t Theme) RenderStatusBar(s StatusBarState) string {
	parts := []string{}
	if s.Address != "" {
		parts = append(parts, "server="+s.Address)
	}
	if s.Namespace != "" {
		parts = append(parts, "ns="+s.Namespace)
	}
	if s.Token != nil {
		parts = append(parts, "ttl="+formatTTL(s.Token))
		if len(s.Token.Policies) > 0 {
			parts = append(parts, "policies="+strings.Join(s.Token.Policies, ","))
		}
		if s.AutoRenew && s.Token.Renewable {
			parts = append(parts, "auto-renew=on")
		}
	}
	hint := s.HelpHint
	if hint == "" {
		hint = "ctrl+c quit  esc back  ? help"
	}
	left := strings.Join(parts, "  |  ")
	width := s.Width
	if width <= 0 {
		return t.StatusBar.Render(left + "  |  " + hint)
	}
	pad := width - lipgloss.Width(left) - lipgloss.Width(hint) - 2
	if pad < 1 {
		return t.StatusBar.Width(width).Render(left + " " + hint)
	}
	spacer := strings.Repeat(" ", pad)
	return t.StatusBar.Width(width).Render(left + spacer + hint)
}

// formatTTL turns a TokenInfo into a compact "1h2m3s" string. If the
// token has an ExpireTime set we recompute against the wall clock so a
// 1-second tick keeps the display fresh; otherwise we fall back to TTL.
func formatTTL(ti *vault.TokenInfo) string {
	var d time.Duration
	switch {
	case !ti.ExpireTime.IsZero():
		d = time.Until(ti.ExpireTime)
	case !ti.IssueTime.IsZero() && ti.TTL > 0:
		d = time.Until(ti.IssueTime.Add(ti.TTL))
	case ti.TTL > 0:
		d = ti.TTL
	default:
		return "∞"
	}
	if d <= 0 {
		return "expired"
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%02dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm%02ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
