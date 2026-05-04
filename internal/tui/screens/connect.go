package screens

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

// ConnectScreen collects the server address, optional namespace, and
// optional CA bundle path. On submit it calls Health() and, on success,
// pushes the AuthScreen.
type ConnectScreen struct {
	ctx   *tui.AppContext
	theme tui.Theme

	inputs   []textinput.Model
	focus    int
	loading  bool
	err      error
	insecure bool // set when user confirms http://
	awaitOK  bool // set when an http confirmation prompt is showing
}

// NewConnectScreen builds the screen, pre-populating fields from the
// most recent server-history entry.
func NewConnectScreen(ctx *tui.AppContext, theme tui.Theme) tui.Screen {
	addr := textinput.New()
	addr.Prompt = "address  > "
	addr.Placeholder = "https://vault.example.com:8200"
	addr.CharLimit = 200
	addr.Focus()

	ns := textinput.New()
	ns.Prompt = "namespace> "
	ns.Placeholder = "(optional, Vault Enterprise)"
	ns.CharLimit = 200

	ca := textinput.New()
	ca.Prompt = "ca bundle> "
	ca.Placeholder = "(optional, path to PEM)"
	ca.CharLimit = 500

	if len(ctx.Config.ServerHistory) > 0 {
		last := ctx.Config.ServerHistory[0]
		addr.SetValue(last.Address)
		ns.SetValue(last.Namespace)
		ca.SetValue(last.CABundlePath)
	} else if envAddr := strings.TrimSpace(os.Getenv("VAULT_ADDR")); envAddr != "" {
		// First-run fallback: respect the standard Vault env var so the
		// user doesn't have to retype an address they've already exported.
		addr.SetValue(envAddr)
		if envNS := strings.TrimSpace(os.Getenv("VAULT_NAMESPACE")); envNS != "" {
			ns.SetValue(envNS)
		}
		if envCA := strings.TrimSpace(os.Getenv("VAULT_CACERT")); envCA != "" {
			ca.SetValue(envCA)
		}
	}

	return &ConnectScreen{
		ctx:    ctx,
		theme:  theme,
		inputs: []textinput.Model{addr, ns, ca},
	}
}

func (s *ConnectScreen) Title() string { return "Connect" }

func (s *ConnectScreen) Init() tea.Cmd { return textinput.Blink }

func (s *ConnectScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
		key := m.String()

		// HTTP confirmation prompt — y/n.
		if s.awaitOK {
			switch strings.ToLower(key) {
			case "y":
				s.awaitOK = false
				s.insecure = true
				return s, s.connect()
			case "n", "esc":
				s.awaitOK = false
				return s, nil
			}
			return s, nil
		}

		switch key {
		case "tab", "down":
			s.advanceFocus(1)
			return s, nil
		case "shift+tab", "up":
			s.advanceFocus(-1)
			return s, nil
		case "enter":
			return s, s.submit()
		}
	case connectSuccessMsg:
		s.loading = false
		s.err = nil
		// Record server history before navigating.
		s.ctx.Config.RecordServerUse(s.currentEntry())
		s.ctx.SaveConfig()
		auth := NewAuthScreen(s.ctx, s.theme, m.health)
		return s, func() tea.Msg { return tui.PushScreenMsg{Screen: auth} }
	case connectErrorMsg:
		s.loading = false
		s.err = m.err
		return s, nil
	}

	// Forward to the focused input.
	var cmd tea.Cmd
	for i := range s.inputs {
		if i == s.focus {
			s.inputs[i], cmd = s.inputs[i].Update(msg)
		}
	}
	return s, cmd
}

func (s *ConnectScreen) View() string {
	var b strings.Builder
	b.WriteString(s.theme.RenderLogo())
	b.WriteString("\n\n")
	b.WriteString(s.theme.Subtitle.Render("Connect to a Vault server"))
	b.WriteString("\n\n")
	for _, in := range s.inputs {
		b.WriteString(in.View())
		b.WriteString("\n")
	}
	b.WriteString("\n")
	switch {
	case s.awaitOK:
		b.WriteString(s.theme.Warning.Render(
			"⚠  http:// is plaintext. Type 'y' to connect anyway, 'n' to cancel."))
	case s.loading:
		b.WriteString(s.theme.Hint.Render("connecting..."))
	case s.err != nil:
		b.WriteString(s.theme.Error.Render(s.err.Error()))
	default:
		b.WriteString(s.theme.Hint.Render("tab/shift+tab to move, enter to connect"))
	}
	b.WriteString("\n")
	return b.String()
}

func (s *ConnectScreen) advanceFocus(delta int) {
	s.inputs[s.focus].Blur()
	s.focus = (s.focus + delta + len(s.inputs)) % len(s.inputs)
	s.inputs[s.focus].Focus()
}

func (s *ConnectScreen) currentEntry() config.ServerEntry {
	return config.ServerEntry{
		Address:      strings.TrimSpace(s.inputs[0].Value()),
		Namespace:    strings.TrimSpace(s.inputs[1].Value()),
		CABundlePath: strings.TrimSpace(s.inputs[2].Value()),
	}
}

// submit validates inputs and either prompts for http confirmation or
// kicks off the connect command.
func (s *ConnectScreen) submit() tea.Cmd {
	addr := strings.TrimSpace(s.inputs[0].Value())
	if addr == "" {
		s.err = fmt.Errorf("address is required")
		return nil
	}
	u, err := url.Parse(addr)
	if err != nil {
		s.err = fmt.Errorf("invalid address: %w", err)
		return nil
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		s.err = fmt.Errorf("address must use http:// or https://")
		return nil
	}
	if u.Scheme == "http" && !s.insecure {
		s.awaitOK = true
		return nil
	}
	return s.connect()
}

func (s *ConnectScreen) connect() tea.Cmd {
	addr := strings.TrimSpace(s.inputs[0].Value())
	ns := strings.TrimSpace(s.inputs[1].Value())
	ca := strings.TrimSpace(s.inputs[2].Value())
	s.loading = true
	s.err = nil
	ctx := s.ctx
	return func() tea.Msg {
		if err := ctx.Vault.SetAddress(addr); err != nil {
			return connectErrorMsg{err: err}
		}
		ctx.Vault.SetNamespace(ns)
		if err := ctx.Vault.SetCABundlePath(ca); err != nil {
			return connectErrorMsg{err: err}
		}
		h, err := ctx.Vault.Health(ctx.BackgroundContext())
		if err != nil {
			return connectErrorMsg{err: err}
		}
		if h.Sealed {
			return connectErrorMsg{err: fmt.Errorf("server is sealed")}
		}
		return connectSuccessMsg{health: h}
	}
}

type connectSuccessMsg struct{ health *vault.HealthInfo }
type connectErrorMsg struct{ err error }
