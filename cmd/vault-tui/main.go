// Command vault-tui starts the interactive Vault TUI.
package main

import (
	"fmt"
	"log/slog"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/awnumar/memguard"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/logging"
	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/tui/screens"
	"github.com/ned1313/vault-tui/internal/vault"
)

func main() {
	// memguard intercepts fatal signals to wipe enclaves before exit.
	memguard.CatchInterrupt()
	defer memguard.Purge()

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "vault-tui:", err)
		os.Exit(1)
	}
}

func run() error {
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	if err := config.EnsureRoot(paths); err != nil {
		return err
	}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, closer, err := logging.Init(paths.LogFile, slog.LevelInfo)
	if err != nil {
		return err
	}
	defer closer.Close()

	logger.Info("startup", "version", "phase1")

	// Phase 1 keeps the passphrase tier opt-in: only request a passphrase
	// if the keyring isn't usable AND the user has explicitly chosen the
	// passphrase mode. The auto-mode logic lives inside secure.NewTokenStore.
	store, err := secure.NewTokenStore(secure.PersistenceMode(cfg.TokenPersistenceMode), nil, paths.TokensFile)
	if err != nil {
		// Fall back to no persistence rather than refusing to start.
		logger.Warn("token store init failed; persistence disabled", "err", err)
		store, _ = secure.NewTokenStore(secure.PersistenceNone, nil, paths.TokensFile)
	}
	logger.Info("token store", "mode", string(store.Mode()))

	vc, err := vault.NewClient()
	if err != nil {
		return err
	}

	appCtx := tui.NewContext(vc, store, &cfg, paths, paths.ConfigFile, logger)
	theme := tui.DefaultTheme()
	connect := screens.NewConnectScreen(appCtx, theme)
	root := tui.NewRoot(appCtx, connect)

	prog := tea.NewProgram(root)
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("tea program: %w", err)
	}
	return nil
}
