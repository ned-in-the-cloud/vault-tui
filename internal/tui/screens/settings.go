package screens

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/tui"
)

type settingsField int

const (
	fieldMaskingStyle settingsField = iota
	fieldClipboardSeconds
	fieldAutoRenew
	fieldExportFormat
	fieldPersistenceMode
	fieldExportDir
	fieldResetConfirmPrefs
	fieldClearHistory
)

var settingsOrder = []settingsField{
	fieldMaskingStyle,
	fieldClipboardSeconds,
	fieldAutoRenew,
	fieldExportFormat,
	fieldPersistenceMode,
	fieldExportDir,
	fieldResetConfirmPrefs,
	fieldClearHistory,
}

type SettingsScreen struct {
	ctx   *tui.AppContext
	theme tui.Theme

	idx int

	editingExportDir bool
	exportDirInput   textinput.Model

	status string
}

func NewSettingsScreen(ctx *tui.AppContext, theme tui.Theme) tui.Screen {
	in := textinput.New()
	in.Prompt = "export dir > "
	in.CharLimit = 1024
	in.SetValue(strings.TrimSpace(ctx.Config.DefaultExportDir))
	return &SettingsScreen{
		ctx:            ctx,
		theme:          theme,
		exportDirInput: in,
	}
}

func (s *SettingsScreen) Title() string { return "Settings" }

func (s *SettingsScreen) HelpHint() string {
	if s.editingExportDir {
		return "enter save  esc cancel"
	}
	return "↑/↓ select  ←/→ toggle  enter action  esc back"
}

func (s *SettingsScreen) Init() tea.Cmd { return textinput.Blink }

func (s *SettingsScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		if s.editingExportDir {
			switch km.String() {
			case "enter":
				s.ctx.Config.DefaultExportDir = strings.TrimSpace(s.exportDirInput.Value())
				s.ctx.SaveConfig()
				s.status = "default export directory saved"
				s.editingExportDir = false
				return s, nil
			case "esc":
				s.editingExportDir = false
				s.exportDirInput.SetValue(strings.TrimSpace(s.ctx.Config.DefaultExportDir))
				return s, nil
			}
			var cmd tea.Cmd
			s.exportDirInput, cmd = s.exportDirInput.Update(msg)
			return s, cmd
		}

		switch km.String() {
		case "up", "k":
			if s.idx > 0 {
				s.idx--
			}
			return s, nil
		case "down", "j":
			if s.idx < len(settingsOrder)-1 {
				s.idx++
			}
			return s, nil
		case "left":
			return s.adjustCurrent(-1)
		case "right", "space":
			return s.adjustCurrent(1)
		case "enter":
			return s.activateCurrent()
		}
	}
	return s, nil
}

func (s *SettingsScreen) adjustCurrent(delta int) (tui.Screen, tea.Cmd) {
	field := settingsOrder[s.idx]
	s.status = ""
	switch field {
	case fieldMaskingStyle:
		if s.ctx.Config.MaskingStyle == config.MaskingStars {
			s.ctx.Config.MaskingStyle = config.MaskingBlank
		} else {
			s.ctx.Config.MaskingStyle = config.MaskingStars
		}
		s.ctx.SaveConfig()
		s.status = "masking style updated"
	case fieldClipboardSeconds:
		next := s.ctx.Config.ClipboardClearSecs + (5 * delta)
		if next < 5 {
			next = 5
		}
		s.ctx.Config.ClipboardClearSecs = next
		s.ctx.SaveConfig()
		s.status = "clipboard clear timeout updated"
	case fieldAutoRenew:
		s.ctx.Config.AutoRenewToken = !s.ctx.Config.AutoRenewToken
		s.ctx.SaveConfig()
		s.status = "auto-renew preference updated"
		return s, tui.AutoRenewChanged()
	case fieldExportFormat:
		formats := []config.ExportFormat{
			config.ExportFormatJSON,
			config.ExportFormatYAML,
			config.ExportFormatDotenv,
			config.ExportFormatSingleKey,
		}
		idx := 0
		for i := range formats {
			if formats[i] == s.ctx.Config.DefaultExportFormat {
				idx = i
				break
			}
		}
		idx = (idx + delta + len(formats)) % len(formats)
		s.ctx.Config.DefaultExportFormat = formats[idx]
		s.ctx.SaveConfig()
		s.status = "default export format updated"
	case fieldPersistenceMode:
		modes := []string{
			string(secure.PersistenceAuto),
			string(secure.PersistenceKeyring),
			string(secure.PersistencePassphrase),
			string(secure.PersistenceNone),
		}
		idx := 0
		for i := range modes {
			if modes[i] == s.ctx.Config.TokenPersistenceMode {
				idx = i
				break
			}
		}
		idx = (idx + delta + len(modes)) % len(modes)
		s.ctx.Config.TokenPersistenceMode = modes[idx]
		s.ctx.SaveConfig()
		s.status = "token persistence mode updated"
	}
	return s, nil
}

func (s *SettingsScreen) activateCurrent() (tui.Screen, tea.Cmd) {
	field := settingsOrder[s.idx]
	s.status = ""
	switch field {
	case fieldExportDir:
		s.editingExportDir = true
		s.exportDirInput.SetValue(strings.TrimSpace(s.ctx.Config.DefaultExportDir))
		s.exportDirInput.Focus()
		return s, textinput.Blink
	case fieldResetConfirmPrefs:
		s.ctx.Config.ConfirmationPrefs = config.ConfirmationPrefs{}
		s.ctx.SaveConfig()
		s.status = "confirmation preferences reset"
	case fieldClearHistory:
		s.ctx.Config.ServerHistory = nil
		s.ctx.SaveConfig()
		s.status = "server history cleared"
	default:
		return s.adjustCurrent(1)
	}
	return s, nil
}

func (s *SettingsScreen) View() string {
	if s.editingExportDir {
		var b strings.Builder
		b.WriteString(s.theme.Subtitle.Render("Default export directory"))
		b.WriteString("\n\n")
		b.WriteString(s.exportDirInput.View())
		b.WriteString("\n\n")
		b.WriteString(s.theme.Hint.Render("leave blank to use ~/.vault-tui/exports"))
		return b.String()
	}

	rows := []string{
		fmt.Sprintf("Masking style           %s", s.ctx.Config.MaskingStyle),
		fmt.Sprintf("Clipboard clear seconds %d", s.ctx.Config.ClipboardClearSecs),
		fmt.Sprintf("Auto-renew token        %v", s.ctx.Config.AutoRenewToken),
		fmt.Sprintf("Default export format   %s", s.ctx.Config.DefaultExportFormat),
		fmt.Sprintf("Token persistence mode  %s", s.ctx.Config.TokenPersistenceMode),
		fmt.Sprintf("Default export dir      %s", blankFallback(s.ctx.Config.DefaultExportDir, "(auto)")),
		"Reset confirmation prefs",
		"Clear server history",
	}

	var b strings.Builder
	b.WriteString(s.theme.Subtitle.Render("Application settings"))
	b.WriteString("\n\n")
	for i := range rows {
		line := "  " + rows[i]
		if i == s.idx {
			line = s.theme.Selection.Render("▶ " + rows[i])
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if s.status != "" {
		b.WriteString("\n")
		b.WriteString(s.theme.Success.Render(s.status))
	}
	return strings.TrimRight(b.String(), "\n")
}

func blankFallback(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}
