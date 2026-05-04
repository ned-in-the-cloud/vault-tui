// Package config manages user preferences persisted to
// ~/.vault-tui/config.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Defaults.
const (
	DefaultClipboardClearSecs = 15
	DefaultDir                = ".vault-tui"
	configFileName            = "config.json"
	tokensFileName            = "tokens.enc"
	logFileName               = "vault-tui.log"
	exportsDirName            = "exports"
)

// MaskingStyle controls how secret inputs are rendered.
type MaskingStyle string

const (
	MaskingStars MaskingStyle = "stars"
	MaskingBlank MaskingStyle = "blank"
)

// ExportFormat is the default file-export format.
type ExportFormat string

const (
	ExportFormatJSON      ExportFormat = "json"
	ExportFormatYAML      ExportFormat = "yaml"
	ExportFormatDotenv    ExportFormat = "dotenv"
	ExportFormatSingleKey ExportFormat = "single-key"
)

// AuthMethodID identifies an auth method type for defaults.
type AuthMethodID string

const (
	AuthMethodToken    AuthMethodID = "token"
	AuthMethodUserPass AuthMethodID = "userpass"
	AuthMethodLDAP     AuthMethodID = "ldap"
)

// ConfirmationPrefs holds independent suppression flags per category.
// Destroy operations always require typing "DESTROY" regardless of these
// flags.
type ConfirmationPrefs struct {
	SkipDeleteVersion  bool `json:"skip_delete_version"`
	SkipDestroyVersion bool `json:"skip_destroy_version"`
	SkipDeleteAll      bool `json:"skip_delete_all"`
	SkipDestroyAll     bool `json:"skip_destroy_all"`
}

// ServerEntry is one row in the connection history.
type ServerEntry struct {
	Address        string       `json:"address"`
	Namespace      string       `json:"namespace,omitempty"`
	CABundlePath   string       `json:"ca_bundle_path,omitempty"`
	LastUsed       time.Time    `json:"last_used"`
	AuthMethod     AuthMethodID `json:"auth_method,omitempty"`
	AuthPath       string       `json:"auth_path,omitempty"`
	LastIdentifier string       `json:"last_identifier,omitempty"`
}

// Config is the top-level user-configuration document.
type Config struct {
	MaskingStyle              MaskingStyle      `json:"masking_style"`
	ClipboardClearSecs        int               `json:"clipboard_clear_secs"`
	ConfirmationPrefs         ConfirmationPrefs `json:"confirmation_prefs"`
	ServerHistory             []ServerEntry     `json:"server_history"`
	DefaultAuthMethod         AuthMethodID      `json:"default_auth_method,omitempty"`
	AutoRenewToken            bool              `json:"auto_renew_token"`
	DefaultExportDir          string            `json:"default_export_dir,omitempty"`
	DefaultExportFormat       ExportFormat      `json:"default_export_format"`
	PreferEncryptedFileExport bool              `json:"prefer_encrypted_file_export"`
	TokenPersistenceMode      string            `json:"token_persistence_mode"` // matches secure.PersistenceMode values
}

// Default returns a Config populated with built-in defaults.
func Default() Config {
	return Config{
		MaskingStyle:         MaskingStars,
		ClipboardClearSecs:   DefaultClipboardClearSecs,
		DefaultAuthMethod:    AuthMethodToken,
		AutoRenewToken:       true,
		DefaultExportFormat:  ExportFormatJSON,
		TokenPersistenceMode: "auto",
	}
}

// Paths bundles file-system locations derived from the user home dir.
type Paths struct {
	Root       string // ~/.vault-tui
	ConfigFile string
	TokensFile string
	LogFile    string
	ExportsDir string
}

// ResolvePaths returns the standard set of paths under the user home dir.
func ResolvePaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("config: user home: %w", err)
	}
	root := filepath.Join(home, DefaultDir)
	return Paths{
		Root:       root,
		ConfigFile: filepath.Join(root, configFileName),
		TokensFile: filepath.Join(root, tokensFileName),
		LogFile:    filepath.Join(root, logFileName),
		ExportsDir: filepath.Join(root, exportsDirName),
	}, nil
}

// EnsureRoot creates the ~/.vault-tui directory if missing.
func EnsureRoot(p Paths) error {
	return os.MkdirAll(p.Root, 0o700)
}

// Load reads the config file at the given path. If the file does not
// exist, defaults are returned (and the returned error is nil).
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("config: read: %w", err)
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("config: decode: %w", err)
	}
	cfg.normalize()
	return cfg, nil
}

// Save writes the config to path with private file permissions.
func Save(path string, cfg Config) error {
	cfg.normalize()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("config: rename: %w", err)
	}
	return nil
}

func (c *Config) normalize() {
	if c.MaskingStyle == "" {
		c.MaskingStyle = MaskingStars
	}
	if c.ClipboardClearSecs <= 0 {
		c.ClipboardClearSecs = DefaultClipboardClearSecs
	}
	if c.DefaultExportFormat == "" {
		c.DefaultExportFormat = ExportFormatJSON
	}
	if c.TokenPersistenceMode == "" {
		c.TokenPersistenceMode = "auto"
	}
}

// RecordServerUse adds or refreshes a server entry in the history. The
// most recently used entry sorts first.
func (c *Config) RecordServerUse(entry ServerEntry) {
	entry.LastUsed = time.Now()
	for i, e := range c.ServerHistory {
		if e.Address == entry.Address && e.Namespace == entry.Namespace {
			c.ServerHistory[i] = entry
			c.sortHistory()
			return
		}
	}
	c.ServerHistory = append(c.ServerHistory, entry)
	c.sortHistory()
}

func (c *Config) sortHistory() {
	sort.SliceStable(c.ServerHistory, func(i, j int) bool {
		return c.ServerHistory[i].LastUsed.After(c.ServerHistory[j].LastUsed)
	})
}
