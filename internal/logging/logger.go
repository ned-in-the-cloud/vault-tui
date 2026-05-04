// Package logging configures slog with a file-only sink under
// ~/.vault-tui/vault-tui.log.
//
// Redaction rules enforced by callers (this package does not inspect
// values it is asked to log):
//   - Never log secret values.
//   - Log secret keys only at debug.
//   - Log paths at info.
//   - Never echo response bodies.
//   - Never log tokens or accessors above debug.
package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Init opens the log file and returns a configured *slog.Logger plus a
// closer that should be invoked at program exit.
func Init(logFile string, level slog.Level) (*slog.Logger, io.Closer, error) {
	if logFile == "" {
		return nil, nil, errors.New("logging: empty log file path")
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o700); err != nil {
		return nil, nil, fmt.Errorf("logging: mkdir: %w", err)
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("logging: open: %w", err)
	}
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	return logger, f, nil
}
