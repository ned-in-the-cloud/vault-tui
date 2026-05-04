package screens

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/tui"
)

// namespacePrompt is a tiny inline prompt screen pushed when the user
// presses N to change the active Vault namespace.
type namespacePrompt struct {
	ctx     *tui.AppContext
	theme   tui.Theme
	input   textinput.Model
	current string
}

// NewNamespacePrompt constructs the prompt prefilled with the current
// namespace.
func NewNamespacePrompt(ctx *tui.AppContext, theme tui.Theme) tui.Screen {
	in := textinput.New()
	in.Prompt = "namespace> "
	in.Placeholder = "(blank to clear)"
	in.CharLimit = 200
	in.SetValue(ctx.Vault.Namespace())
	in.Focus()
	return &namespacePrompt{
		ctx:     ctx,
		theme:   theme,
		input:   in,
		current: ctx.Vault.Namespace(),
	}
}

func (p *namespacePrompt) Title() string { return "Switch namespace" }
func (p *namespacePrompt) Init() tea.Cmd { return textinput.Blink }
func (p *namespacePrompt) HelpHint() string {
	return "enter accept  esc cancel"
}

func (p *namespacePrompt) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch km.String() {
		case "enter":
			ns := strings.TrimSpace(p.input.Value())
			p.ctx.Vault.SetNamespace(ns)
			p.ctx.Logger.Info("namespace switched", "from", p.current, "to", ns)
			return p, func() tea.Msg { return tui.PopScreenMsg{} }
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p *namespacePrompt) View() string {
	var b strings.Builder
	b.WriteString(p.theme.Subtitle.Render("Switch Vault namespace"))
	b.WriteString("\n\n")
	b.WriteString(p.input.View())
	b.WriteString("\n\n")
	b.WriteString(p.theme.Hint.Render(
		"applies only to subsequent calls; servers and tokens remain unchanged"))
	return b.String()
}
