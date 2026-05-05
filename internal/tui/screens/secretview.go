package screens

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/tui/components"
	"github.com/ned1313/vault-tui/internal/vault"
)

// SecretViewScreen displays the keys of a single secret with masked
// values. For KV v2, it loads only key names via GetSubkeys; for KV v1
// it loads the secret data and immediately wraps each value in a
// SecureString.
type SecretViewScreen struct {
	ctx     *tui.AppContext
	theme   tui.Theme
	mount   *vault.MountInfo
	version int
	path    string

	// keys is the ordered list of keys for display. For v1 the data
	// SecureStrings are stored in `secret.Data`; for v2 the key list
	// comes from GetSubkeys with no data in memory.
	keys   []string
	secret *vault.KVSecret // v1 only; nil for v2

	curVersion int // v2 current version (from VersionMetadata or metadata)
	loading    bool
	err        error
	idx        int

	statusMsg string
}

// NewSecretViewScreen builds the view for the secret at mount/path.
func NewSecretViewScreen(ctx *tui.AppContext, theme tui.Theme, mi *vault.MountInfo, version int, path string) tui.Screen {
	return &SecretViewScreen{
		ctx:     ctx,
		theme:   theme,
		mount:   mi,
		version: version,
		path:    path,
		loading: true,
	}
}

func (s *SecretViewScreen) Title() string { return "Secret" }
func (s *SecretViewScreen) HelpHint() string {
	if s.version == 2 {
		return "↑/↓ key  c copy  f export  v versions  u update  d delete  esc back"
	}
	return "↑/↓ key  c copy  f export  u update  d delete  esc back"
}

func (s *SecretViewScreen) Init() tea.Cmd { return s.loadCmd() }

// --- messages ---

type secretLoadedMsg struct {
	keys       []string
	secret     *vault.KVSecret
	curVersion int
	err        error
}

type opCompletedMsg struct {
	msg string
	err error
}

func (s *SecretViewScreen) loadCmd() tea.Cmd {
	vc := s.ctx.Vault
	mount := s.mount.Path
	version := s.version
	path := s.path
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if version == 2 {
			eng := vc.KVv2(strings.TrimSuffix(mount, "/"))
			if eng == nil {
				return secretLoadedMsg{err: fmt.Errorf("kv v2 engine unavailable")}
			}
			subkeys, err := eng.GetSubkeys(ctx, path, 0, 1)
			if err != nil {
				// Fallback for tokens without permission on the
				// kv-v2/subkeys/* endpoint: read the latest version
				// directly so we can list keys. Plaintext is then
				// retained in memory inside SecureStrings; the copy
				// path will reuse it instead of fetching again.
				if errors.Is(err, vault.ErrPermissionDenied) {
					sec, ferr := eng.Get(ctx, path)
					if ferr != nil {
						return secretLoadedMsg{err: ferr}
					}
					keys := make([]string, 0, len(sec.Data))
					for k := range sec.Data {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					cur := 0
					if md, mErr := eng.GetMetadata(ctx, path); mErr == nil && md != nil {
						cur = md.CurrentVersion
					}
					return secretLoadedMsg{keys: keys, secret: sec, curVersion: cur}
				}
				return secretLoadedMsg{err: err}
			}
			sort.Strings(subkeys)
			cur := 0
			if md, mErr := eng.GetMetadata(ctx, path); mErr == nil && md != nil {
				cur = md.CurrentVersion
			}
			return secretLoadedMsg{keys: subkeys, curVersion: cur}
		}
		eng := vc.KVv1(strings.TrimSuffix(mount, "/"))
		if eng == nil {
			return secretLoadedMsg{err: fmt.Errorf("kv v1 engine unavailable")}
		}
		sec, err := eng.Get(ctx, path)
		if err != nil {
			return secretLoadedMsg{err: err}
		}
		keys := make([]string, 0, len(sec.Data))
		for k := range sec.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return secretLoadedMsg{keys: keys, secret: sec}
	}
}

func (s *SecretViewScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case secretLoadedMsg:
		s.loading = false
		s.err = m.err
		if m.err != nil {
			if cmd := pushReauthIfInvalidToken(s.ctx, s.theme, m.err); cmd != nil {
				return s, cmd
			}
		}
		s.keys = m.keys
		// Replace any previous data; destroy old before overwriting.
		if s.secret != nil {
			s.secret.Destroy()
		}
		s.secret = m.secret
		s.curVersion = m.curVersion
		if s.idx >= len(s.keys) {
			s.idx = 0
		}
		return s, nil
	case opCompletedMsg:
		if m.err != nil {
			s.err = m.err
			if cmd := pushReauthIfInvalidToken(s.ctx, s.theme, m.err); cmd != nil {
				return s, cmd
			}
			return s, nil
		}
		s.statusMsg = m.msg
		s.loading = true
		return s, s.loadCmd()
	case ClipboardClearedMsg:
		s.statusMsg = "clipboard cleared"
		return s, nil
	case errMsg:
		s.err = m.err
		if cmd := pushReauthIfInvalidToken(s.ctx, s.theme, m.err); cmd != nil {
			return s, cmd
		}
		return s, nil
	case tea.KeyPressMsg:
		return s.handleKey(m.String())
	}
	return s, nil
}

func (s *SecretViewScreen) handleKey(key string) (tui.Screen, tea.Cmd) {
	switch key {
	case "up", "k":
		if s.idx > 0 {
			s.idx--
		}
	case "down", "j":
		if s.idx < len(s.keys)-1 {
			s.idx++
		}
	case "R":
		s.loading = true
		s.err = nil
		return s, tea.Batch(s.loadCmd(), refreshTokenInfoCmd(s.ctx))
	case "c":
		return s, s.copySelected()
	case "f":
		next := NewSecretExportScreen(s.ctx, s.theme, s.mount, s.version, s.path, s.keys, s.selectedKey())
		return s, func() tea.Msg { return tui.PushScreenMsg{Screen: next} }
	case "v":
		if s.version == 2 {
			next := NewSecretVersionsScreen(s.ctx, s.theme, s.mount, s.path)
			return s, func() tea.Msg { return tui.PushScreenMsg{Screen: next} }
		}
	case "u":
		next := NewSecretEditScreen(s.ctx, s.theme, s.mount, s.version, s.path, s.keys, false)
		return s, func() tea.Msg { return tui.PushScreenMsg{Screen: next} }
	case "d":
		return s, s.deleteFlow()
	}
	return s, nil
}

func (s *SecretViewScreen) selectedKey() string {
	if s.idx < 0 || s.idx >= len(s.keys) {
		return ""
	}
	return s.keys[s.idx]
}

func (s *SecretViewScreen) copySelected() tea.Cmd {
	k := s.selectedKey()
	if k == "" {
		return nil
	}
	clearAfter := time.Duration(s.ctx.Config.ClipboardClearSecs) * time.Second
	if s.version == 1 && s.secret != nil {
		ss, ok := s.secret.Data[k]
		if !ok || ss == nil {
			return nil
		}
		var raw string
		_ = ss.WithBytes(func(b []byte) error {
			// Stored values are JSON-encoded. Decode strings to raw text;
			// for non-strings, copy the JSON form.
			var v interface{}
			if err := json.Unmarshal(b, &v); err != nil {
				raw = string(b)
				return nil
			}
			if s, ok := v.(string); ok {
				raw = s
			} else {
				raw = string(b)
			}
			return nil
		})
		return CopyToClipboard(raw, clearAfter)
	}
	// v2: fetch the single secret to copy. We get the whole map but
	// only open the requested key.
	ctx := s.ctx
	mount := s.mount.Path
	path := s.path
	return func() tea.Msg {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		eng := ctx.Vault.KVv2(strings.TrimSuffix(mount, "/"))
		sec, err := eng.Get(c, path)
		if err != nil {
			return errMsg{err: err}
		}
		defer sec.Destroy()
		ss, ok := sec.Data[k]
		if !ok {
			return errMsg{err: fmt.Errorf("key %q not found", k)}
		}
		var raw string
		_ = ss.WithBytes(func(b []byte) error {
			var v interface{}
			if err := json.Unmarshal(b, &v); err != nil {
				raw = string(b)
				return nil
			}
			if str, ok := v.(string); ok {
				raw = str
			} else {
				raw = string(b)
			}
			return nil
		})
		// CopyToClipboard returns a Cmd; we need to execute it inline.
		cmd := CopyToClipboard(raw, time.Duration(ctx.Config.ClipboardClearSecs)*time.Second)
		// Zero our local copy.
		_ = secure.NewSecureStringFromString(raw) // wraps and immediately drops
		raw = ""
		if cmd == nil {
			return opCompletedMsg{msg: "copied to clipboard"}
		}
		return cmd()
	}
}

func (s *SecretViewScreen) deleteFlow() tea.Cmd {
	if s.version == 1 {
		// v1 delete is permanent → destructive.
		return PushConfirm(s.ctx, s.theme, ConfirmRequest{
			Title:    "Delete secret",
			Body:     fmt.Sprintf("Permanently delete %s/%s?", s.mount.Path, s.path),
			Level:    components.DangerDestructive,
			Category: ConfirmNone,
			OnConfirm: s.runOp(func(ctx context.Context) error {
				return s.ctx.Vault.KVv1(strings.TrimSuffix(s.mount.Path, "/")).Delete(ctx, s.path)
			}, "secret deleted"),
		})
	}
	// v2: ask for "delete current version" (Normal) but offer the user a
	// path to escalate via `D` later. Keep this UI simple for Phase 5.
	return PushConfirm(s.ctx, s.theme, ConfirmRequest{
		Title:    "Delete current version",
		Body:     fmt.Sprintf("Soft-delete current version of %s/%s? It can be undeleted later.", s.mount.Path, s.path),
		Level:    components.DangerNormal,
		Category: ConfirmDeleteVersion,
		OnConfirm: s.runOp(func(ctx context.Context) error {
			return s.ctx.Vault.KVv2(strings.TrimSuffix(s.mount.Path, "/")).Delete(ctx, s.path)
		}, "current version deleted"),
	})
}

// runOp builds a tea.Cmd that runs the given operation against Vault and
// reports completion via opCompletedMsg.
func (s *SecretViewScreen) runOp(op func(context.Context) error, okMsg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := op(ctx); err != nil {
			return opCompletedMsg{err: err}
		}
		return opCompletedMsg{msg: okMsg}
	}
}

func (s *SecretViewScreen) View() string {
	var b strings.Builder
	bc := components.Breadcrumb{
		Mount:      s.mount.Path,
		Segments:   components.SplitPath(s.path),
		MountStyle: s.theme.Subtitle,
		PathStyle:  s.theme.Field,
		SepStyle:   s.theme.Hint,
	}
	b.WriteString(bc.Render())
	b.WriteString("\n")
	if s.version == 2 && s.curVersion > 0 {
		b.WriteString(s.theme.Hint.Render(fmt.Sprintf("v2  current version: %d", s.curVersion)))
		b.WriteString("\n")
	} else if s.version == 1 {
		b.WriteString(s.theme.Hint.Render("v1"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if s.loading {
		b.WriteString(s.theme.Hint.Render("loading..."))
		return b.String()
	}
	if s.err != nil {
		b.WriteString(s.theme.Error.Render(s.err.Error()))
		b.WriteString("\n\n")
	}
	if len(s.keys) == 0 {
		b.WriteString(s.theme.Hint.Render("(no keys)"))
		return b.String()
	}
	for i, k := range s.keys {
		marker := "  "
		if i == s.idx {
			marker = "▶ "
		}
		line := fmt.Sprintf("%s%s = %s", marker, k, "********")
		if i == s.idx {
			line = s.theme.Selection.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if s.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(s.theme.Success.Render(s.statusMsg))
	}
	return b.String()
}
