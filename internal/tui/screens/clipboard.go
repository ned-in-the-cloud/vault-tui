package screens

import (
	"time"

	tea "charm.land/bubbletea/v2"

	clip "github.com/atotto/clipboard"
)

// ClipboardClearedMsg is published when an auto-clear timer fires and the
// clipboard has been wiped (best-effort).
type ClipboardClearedMsg struct{}

// CopyToClipboard writes the given text to the system clipboard and
// returns a tea.Cmd that, after `clearAfter`, replaces the clipboard with
// an empty string and emits ClipboardClearedMsg. If clearAfter <= 0 the
// auto-clear is skipped.
//
// The value bytes are not retained by this helper after the call. Callers
// owning a SecureString should open it, pass the bytes here, and destroy
// the source immediately.
func CopyToClipboard(value string, clearAfter time.Duration) tea.Cmd {
	if err := clip.WriteAll(value); err != nil {
		return func() tea.Msg { return errMsg{err: err} }
	}
	if clearAfter <= 0 {
		return nil
	}
	return tea.Tick(clearAfter, func(time.Time) tea.Msg {
		_ = clip.WriteAll("")
		return ClipboardClearedMsg{}
	})
}

// errMsg is a local copy so we don't depend on the tui package's private
// errorMsg type. Screens that want to surface the error should translate
// to tui.ShowError when they observe it.
type errMsg struct{ err error }

// Err returns the wrapped error.
func (e errMsg) Err() error { return e.err }
