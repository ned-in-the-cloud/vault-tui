package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ned1313/vault-tui/internal/vault"
)

// rootModel is the top-level tea.Model. It owns a ScreenStack and routes
// messages to the active screen, while handling global concerns
// (quit, help toggle, error overlay, status bar).
type rootModel struct {
	ctx       *AppContext
	theme     Theme
	stack     *ScreenStack
	width     int
	height    int
	lastErr   error
	quitting  bool
	tokenInfo *vault.TokenInfo // most recent token info for status bar
}

// NewRoot constructs the root tea.Model with the supplied initial screen.
func NewRoot(ctx *AppContext, initial Screen) tea.Model {
	stack := &ScreenStack{}
	stack.Push(initial)
	return &rootModel{
		ctx:   ctx,
		theme: DefaultTheme(),
		stack: stack,
	}
}

// Theme exposes the theme so screen factories outside the package can use
// it for consistent styling.
func (r *rootModel) Theme() Theme { return r.theme }

func (r *rootModel) Init() tea.Cmd {
	if cur := r.stack.Current(); cur != nil {
		return cur.Init()
	}
	return nil
}

func (r *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = m.Width
		r.height = m.Height

	case tea.KeyPressMsg:
		switch m.String() {
		case KeyQuit:
			r.quitting = true
			return r, tea.Quit
		case KeyBack:
			// Pop unless this is the only screen on the stack.
			if r.stack.Len() > 1 {
				r.stack.Pop()
				if cur := r.stack.Current(); cur != nil {
					return r, cur.Init()
				}
				return r, nil
			}
			// Fall through to current screen if it wants to handle esc.
		}

	case PushScreenMsg:
		r.stack.Push(m.Screen)
		return r, m.Screen.Init()
	case PopScreenMsg:
		if r.stack.Len() > 1 {
			r.stack.Pop()
			if cur := r.stack.Current(); cur != nil {
				return r, cur.Init()
			}
		}
		return r, nil
	case ReplaceScreenMsg:
		r.stack.Replace(m.Screen)
		return r, m.Screen.Init()
	case QuitMsg:
		r.quitting = true
		return r, tea.Quit
	case errorMsg:
		r.lastErr = m.err
		return r, nil
	case ClearErrorMsg:
		r.lastErr = nil
		return r, nil
	case TokenInfoMsg:
		r.tokenInfo = m.Info
		return r, nil
	}

	cur := r.stack.Current()
	if cur == nil {
		return r, nil
	}
	next, cmd := cur.Update(msg)
	// Replace current with next (screens may return updated copies).
	r.stack.stack[len(r.stack.stack)-1] = next
	return r, cmd
}

func (r *rootModel) View() tea.View {
	if r.quitting {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}
	cur := r.stack.Current()
	if cur == nil {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}
	body := cur.View()

	header := r.theme.Title.Render("vault-tui")
	if t := cur.Title(); t != "" {
		header += "  " + r.theme.Subtitle.Render(t)
	}

	status := r.statusBar()

	var errLine string
	if r.lastErr != nil {
		errLine = r.theme.Error.Render("error: "+r.lastErr.Error()) + "\n"
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
		"",
		errLine+status,
	)
	v := tea.NewView(view)
	v.AltScreen = true
	return v
}

func (r *rootModel) statusBar() string {
	hint := "ctrl+c quit  esc back  ? help"
	if cur := r.stack.Current(); cur != nil {
		if hh, ok := cur.(HelpHinter); ok {
			if h := hh.HelpHint(); h != "" {
				hint = h
			}
		}
	}
	return r.theme.RenderStatusBar(StatusBarState{
		Address:   r.ctx.Vault.Address(),
		Namespace: r.ctx.Vault.Namespace(),
		Token:     r.tokenInfo,
		AutoRenew: r.ctx.Config.AutoRenewToken,
		HelpHint:  hint,
		Width:     r.width,
	})
}

// errorMsg is delivered by screens that want to surface an error in the
// global error line.
type errorMsg struct{ err error }

// ShowError returns a tea.Cmd that posts an errorMsg.
func ShowError(err error) tea.Cmd {
	return func() tea.Msg { return errorMsg{err: err} }
}

// ClearErrorMsg clears the global error overlay.
type ClearErrorMsg struct{}

// TokenInfoMsg is delivered by screens to publish the active token's
// info to the root model so the status bar can render TTL etc.
type TokenInfoMsg struct{ Info *vault.TokenInfo }

// HelpHinter is implemented by screens that want to override the
// default help hint shown on the right side of the status bar.
type HelpHinter interface {
	HelpHint() string
}

// ClearError returns a tea.Cmd that clears the global error line.
func ClearError() tea.Cmd {
	return func() tea.Msg { return errorMsg{err: nil} }
}
