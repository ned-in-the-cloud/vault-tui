// Package tui implements the Bubble Tea presentation layer.
package tui

import (
	"context"
	"log/slog"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/vault"
)

// AppContext bundles dependencies passed to every screen constructor.
// It must not be mutated after the TUI starts; the only exception is
// CurrentToken which is updated by the auth flow.
type AppContext struct {
	Vault      vault.Client
	Tokens     secure.TokenStore
	Config     *config.Config
	ConfigPath string
	Paths      config.Paths
	Logger     *slog.Logger
}

// NewContext is a constructor for cleanliness.
func NewContext(c vault.Client, t secure.TokenStore, cfg *config.Config, p config.Paths, cfgPath string, lg *slog.Logger) *AppContext {
	return &AppContext{
		Vault:      c,
		Tokens:     t,
		Config:     cfg,
		ConfigPath: cfgPath,
		Paths:      p,
		Logger:     lg,
	}
}

// SaveConfig persists the current config to disk. Errors are logged at
// warn but not surfaced to callers.
func (a *AppContext) SaveConfig() {
	if err := config.Save(a.ConfigPath, *a.Config); err != nil {
		a.Logger.Warn("save config", "err", err)
	}
}

// BackgroundContext returns a context.Background with no deadline. Future
// versions may carry a cancellable application context.
func (a *AppContext) BackgroundContext() context.Context {
	return context.Background()
}
