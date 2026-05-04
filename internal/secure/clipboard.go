package secure

import (
	"sync"
	"time"

	"golang.design/x/clipboard"
)

var (
	clipInitOnce sync.Once
	clipInitErr  error
)

func ensureClipboard() error {
	clipInitOnce.Do(func() {
		clipInitErr = clipboard.Init()
	})
	return clipInitErr
}

// CopyToClipboard writes data to the system clipboard, then schedules a
// best-effort overwrite of the clipboard contents with empty bytes after
// clearAfter. The supplied data slice is wiped after the clipboard write.
//
// If clearAfter is zero or negative, the auto-clear goroutine is not
// scheduled (the caller is responsible for clearing).
func CopyToClipboard(data []byte, clearAfter time.Duration) error {
	if err := ensureClipboard(); err != nil {
		return err
	}
	clipboard.Write(clipboard.FmtText, data)
	for i := range data {
		data[i] = 0
	}
	if clearAfter <= 0 {
		return nil
	}
	go func() {
		time.Sleep(clearAfter)
		// Best-effort: only clear if no later copy has replaced ours.
		// We keep this simple by always writing empty bytes.
		clipboard.Write(clipboard.FmtText, []byte(""))
	}()
	return nil
}
