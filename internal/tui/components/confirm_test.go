package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func keyPress(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func sendKey(d *ConfirmDialog, s string) {
	var msg tea.Msg
	switch s {
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		msg = tea.KeyPressMsg{Code: tea.KeyTab}
	case " ":
		msg = tea.KeyPressMsg{Code: ' ', Text: " "}
	default:
		msg = keyPress(s)
	}
	d, _ = d.Update(msg)
	_ = d
}

func TestConfirmDialogYConfirmsNormal(t *testing.T) {
	d := NewConfirmDialog("t", "b", DangerNormal, ConfirmStyles{})
	sendKey(d, "y")
	if !d.Decided() || !d.Confirmed() {
		t.Fatalf("y should confirm normal: decided=%v confirmed=%v", d.Decided(), d.Confirmed())
	}
}

func TestConfirmDialogNCancels(t *testing.T) {
	d := NewConfirmDialog("t", "b", DangerNormal, ConfirmStyles{})
	sendKey(d, "n")
	if !d.Decided() || d.Confirmed() {
		t.Fatalf("n should cancel: decided=%v confirmed=%v", d.Decided(), d.Confirmed())
	}
}

func TestConfirmDialogDestructiveRequiresDESTROY(t *testing.T) {
	d := NewConfirmDialog("t", "b", DangerDestructive, ConfirmStyles{})
	// Pressing y must NOT confirm destructive.
	sendKey(d, "y")
	if d.Decided() {
		t.Fatalf("destructive must not confirm on y")
	}
	// Pressing enter without DESTROY typed must not confirm.
	sendKey(d, "enter")
	if d.Decided() {
		t.Fatalf("destructive must not confirm without DESTROY")
	}
	// Type DESTROY and press enter.
	d.destroyInput.SetValue("DESTROY")
	sendKey(d, "enter")
	if !d.Decided() || !d.Confirmed() {
		t.Fatalf("destructive should confirm after typing DESTROY: decided=%v confirmed=%v", d.Decided(), d.Confirmed())
	}
}

func TestConfirmDialogRememberToggle(t *testing.T) {
	d := NewConfirmDialog("t", "b", DangerNormal, ConfirmStyles{})
	d.OfferRemember = true
	// Drive the state directly: simulate tab, then toggle remember.
	if d.rememberFocused {
		t.Fatalf("expected rememberFocused=false initially")
	}
	d.rememberFocused = true
	d.remember = true
	if !d.Remember() {
		t.Fatalf("expected remember=true")
	}
}
