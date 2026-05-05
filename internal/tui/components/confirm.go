package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// DangerLevel controls the color and gating of a ConfirmDialog.
type DangerLevel int

const (
	// DangerNormal is for low-risk reversible actions (yellow accent).
	DangerNormal DangerLevel = iota
	// DangerWarning is for higher-impact but recoverable actions (orange).
	DangerWarning
	// DangerDestructive is for irreversible actions; user must type
	// "DESTROY" to enable the confirm key.
	DangerDestructive
)

// ConfirmStyles bundles the lipgloss styles used to render a dialog.
type ConfirmStyles struct {
	BoxNormal      lipgloss.Style
	BoxWarning     lipgloss.Style
	BoxDestructive lipgloss.Style
	Title          lipgloss.Style
	Body           lipgloss.Style
	Hint           lipgloss.Style
	Danger         lipgloss.Style
}

// ConfirmDialog is a state machine for a yes/no prompt with optional
// "don't ask again" toggle and optional DESTROY-to-confirm gating.
//
// The owning screen calls Update(msg) and View(); it inspects Decided()
// after each Update for the result.
type ConfirmDialog struct {
	Title         string
	Body          string
	Level         DangerLevel
	OfferRemember bool

	styles ConfirmStyles

	// Decided/Confirmed/Remember are output state.
	decided   bool
	confirmed bool
	remember  bool

	// destroyInput collects "DESTROY" when Level==Destructive.
	destroyInput textinput.Model
	// rememberFocused: when true, space toggles remember; otherwise
	// y/n/enter make decisions.
	rememberFocused bool
}

// NewConfirmDialog builds a dialog. body lines are rendered as-is.
func NewConfirmDialog(title, body string, level DangerLevel, styles ConfirmStyles) *ConfirmDialog {
	d := &ConfirmDialog{
		Title:  title,
		Body:   body,
		Level:  level,
		styles: styles,
	}
	if level == DangerDestructive {
		ti := textinput.New()
		ti.Prompt = "type DESTROY > "
		ti.CharLimit = 10
		ti.Focus()
		d.destroyInput = ti
	}
	return d
}

// Decided reports whether the user has answered.
func (d *ConfirmDialog) Decided() bool { return d.decided }

// Confirmed is true when the user accepted (only meaningful after Decided).
func (d *ConfirmDialog) Confirmed() bool { return d.confirmed }

// Remember reports whether the user opted into "don't ask again".
func (d *ConfirmDialog) Remember() bool { return d.remember }

// Update handles a tea.Msg routed to the dialog.
func (d *ConfirmDialog) Update(msg tea.Msg) (*ConfirmDialog, tea.Cmd) {
	if d.decided {
		return d, nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		ks := k.String()
		switch ks {
		case "esc":
			d.decided = true
			d.confirmed = false
			return d, nil
		case "tab", "shift+tab":
			if d.OfferRemember {
				d.rememberFocused = !d.rememberFocused
				if d.Level == DangerDestructive {
					if d.rememberFocused {
						d.destroyInput.Blur()
					} else {
						d.destroyInput.Focus()
					}
				}
			}
			return d, nil
		case " ":
			if d.OfferRemember && d.rememberFocused {
				d.remember = !d.remember
				return d, nil
			}
		case "n", "N":
			d.decided = true
			d.confirmed = false
			return d, nil
		case "y", "Y":
			if d.Level != DangerDestructive {
				d.decided = true
				d.confirmed = true
				return d, nil
			}
		case "enter":
			if d.Level == DangerDestructive {
				if strings.TrimSpace(d.destroyInput.Value()) == "DESTROY" {
					d.decided = true
					d.confirmed = true
				}
				return d, nil
			}
			d.decided = true
			d.confirmed = true
			return d, nil
		}
	}
	if d.Level == DangerDestructive && !d.rememberFocused {
		var cmd tea.Cmd
		d.destroyInput, cmd = d.destroyInput.Update(msg)
		return d, cmd
	}
	return d, nil
}

// View renders the dialog as a string.
func (d *ConfirmDialog) View() string {
	box := d.styles.BoxNormal
	switch d.Level {
	case DangerWarning:
		box = d.styles.BoxWarning
	case DangerDestructive:
		box = d.styles.BoxDestructive
	}

	var b strings.Builder
	b.WriteString(d.styles.Title.Render(d.Title))
	b.WriteString("\n\n")
	b.WriteString(d.styles.Body.Render(d.Body))
	b.WriteString("\n\n")
	if d.Level == DangerDestructive {
		b.WriteString(d.styles.Danger.Render("This action cannot be undone."))
		b.WriteString("\n")
		b.WriteString(d.destroyInput.View())
		b.WriteString("\n")
	}
	if d.OfferRemember {
		mark := "[ ]"
		if d.remember {
			mark = "[x]"
		}
		line := fmt.Sprintf("%s don't ask again for this category", mark)
		if d.rememberFocused {
			line = d.styles.Title.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if d.Level == DangerDestructive {
		b.WriteString(d.styles.Hint.Render("type DESTROY then enter to confirm  ·  n/esc cancel  ·  tab toggle remember"))
	} else {
		b.WriteString(d.styles.Hint.Render("y/enter confirm  ·  n/esc cancel  ·  tab toggle remember"))
	}

	return box.Render(b.String())
}
