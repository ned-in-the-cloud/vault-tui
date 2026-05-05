package screens

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

// authStep represents the two-phase flow: pick a method, then fill it in.
type authStep int

const (
	stepSelectMethod authStep = iota
	stepFillCredentials
)

// authMethodOption pairs the human-facing label with the method type id.
type authMethodOption struct {
	label        string
	methodID     config.AuthMethodID
	defaultMount string
}

var authMethodOptions = []authMethodOption{
	{"Token", config.AuthMethodToken, "token"},
	{"UserPass", config.AuthMethodUserPass, "userpass"},
	{"LDAP", config.AuthMethodLDAP, "ldap"},
}

// AuthScreen drives method selection + credential entry.
type AuthScreen struct {
	ctx    *tui.AppContext
	theme  tui.Theme
	health *vault.HealthInfo

	step           authStep
	selected       int
	credentialView *credentialForm

	loading bool
	err     error
}

// NewAuthScreen constructs the auth flow. health is informational and may
// be nil if not yet known.
func NewAuthScreen(ctx *tui.AppContext, theme tui.Theme, health *vault.HealthInfo) tui.Screen {
	// Default selection from config.
	def := 0
	for i, opt := range authMethodOptions {
		if opt.methodID == ctx.Config.DefaultAuthMethod {
			def = i
		}
	}
	return &AuthScreen{
		ctx:      ctx,
		theme:    theme,
		health:   health,
		step:     stepSelectMethod,
		selected: def,
	}
}

func (s *AuthScreen) Title() string { return "Authenticate" }

func (s *AuthScreen) Init() tea.Cmd { return textinput.Blink }

func (s *AuthScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
		key := m.String()
		switch s.step {
		case stepSelectMethod:
			switch key {
			case "up", "k":
				if s.selected > 0 {
					s.selected--
				}
				return s, nil
			case "down", "j":
				if s.selected < len(authMethodOptions)-1 {
					s.selected++
				}
				return s, nil
			case "enter":
				s.credentialView = newCredentialForm(authMethodOptions[s.selected])
				s.step = stepFillCredentials
				return s, textinput.Blink
			}
		case stepFillCredentials:
			switch key {
			case "esc":
				s.step = stepSelectMethod
				s.credentialView = nil
				return s, nil
			case "tab", "down":
				s.credentialView.advance(1)
				return s, nil
			case "shift+tab", "up":
				s.credentialView.advance(-1)
				return s, nil
			case "enter":
				// If the focused field or any earlier required field is
				// empty, advance to the first empty field instead of
				// submitting. This makes it easy to tab through the
				// form using only Enter when fields are blank.
				if idx, blank := s.credentialView.firstEmpty(); blank {
					s.credentialView.focusIndex(idx)
					return s, nil
				}
				return s, s.submit()
			}
		}
	case authSuccessMsg:
		s.loading = false
		s.err = nil
		if err := s.persistToken(m); err != nil {
			s.ctx.Logger.Warn("persist token", "err", err)
		}
		// Update default auth method preference.
		s.ctx.Config.DefaultAuthMethod = authMethodOptions[s.selected].methodID
		s.ctx.SaveConfig()
		dash := NewDashboardScreen(s.ctx, s.theme, m.tokenInfo, s.health)
		return s, func() tea.Msg { return tui.ReplaceScreenMsg{Screen: dash} }
	case authErrorMsg:
		s.loading = false
		s.err = m.err
		return s, nil
	}
	if s.step == stepFillCredentials && s.credentialView != nil {
		var cmd tea.Cmd
		s.credentialView, cmd = s.credentialView.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s *AuthScreen) View() string {
	var b strings.Builder
	if s.health != nil {
		b.WriteString(s.theme.Hint.Render(fmt.Sprintf(
			"connected to vault %s (initialized=%v sealed=%v)",
			s.health.Version, s.health.Initialized, s.health.Sealed)))
		b.WriteString("\n\n")
	}

	switch s.step {
	case stepSelectMethod:
		b.WriteString(s.theme.Subtitle.Render("Select an auth method"))
		b.WriteString("\n\n")
		for i, opt := range authMethodOptions {
			cursor := "  "
			line := opt.label
			if i == s.selected {
				cursor = "▶ "
				line = s.theme.Selection.Render(line)
			}
			b.WriteString(cursor + line + "\n")
		}
		b.WriteString("\n" + s.theme.Hint.Render("up/down to choose, enter to continue"))
	case stepFillCredentials:
		b.WriteString(s.theme.Subtitle.Render(
			"Enter credentials for " + authMethodOptions[s.selected].label))
		b.WriteString("\n\n")
		b.WriteString(s.credentialView.View())
		b.WriteString("\n")
		switch {
		case s.loading:
			b.WriteString(s.theme.Hint.Render("authenticating..."))
		case s.err != nil:
			b.WriteString(s.theme.Error.Render(s.err.Error()))
		default:
			b.WriteString(s.theme.Hint.Render(
				"tab/shift+tab to move, enter to log in, esc to back up"))
		}
	}
	return b.String()
}

func (s *AuthScreen) submit() tea.Cmd {
	method, err := s.credentialView.buildAuthMethod(s.ctx)
	if err != nil {
		s.err = err
		return nil
	}
	s.loading = true
	s.err = nil
	ctx := s.ctx.BackgroundContext()
	vc := s.ctx.Vault
	return func() tea.Msg {
		ti, err := vc.Login(ctx, method)
		method.Destroy()
		if err != nil {
			return authErrorMsg{err: err}
		}
		return authSuccessMsg{
			methodID:   authMethodOptions[s.selected].methodID,
			methodPath: authMethodOptions[s.selected].defaultMount,
			identifier: ti.DisplayName,
			tokenInfo:  ti,
		}
	}
}

// persistToken writes the freshly issued token to the active store and
// updates the matching ServerHistory entry.
func (s *AuthScreen) persistToken(m authSuccessMsg) error {
	addr := s.ctx.Vault.Address()
	if addr == "" {
		return fmt.Errorf("no server address recorded")
	}
	identifier := m.identifier
	if identifier == "" {
		identifier = m.tokenInfo.Accessor
	}
	key := secure.TokenKey(addr, string(m.methodID), identifier)
	if err := s.ctx.Tokens.Set(key, m.tokenInfo.ID); err != nil {
		return err
	}
	// Update history with last identifier.
	for i := range s.ctx.Config.ServerHistory {
		if s.ctx.Config.ServerHistory[i].Address == addr &&
			s.ctx.Config.ServerHistory[i].Namespace == s.ctx.Vault.Namespace() {
			s.ctx.Config.ServerHistory[i].AuthMethod = m.methodID
			s.ctx.Config.ServerHistory[i].AuthPath = m.methodPath
			s.ctx.Config.ServerHistory[i].LastIdentifier = identifier
		}
	}
	return nil
}

type authSuccessMsg struct {
	methodID   config.AuthMethodID
	methodPath string
	identifier string
	tokenInfo  *vault.TokenInfo
}

type authErrorMsg struct{ err error }

// ---- credential form ----

// credentialForm builds inputs based on the selected method type.
type credentialForm struct {
	option authMethodOption
	inputs []textinput.Model
	labels []string
	focus  int
}

func newCredentialForm(opt authMethodOption) *credentialForm {
	cf := &credentialForm{option: opt}
	mountInput := textinput.New()
	mountInput.Prompt = "mount    > "
	mountInput.SetValue(opt.defaultMount)
	mountInput.CharLimit = 100

	switch opt.methodID {
	case config.AuthMethodToken:
		tok := textinput.New()
		tok.Prompt = "token    > "
		tok.EchoMode = textinput.EchoPassword
		tok.EchoCharacter = '•'
		tok.CharLimit = 200
		// Pre-populate from VAULT_TOKEN if exported. The masked echo mode
		// keeps the value off-screen even when prefilled.
		if envTok := strings.TrimSpace(os.Getenv("VAULT_TOKEN")); envTok != "" {
			tok.SetValue(envTok)
		}
		tok.Focus()
		cf.inputs = []textinput.Model{tok}
		cf.labels = []string{"token"}
	case config.AuthMethodUserPass, config.AuthMethodLDAP:
		user := textinput.New()
		user.Prompt = "username > "
		user.CharLimit = 100
		user.Focus()
		pass := textinput.New()
		pass.Prompt = "password > "
		pass.EchoMode = textinput.EchoPassword
		pass.EchoCharacter = '•'
		pass.CharLimit = 200
		cf.inputs = []textinput.Model{user, pass, mountInput}
		cf.labels = []string{"username", "password", "mount"}
	}
	return cf
}

func (c *credentialForm) advance(delta int) {
	if len(c.inputs) == 0 {
		return
	}
	c.inputs[c.focus].Blur()
	c.focus = (c.focus + delta + len(c.inputs)) % len(c.inputs)
	c.inputs[c.focus].Focus()
}

// firstEmpty returns the index of the first input whose trimmed value
// is empty. Any field whose label is "mount" is skipped because mount
// inputs are pre-populated with sensible defaults. Returns ok=false
// when every required field is filled in.
func (c *credentialForm) firstEmpty() (int, bool) {
	for i := range c.inputs {
		if i < len(c.labels) && c.labels[i] == "mount" {
			continue
		}
		if strings.TrimSpace(c.inputs[i].Value()) == "" {
			return i, true
		}
	}
	return 0, false
}

// focusIndex moves focus to the given input index, blurring all others.
func (c *credentialForm) focusIndex(i int) {
	if i < 0 || i >= len(c.inputs) {
		return
	}
	for j := range c.inputs {
		c.inputs[j].Blur()
	}
	c.focus = i
	c.inputs[i].Focus()
}

func (c *credentialForm) Update(msg tea.Msg) (*credentialForm, tea.Cmd) {
	var cmd tea.Cmd
	for i := range c.inputs {
		if i == c.focus {
			c.inputs[i], cmd = c.inputs[i].Update(msg)
		}
	}
	return c, cmd
}

func (c *credentialForm) View() string {
	var b strings.Builder
	for _, in := range c.inputs {
		b.WriteString(in.View())
		b.WriteString("\n")
	}
	return b.String()
}

// buildAuthMethod constructs a vault.AuthMethod from the form contents.
// The credential text inputs are reset (zeroed) after construction.
func (c *credentialForm) buildAuthMethod(_ *tui.AppContext) (vault.AuthMethod, error) {
	switch c.option.methodID {
	case config.AuthMethodToken:
		token := strings.TrimSpace(c.inputs[0].Value())
		if token == "" {
			return nil, fmt.Errorf("token is required")
		}
		c.inputs[0].SetValue("")
		return vault.NewTokenAuth([]byte(token)), nil
	case config.AuthMethodUserPass:
		user := strings.TrimSpace(c.inputs[0].Value())
		pass := c.inputs[1].Value()
		mount := strings.TrimSpace(c.inputs[2].Value())
		if user == "" || pass == "" {
			return nil, fmt.Errorf("username and password are required")
		}
		c.inputs[1].SetValue("")
		return vault.NewUserPassAuth(mount, user, []byte(pass)), nil
	case config.AuthMethodLDAP:
		user := strings.TrimSpace(c.inputs[0].Value())
		pass := c.inputs[1].Value()
		mount := strings.TrimSpace(c.inputs[2].Value())
		if user == "" || pass == "" {
			return nil, fmt.Errorf("username and password are required")
		}
		c.inputs[1].SetValue("")
		return vault.NewLDAPAuth(mount, user, []byte(pass)), nil
	}
	return nil, fmt.Errorf("unsupported auth method")
}
