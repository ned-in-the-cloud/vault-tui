package screens

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

// secretDirectPathPrompt lets users jump to a specific logical path when
// folder browsing is denied by policy but direct access is allowed.
type secretDirectPathPrompt struct {
	ctx     *tui.AppContext
	theme   tui.Theme
	mount   *vault.MountInfo
	version int
	current string
	input   textinput.Model
}

func NewSecretDirectPathPrompt(ctx *tui.AppContext, theme tui.Theme, mount *vault.MountInfo, version int, currentPath string) tui.Screen {
	in := textinput.New()
	in.Prompt = "path > "
	in.CharLimit = 512
	in.SetValue(strings.Trim(currentPath, "/"))
	in.Focus()
	return &secretDirectPathPrompt{
		ctx:     ctx,
		theme:   theme,
		mount:   mount,
		version: version,
		current: strings.Trim(currentPath, "/"),
		input:   in,
	}
}

func (p *secretDirectPathPrompt) Title() string    { return "Direct path" }
func (p *secretDirectPathPrompt) HelpHint() string { return "enter open  esc cancel" }
func (p *secretDirectPathPrompt) Init() tea.Cmd    { return textinput.Blink }

func (p *secretDirectPathPrompt) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch km.String() {
		case "enter":
			nextPath := strings.Trim(p.input.Value(), "/ ")
			if nextPath == p.current {
				return p, func() tea.Msg { return tui.PopScreenMsg{} }
			}
			next := NewSecretListScreen(p.ctx, p.theme, p.mount, p.version, nextPath)
			return p, tea.Sequence(
				func() tea.Msg { return tui.PopScreenMsg{} },
				func() tea.Msg { return tui.PushScreenMsg{Screen: next} },
			)
		case "esc":
			return p, func() tea.Msg { return tui.PopScreenMsg{} }
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p *secretDirectPathPrompt) View() string {
	var b strings.Builder
	b.WriteString(p.theme.Subtitle.Render("Open direct secret path"))
	b.WriteString("\n\n")
	b.WriteString(p.theme.Hint.Render("Enter a path relative to this mount."))
	b.WriteString("\n")
	b.WriteString(p.theme.Hint.Render("Example: app/prod/database"))
	b.WriteString("\n\n")
	b.WriteString(p.input.View())
	return b.String()
}
