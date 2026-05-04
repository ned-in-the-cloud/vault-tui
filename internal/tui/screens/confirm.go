package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/tui/components"
)

// ConfirmCategory identifies which preference flag in
// config.ConfirmationPrefs governs a particular dialog.
type ConfirmCategory int

const (
	// ConfirmNone disables the "remember" toggle.
	ConfirmNone ConfirmCategory = iota
	ConfirmDeleteVersion
	ConfirmDestroyVersion
	ConfirmDeleteAll
	ConfirmDestroyAll
)

// ConfirmRequest bundles the inputs to PushConfirm.
type ConfirmRequest struct {
	Title    string
	Body     string
	Level    components.DangerLevel
	Category ConfirmCategory
	// OnConfirm is the command run only when the user confirms.
	OnConfirm tea.Cmd
}

// PushConfirm returns a tea.Cmd that pushes a ConfirmScreen onto the
// stack, unless the user has previously opted out for this category — in
// which case it runs OnConfirm directly.
func PushConfirm(ctx *tui.AppContext, theme tui.Theme, req ConfirmRequest) tea.Cmd {
	if isCategorySkipped(ctx, req.Category) && req.Level != components.DangerDestructive {
		// Destructive operations are never auto-skipped.
		return req.OnConfirm
	}
	scr := newConfirmScreen(ctx, theme, req)
	return func() tea.Msg { return tui.PushScreenMsg{Screen: scr} }
}

func isCategorySkipped(ctx *tui.AppContext, c ConfirmCategory) bool {
	if ctx.Config == nil {
		return false
	}
	switch c {
	case ConfirmDeleteVersion:
		return ctx.Config.ConfirmationPrefs.SkipDeleteVersion
	case ConfirmDeleteAll:
		return ctx.Config.ConfirmationPrefs.SkipDeleteAll
	case ConfirmDestroyVersion:
		return ctx.Config.ConfirmationPrefs.SkipDestroyVersion
	case ConfirmDestroyAll:
		return ctx.Config.ConfirmationPrefs.SkipDestroyAll
	}
	return false
}

func setCategorySkipped(ctx *tui.AppContext, c ConfirmCategory) {
	if ctx.Config == nil {
		return
	}
	switch c {
	case ConfirmDeleteVersion:
		ctx.Config.ConfirmationPrefs.SkipDeleteVersion = true
	case ConfirmDeleteAll:
		ctx.Config.ConfirmationPrefs.SkipDeleteAll = true
	case ConfirmDestroyVersion:
		ctx.Config.ConfirmationPrefs.SkipDestroyVersion = true
	case ConfirmDestroyAll:
		ctx.Config.ConfirmationPrefs.SkipDestroyAll = true
	}
	ctx.SaveConfig()
}

// ConfirmScreen wraps a ConfirmDialog and runs OnConfirm on confirm.
type ConfirmScreen struct {
	ctx    *tui.AppContext
	theme  tui.Theme
	dialog *components.ConfirmDialog
	req    ConfirmRequest
}

func newConfirmScreen(ctx *tui.AppContext, theme tui.Theme, req ConfirmRequest) *ConfirmScreen {
	d := components.NewConfirmDialog(req.Title, req.Body, req.Level, confirmStylesFromTheme(theme))
	d.OfferRemember = req.Category != ConfirmNone && req.Level != components.DangerDestructive
	return &ConfirmScreen{ctx: ctx, theme: theme, dialog: d, req: req}
}

func confirmStylesFromTheme(t tui.Theme) components.ConfirmStyles {
	red := lipgloss.Color("#F87171")
	orange := lipgloss.Color("#FB923C")
	yellow := lipgloss.Color("#FCD34D")
	return components.ConfirmStyles{
		BoxNormal:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(yellow).Padding(1, 2),
		BoxWarning:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(orange).Padding(1, 2),
		BoxDestructive: lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(red).Padding(1, 2),
		Title:          t.Subtitle,
		Body:           t.Body,
		Hint:           t.Hint,
		Danger:         lipgloss.NewStyle().Foreground(red).Bold(true),
	}
}

func (s *ConfirmScreen) Title() string     { return "Confirm" }
func (s *ConfirmScreen) HelpHint() string  { return "y confirm  ·  n/esc cancel" }
func (s *ConfirmScreen) Init() tea.Cmd     { return nil }
func (s *ConfirmScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		var cmd tea.Cmd
		s.dialog, cmd = s.dialog.Update(msg)
		if !s.dialog.Decided() {
			return s, cmd
		}
		// Decided.
		if s.dialog.Confirmed() {
			if s.dialog.Remember() && s.req.Category != ConfirmNone {
				setCategorySkipped(s.ctx, s.req.Category)
			}
			return s, tea.Sequence(
				func() tea.Msg { return tui.PopScreenMsg{} },
				s.req.OnConfirm,
			)
		}
		return s, func() tea.Msg { return tui.PopScreenMsg{} }
	}
	var cmd tea.Cmd
	s.dialog, cmd = s.dialog.Update(msg)
	return s, cmd
}

func (s *ConfirmScreen) View() string {
	body := s.dialog.View()
	// Indent slightly so the dialog feels modal.
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
