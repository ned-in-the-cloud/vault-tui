package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ned1313/vault-tui/internal/vault"
)

// rootModel is the top-level tea.Model. It owns a ScreenStack and routes
// messages to the active screen, while handling global concerns
// (quit, help toggle, error overlay, status bar).
type rootModel struct {
	ctx         *AppContext
	theme       Theme
	stack       *ScreenStack
	width       int
	height      int
	lastErr     error
	appErr      *appErrorState
	quitting    bool
	tokenInfo   *vault.TokenInfo // most recent token info for status bar
	showCommand bool             // toggled by KeyShowCmd
	showHelp    bool             // toggled by KeyHelp

	watcher         *vault.TokenWatcher
	watcherEvents   <-chan vault.WatchEvent
	watcherCancel   context.CancelFunc
	watcherAccessor string
}

type appErrorState struct {
	err     error
	actions []ErrorAction
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
	cmds := []tea.Cmd{statusTickCmd()}
	if cur := r.stack.Current(); cur != nil {
		cmds = append(cmds, cur.Init())
	}
	return tea.Batch(cmds...)
}

func (r *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var rootCmd tea.Cmd
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = m.Width
		r.height = m.Height

	case statusTickMsg:
		return r, statusTickCmd()

	case tea.KeyPressMsg:
		if r.appErr != nil {
			switch m.String() {
			case "esc", "c":
				r.appErr = nil
				return r, nil
			}
			for _, a := range r.appErr.actions {
				if strings.EqualFold(m.String(), a.Key) {
					r.appErr = nil
					if a.Cmd != nil {
						return r, a.Cmd
					}
					return r, nil
				}
			}
		}
		switch m.String() {
		case KeyQuit, KeyQuitAlt:
			r.stopWatcher()
			r.quitting = true
			return r, tea.Quit
		case KeyHelp:
			r.showHelp = !r.showHelp
			return r, nil
		case KeyShowCmd:
			// alt+x is reserved as a global toggle for the equivalent
			// Vault CLI command overlay. It is not produced by typing
			// in a textinput so we can safely intercept it here.
			r.showCommand = !r.showCommand
			return r, nil
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
		r.stopWatcher()
		r.quitting = true
		return r, tea.Quit
	case errorMsg:
		r.lastErr = m.err
		return r, nil
	case appErrorMsg:
		r.appErr = &appErrorState{err: m.err, actions: m.actions}
		return r, nil
	case ClearErrorMsg:
		r.lastErr = nil
		r.appErr = nil
		return r, nil
	case TokenInfoMsg:
		r.tokenInfo = m.Info
		rootCmd = r.reconcileWatcher()
	case autoRenewChangedMsg:
		rootCmd = r.reconcileWatcher()
		return r, rootCmd
	case tokenWatchEventMsg:
		if !m.ok {
			r.stopWatcher()
			return r, nil
		}
		if m.event.Renewed != nil {
			return r, tea.Batch(r.waitForWatchEventCmd(), r.lookupTokenCmd())
		}
		if m.event.Done {
			if m.event.Err != nil {
				r.lastErr = fmt.Errorf("auto-renew: %w", m.event.Err)
			}
			r.stopWatcher()
			return r, nil
		}
		return r, r.waitForWatchEventCmd()
	case tokenLookupMsg:
		if m.err != nil {
			r.lastErr = fmt.Errorf("auto-renew lookup: %w", m.err)
			return r, nil
		}
		r.tokenInfo = m.info
		return r, func() tea.Msg { return TokenInfoMsg{Info: m.info} }
	}

	cur := r.stack.Current()
	if cur == nil {
		return r, rootCmd
	}
	next, cmd := cur.Update(msg)
	// Replace current with next (screens may return updated copies).
	r.stack.stack[len(r.stack.stack)-1] = next
	return r, tea.Batch(rootCmd, cmd)
}

func (r *rootModel) reconcileWatcher() tea.Cmd {
	if r.tokenInfo == nil || !r.tokenInfo.Renewable || !r.ctx.Config.AutoRenewToken {
		r.stopWatcher()
		return nil
	}
	if r.watcher != nil && r.watcherAccessor == r.tokenInfo.Accessor {
		return nil
	}
	r.stopWatcher()
	watcher, err := r.ctx.Vault.NewTokenWatcher(0)
	if err != nil {
		r.lastErr = fmt.Errorf("start auto-renew watcher: %w", err)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.watcher = watcher
	r.watcherCancel = cancel
	r.watcherEvents = watcher.Run(ctx)
	r.watcherAccessor = r.tokenInfo.Accessor
	return r.waitForWatchEventCmd()
}

func (r *rootModel) stopWatcher() {
	if r.watcherCancel != nil {
		r.watcherCancel()
		r.watcherCancel = nil
	}
	if r.watcher != nil {
		r.watcher.Stop()
		r.watcher = nil
	}
	r.watcherEvents = nil
	r.watcherAccessor = ""
}

func (r *rootModel) waitForWatchEventCmd() tea.Cmd {
	ch := r.watcherEvents
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-ch
		return tokenWatchEventMsg{event: evt, ok: ok}
	}
}

func (r *rootModel) lookupTokenCmd() tea.Cmd {
	vc := r.ctx.Vault
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ti, err := vc.LookupToken(ctx)
		return tokenLookupMsg{info: ti, err: err}
	}
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

	var cmdLine string
	if r.showCommand {
		if vc, ok := cur.(VaultCommander); ok {
			if c := strings.TrimSpace(vc.VaultCommand()); c != "" {
				cmdLine = r.theme.Hint.Render("$ "+c) + "\n"
			} else {
				cmdLine = r.theme.Hint.Render("(no equivalent command on this screen)") + "\n"
			}
		} else {
			cmdLine = r.theme.Hint.Render("(no equivalent command on this screen)") + "\n"
		}
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		r.mainBody(cur, body),
		"",
		cmdLine+errLine+status,
	)
	v := tea.NewView(view)
	v.AltScreen = true
	return v
}

func (r *rootModel) mainBody(cur Screen, body string) string {
	out := body
	if r.showHelp {
		help := r.helpPanel(cur)
		out = lipgloss.JoinVertical(lipgloss.Left, out, "", help)
	}
	if r.appErr != nil {
		errOverlay := r.errorOverlay()
		out = lipgloss.JoinVertical(lipgloss.Left, out, "", errOverlay)
	}
	return out
}

func (r *rootModel) helpPanel(cur Screen) string {
	hint := ""
	if hh, ok := cur.(HelpHinter); ok {
		hint = hh.HelpHint()
	}
	var b strings.Builder
	b.WriteString(r.theme.Subtitle.Render("Help"))
	b.WriteString("\n\n")
	b.WriteString("Global:\n")
	b.WriteString("  ctrl+c / ctrl+q   quit\n")
	b.WriteString("  esc               back\n")
	b.WriteString("  ?                 toggle help\n")
	b.WriteString("  alt+x             toggle command overlay\n")
	if hint != "" {
		b.WriteString("\nCurrent screen:\n")
		b.WriteString("  " + hint + "\n")
	}
	return r.theme.Box.Render(strings.TrimRight(b.String(), "\n"))
}

func (r *rootModel) errorOverlay() string {
	if r.appErr == nil || r.appErr.err == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(r.theme.Error.Render("Error: " + r.appErr.err.Error()))
	b.WriteString("\n")
	if len(r.appErr.actions) > 0 {
		b.WriteString("Actions: ")
		parts := make([]string, 0, len(r.appErr.actions)+1)
		for _, a := range r.appErr.actions {
			parts = append(parts, "["+a.Key+"] "+a.Label)
		}
		parts = append(parts, "[esc] dismiss")
		b.WriteString(strings.Join(parts, "  "))
	} else {
		b.WriteString("Press esc to dismiss")
	}
	return r.theme.Box.Render(strings.TrimRight(b.String(), "\n"))
}

func (r *rootModel) statusBar() string {
	hint := "ctrl+c/ctrl+q quit  esc back  ? help  alt+x cmd"
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

// ErrorAction is an optional action rendered on the global error overlay.
// Pressing Key executes Cmd.
type ErrorAction struct {
	Key   string
	Label string
	Cmd   tea.Cmd
}

type appErrorMsg struct {
	err     error
	actions []ErrorAction
}

type tokenWatchEventMsg struct {
	event vault.WatchEvent
	ok    bool
}

type tokenLookupMsg struct {
	info *vault.TokenInfo
	err  error
}

type autoRenewChangedMsg struct{}

type statusTickMsg time.Time

func statusTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return statusTickMsg(t) })
}

// AutoRenewChanged notifies the root model that AutoRenewToken was toggled.
func AutoRenewChanged() tea.Cmd {
	return func() tea.Msg { return autoRenewChangedMsg{} }
}

// ShowError returns a tea.Cmd that posts an errorMsg.
func ShowError(err error) tea.Cmd {
	return func() tea.Msg { return errorMsg{err: err} }
}

// ShowAppError renders a dedicated error overlay with optional actions.
func ShowAppError(err error, actions ...ErrorAction) tea.Cmd {
	return func() tea.Msg {
		return appErrorMsg{err: err, actions: actions}
	}
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

// VaultCommander is implemented by screens that can describe the
// equivalent Vault CLI command for their current operation. The
// returned string should be a single line; an empty string means
// "no equivalent command available right now".
type VaultCommander interface {
	VaultCommand() string
}

// ClearError returns a tea.Cmd that clears the global error line.
func ClearError() tea.Cmd {
	return func() tea.Msg { return errorMsg{err: nil} }
}
