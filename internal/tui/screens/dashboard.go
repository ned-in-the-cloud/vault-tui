package screens

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

// tickInterval is how often the dashboard refreshes the TTL display.
const tickInterval = time.Second

// refreshInterval is how often the dashboard re-fetches token info.
const refreshInterval = 30 * time.Second

// autoRenewThreshold: when AutoRenewToken is on, fire a renew once the
// remaining TTL drops below this duration.
const autoRenewThreshold = 60 * time.Second

// DashboardScreen is the post-auth landing screen with three panels
// (server / token / navigation), a TTL countdown via tea.Tick, and
// hotkeys for namespace switch, manual renew, and auto-renew toggle.
type DashboardScreen struct {
	ctx    *tui.AppContext
	theme  tui.Theme
	token  *vault.TokenInfo
	health *vault.HealthInfo

	menuIdx     int
	renewing    bool
	refreshing  bool
	lastErr     error
	autoRenewed time.Time
}

type dashboardMenuItem struct {
	key   string
	label string
	desc  string
}

var dashboardMenu = []dashboardMenuItem{
	{tui.KeyEngines, "Secrets Engines", "Browse KV mounts and secrets"},
	{tui.KeyTokenView, "Token Details", "Renew, inspect, or rotate the active token"},
	{"q", "Quit", "Exit vault-tui"},
}

// NewDashboardScreen constructs the dashboard.
func NewDashboardScreen(ctx *tui.AppContext, theme tui.Theme, token *vault.TokenInfo, health *vault.HealthInfo) tui.Screen {
	return &DashboardScreen{ctx: ctx, theme: theme, token: token, health: health}
}

func (s *DashboardScreen) Title() string { return "Dashboard" }

func (s *DashboardScreen) HelpHint() string {
	return "↑/↓ select  enter open  N namespace  r renew  a auto-renew  q quit"
}

func (s *DashboardScreen) Init() tea.Cmd {
	return tea.Batch(s.publishToken(), tickCmd(), refreshCmd())
}

func (s *DashboardScreen) publishToken() tea.Cmd {
	tok := s.token
	return func() tea.Msg { return tui.TokenInfoMsg{Info: tok} }
}

// --- messages ---

type dashboardTickMsg time.Time
type dashboardRefreshMsg struct{}
type tokenRefreshedMsg struct {
	info *vault.TokenInfo
	err  error
}
type tokenRenewedMsg struct {
	info *vault.TokenInfo
	err  error
}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return dashboardTickMsg(t) })
}
func refreshCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return dashboardRefreshMsg{} })
}

// --- update ---

func (s *DashboardScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
		return s.handleKey(m.String())

	case dashboardTickMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, tickCmd())
		if cmd := s.maybeAutoRenew(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return s, tea.Batch(cmds...)

	case dashboardRefreshMsg:
		return s, tea.Batch(refreshCmd(), s.refreshTokenCmd())

	case tokenRefreshedMsg:
		s.refreshing = false
		if m.err != nil {
			s.lastErr = fmt.Errorf("refresh: %w", m.err)
			return s, nil
		}
		s.token = m.info
		return s, s.publishToken()

	case tokenRenewedMsg:
		s.renewing = false
		if m.err != nil {
			s.lastErr = fmt.Errorf("renew: %w", m.err)
			return s, nil
		}
		s.token = m.info
		s.lastErr = nil
		return s, s.publishToken()
	}
	return s, nil
}

func (s *DashboardScreen) handleKey(key string) (tui.Screen, tea.Cmd) {
	switch key {
	case "up", "k":
		if s.menuIdx > 0 {
			s.menuIdx--
		}
	case "down", "j":
		if s.menuIdx < len(dashboardMenu)-1 {
			s.menuIdx++
		}
	case "enter":
		return s.activateMenuByKey(dashboardMenu[s.menuIdx].key)
	case tui.KeyEngines, tui.KeyTokenView:
		return s.activateMenuByKey(key)
	case "q":
		return s, tea.Quit
	case tui.KeyNamespace:
		ns := NewNamespacePrompt(s.ctx, s.theme)
		return s, func() tea.Msg { return tui.PushScreenMsg{Screen: ns} }
	case tui.KeyRenew:
		if s.token == nil || !s.token.Renewable {
			s.lastErr = fmt.Errorf("token is not renewable")
			return s, nil
		}
		return s, s.renewCmd()
	case tui.KeyAutoRenew:
		s.ctx.Config.AutoRenewToken = !s.ctx.Config.AutoRenewToken
		s.ctx.SaveConfig()
	}
	return s, nil
}

func (s *DashboardScreen) activateMenuByKey(key string) (tui.Screen, tea.Cmd) {
	switch key {
	case tui.KeyTokenView:
		td := NewTokenDetailScreen(s.ctx, s.theme, s.token)
		return s, func() tea.Msg { return tui.PushScreenMsg{Screen: td} }
	case tui.KeyEngines:
		s.lastErr = fmt.Errorf("secrets engine browser arrives in phase 3")
		return s, nil
	case "q":
		return s, tea.Quit
	}
	return s, nil
}

func (s *DashboardScreen) renewCmd() tea.Cmd {
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

func (s *DashboardScreen) refreshTokenCmd() tea.Cmd {
	if s.refreshing {
		return nil
	}
	s.refreshing = true
	vc := s.ctx.Vault
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ti, err := vc.LookupToken(ctx)
		return tokenRefreshedMsg{info: ti, err: err}
	}
}

func (s *DashboardScreen) maybeAutoRenew() tea.Cmd {
	if !s.ctx.Config.AutoRenewToken || s.token == nil || !s.token.Renewable {
		return nil
	}
	if s.renewing {
		return nil
	}
	if s.token.ExpireTime.IsZero() {
		return nil
	}
	if time.Until(s.token.ExpireTime) > autoRenewThreshold {
		return nil
	}
	if time.Since(s.autoRenewed) < 30*time.Second {
		return nil
	}
	s.autoRenewed = time.Now()
	return s.renewCmd()
}

// --- view ---

func (s *DashboardScreen) View() string {
	server := s.renderServerPanel()
	token := s.renderTokenPanel()
	menu := s.renderMenu()

	top := lipgloss.JoinHorizontal(lipgloss.Top, server, "  ", token)
	out := lipgloss.JoinVertical(lipgloss.Left, top, "", menu)
	if s.lastErr != nil {
		out += "\n\n" + s.theme.Error.Render(s.lastErr.Error())
	}
	if s.renewing {
		out += "\n" + s.theme.Hint.Render("renewing token...")
	}
	return out
}

func (s *DashboardScreen) renderServerPanel() string {
	var b strings.Builder
	b.WriteString(s.theme.Subtitle.Render("Server"))
	b.WriteString("\n")
	b.WriteString(kv("address", s.ctx.Vault.Address()))
	if ns := s.ctx.Vault.Namespace(); ns != "" {
		b.WriteString(kv("namespace", ns))
	}
	if s.health != nil {
		b.WriteString(kv("version", s.health.Version))
		b.WriteString(kv("initialized", fmt.Sprint(s.health.Initialized)))
		b.WriteString(kv("sealed", fmt.Sprint(s.health.Sealed)))
		b.WriteString(kv("standby", fmt.Sprint(s.health.Standby)))
	}
	return s.theme.Box.Render(strings.TrimRight(b.String(), "\n"))
}

func (s *DashboardScreen) renderTokenPanel() string {
	var b strings.Builder
	b.WriteString(s.theme.Subtitle.Render("Token"))
	b.WriteString("\n")
	if s.token == nil {
		b.WriteString("(no token info)")
		return s.theme.Box.Render(b.String())
	}
	b.WriteString(kv("display", s.token.DisplayName))
	b.WriteString(kv("accessor", truncate(s.token.Accessor, 20)))
	b.WriteString(kv("policies", strings.Join(s.token.Policies, ", ")))
	b.WriteString(kv("ttl", formatDuration(remaining(s.token))))
	b.WriteString(kv("renewable", fmt.Sprint(s.token.Renewable)))
	b.WriteString(kv("auto-renew", fmt.Sprint(s.ctx.Config.AutoRenewToken)))
	return s.theme.Box.Render(strings.TrimRight(b.String(), "\n"))
}

func (s *DashboardScreen) renderMenu() string {
	var b strings.Builder
	b.WriteString(s.theme.Subtitle.Render("Menu"))
	b.WriteString("\n")
	for i, item := range dashboardMenu {
		cursor := "  "
		label := fmt.Sprintf("[%s] %s — %s", item.key, item.label, item.desc)
		if i == s.menuIdx {
			cursor = "▶ "
			label = s.theme.Selection.Render(label)
		}
		b.WriteString(cursor + label + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func kv(k, v string) string {
	return fmt.Sprintf("  %-11s %s\n", k, v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func remaining(ti *vault.TokenInfo) time.Duration {
	if ti == nil {
		return 0
	}
	if !ti.ExpireTime.IsZero() {
		d := time.Until(ti.ExpireTime)
		if d < 0 {
			return 0
		}
		return d
	}
	return ti.TTL
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "∞"
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	sec := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, sec)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, sec)
	}
	return fmt.Sprintf("%ds", sec)
}
