package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/config"
	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/tui/components"
	"github.com/ned1313/vault-tui/internal/vault"
)

// SecretExportScreen prompts the user for an export format and output
// path, then writes the secret to disk with restricted permissions.
//
// Phase 5 writes plaintext only; the screen is structured so encrypted
// export can be added in Phase 6 without changing the form layout.
type SecretExportScreen struct {
	ctx     *tui.AppContext
	theme   tui.Theme
	mount   *vault.MountInfo
	version int
	path    string
	keys    []string
	selKey  string

	formats []config.ExportFormat
	fmtIdx  int

	pathInput   textinput.Model
	confirmOver bool // overwrite confirmed for current path
	step        exportStep
	saving      bool
	err         error
	doneMsg     string
}

type exportStep int

const (
	exportSelectFormat exportStep = iota
	exportEnterPath
	exportConfirmOverwrite
)

func NewSecretExportScreen(ctx *tui.AppContext, theme tui.Theme, mi *vault.MountInfo, version int, path string, keys []string, selKey string) tui.Screen {
	pi := textinput.New()
	pi.Prompt = "path > "
	pi.CharLimit = 1024
	formats := []config.ExportFormat{
		config.ExportFormatJSON,
		config.ExportFormatYAML,
		config.ExportFormatDotenv,
		config.ExportFormatSingleKey,
	}
	idx := 0
	def := ctx.Config.DefaultExportFormat
	for i, f := range formats {
		if f == def {
			idx = i
			break
		}
	}
	s := &SecretExportScreen{
		ctx:       ctx,
		theme:     theme,
		mount:     mi,
		version:   version,
		path:      path,
		keys:      keys,
		selKey:    selKey,
		formats:   formats,
		fmtIdx:    idx,
		pathInput: pi,
		step:      exportSelectFormat,
	}
	s.pathInput.SetValue(s.defaultOutPath())
	return s
}

func (s *SecretExportScreen) Title() string { return "Export secret" }
func (s *SecretExportScreen) Init() tea.Cmd { return textinput.Blink }
func (s *SecretExportScreen) HelpHint() string {
	switch s.step {
	case exportSelectFormat:
		return "↑/↓ format  enter next  esc cancel"
	case exportEnterPath:
		return "enter export  esc cancel"
	case exportConfirmOverwrite:
		return "y overwrite  ·  n/esc cancel"
	}
	return "esc cancel"
}

func (s *SecretExportScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case opCompletedMsg:
		s.saving = false
		if m.err != nil {
			s.err = m.err
			return s, nil
		}
		s.doneMsg = m.msg
		return s, tea.Tick(time.Second, func(time.Time) tea.Msg { return tui.PopScreenMsg{} })
	case tea.KeyPressMsg:
		return s.handleKey(m)
	}
	if s.step == exportEnterPath {
		var cmd tea.Cmd
		s.pathInput, cmd = s.pathInput.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s *SecretExportScreen) handleKey(m tea.KeyPressMsg) (tui.Screen, tea.Cmd) {
	ks := m.String()
	switch s.step {
	case exportSelectFormat:
		switch ks {
		case "up", "k":
			if s.fmtIdx > 0 {
				s.fmtIdx--
			}
			s.pathInput.SetValue(s.defaultOutPath())
		case "down", "j":
			if s.fmtIdx < len(s.formats)-1 {
				s.fmtIdx++
			}
			s.pathInput.SetValue(s.defaultOutPath())
		case "enter":
			if s.formats[s.fmtIdx] == config.ExportFormatSingleKey && s.selKey == "" {
				s.err = fmt.Errorf("single-key export requires a selected key")
				return s, nil
			}
			s.step = exportEnterPath
			s.pathInput.Focus()
		}
	case exportEnterPath:
		if ks == "enter" {
			out := strings.TrimSpace(s.pathInput.Value())
			if out == "" {
				s.err = fmt.Errorf("path is required")
				return s, nil
			}
			if _, err := os.Stat(out); err == nil {
				s.step = exportConfirmOverwrite
				s.pathInput.Blur()
				return s, nil
			}
			return s.startExport()
		}
		var cmd tea.Cmd
		s.pathInput, cmd = s.pathInput.Update(m)
		return s, cmd
	case exportConfirmOverwrite:
		switch ks {
		case "y", "Y", "enter":
			s.confirmOver = true
			return s.startExport()
		case "n", "N", "esc":
			s.step = exportEnterPath
			s.pathInput.Focus()
			return s, nil
		}
	}
	return s, nil
}

func (s *SecretExportScreen) defaultOutPath() string {
	dir := s.ctx.Config.DefaultExportDir
	if dir == "" {
		dir = s.ctx.Paths.ExportsDir
	}
	base := strings.ReplaceAll(strings.TrimSuffix(s.mount.Path, "/")+"_"+s.path, "/", "_")
	if base == "" {
		base = "secret"
	}
	switch s.formats[s.fmtIdx] {
	case config.ExportFormatJSON:
		return filepath.Join(dir, base+".json")
	case config.ExportFormatYAML:
		return filepath.Join(dir, base+".yaml")
	case config.ExportFormatDotenv:
		return filepath.Join(dir, base+".env")
	case config.ExportFormatSingleKey:
		k := s.selKey
		if k == "" {
			k = "value"
		}
		return filepath.Join(dir, base+"_"+k+".txt")
	}
	return filepath.Join(dir, base+".txt")
}

func (s *SecretExportScreen) startExport() (tui.Screen, tea.Cmd) {
	s.saving = true
	s.err = nil
	out := strings.TrimSpace(s.pathInput.Value())
	format := s.formats[s.fmtIdx]
	mount := strings.TrimSuffix(s.mount.Path, "/")
	version := s.version
	path := s.path
	selKey := s.selKey
	vc := s.ctx.Vault

	return s, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var sec *vault.KVSecret
		var err error
		if version == 2 {
			sec, err = vc.KVv2(mount).Get(ctx, path)
		} else {
			sec, err = vc.KVv1(mount).Get(ctx, path)
		}
		if err != nil {
			return opCompletedMsg{err: err}
		}
		defer sec.Destroy()

		if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
			return opCompletedMsg{err: err}
		}

		var data []byte
		switch format {
		case config.ExportFormatJSON:
			data, err = renderJSON(sec)
		case config.ExportFormatYAML:
			data, err = renderYAML(sec)
		case config.ExportFormatDotenv:
			data, err = renderDotenv(sec)
		case config.ExportFormatSingleKey:
			data, err = renderSingleKey(sec, selKey)
		default:
			err = fmt.Errorf("unsupported format %q", format)
		}
		if err != nil {
			return opCompletedMsg{err: err}
		}
		// Wrap data in a SecureString for the duration of the file write
		// then destroy the buffer.
		holder := secure.NewSecureString(data)
		defer holder.Destroy()
		writeErr := holder.WithBytes(func(b []byte) error {
			return writePrivateFile(out, b)
		})
		if writeErr != nil {
			return opCompletedMsg{err: writeErr}
		}
		return opCompletedMsg{msg: "exported to " + out}
	}
}

func sortedKeys(sec *vault.KVSecret) []string {
	keys := make([]string, 0, len(sec.Data))
	for k := range sec.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func decodeKey(sec *vault.KVSecret, k string) (interface{}, error) {
	ss := sec.Data[k]
	if ss == nil {
		return nil, nil
	}
	var v interface{}
	err := ss.WithBytes(func(b []byte) error {
		return json.Unmarshal(b, &v)
	})
	return v, err
}

func renderJSON(sec *vault.KVSecret) ([]byte, error) {
	out := make(map[string]interface{}, len(sec.Data))
	for _, k := range sortedKeys(sec) {
		v, err := decodeKey(sec, k)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return json.MarshalIndent(out, "", "  ")
}

func renderYAML(sec *vault.KVSecret) ([]byte, error) {
	// Simple flat YAML rendering: `key: value`. Strings are quoted only
	// when they contain characters that would otherwise cause ambiguity.
	var b strings.Builder
	for _, k := range sortedKeys(sec) {
		v, err := decodeKey(sec, k)
		if err != nil {
			return nil, err
		}
		b.WriteString(yamlKey(k))
		b.WriteString(": ")
		b.WriteString(yamlScalar(v))
		b.WriteString("\n")
	}
	return []byte(b.String()), nil
}

func yamlKey(k string) string {
	if strings.ContainsAny(k, ": #'\"\n\t") {
		j, _ := json.Marshal(k)
		return string(j)
	}
	return k
}

func yamlScalar(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch x := v.(type) {
	case string:
		if x == "" || strings.ContainsAny(x, ":#\n\"'") || strings.HasPrefix(x, " ") || strings.HasSuffix(x, " ") {
			j, _ := json.Marshal(x)
			return string(j)
		}
		return x
	case bool, float64, int, int64:
		return fmt.Sprintf("%v", x)
	default:
		j, _ := json.Marshal(x)
		return string(j)
	}
}

func renderDotenv(sec *vault.KVSecret) ([]byte, error) {
	var b strings.Builder
	for _, k := range sortedKeys(sec) {
		v, err := decodeKey(sec, k)
		if err != nil {
			return nil, err
		}
		envKey := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_", "/", "_").Replace(k))
		var vs string
		switch x := v.(type) {
		case string:
			vs = x
		default:
			j, _ := json.Marshal(x)
			vs = string(j)
		}
		// Escape double quotes and newlines.
		vs = strings.ReplaceAll(vs, `\`, `\\`)
		vs = strings.ReplaceAll(vs, `"`, `\"`)
		vs = strings.ReplaceAll(vs, "\n", `\n`)
		b.WriteString(envKey)
		b.WriteString(`="`)
		b.WriteString(vs)
		b.WriteString("\"\n")
	}
	return []byte(b.String()), nil
}

func renderSingleKey(sec *vault.KVSecret, key string) ([]byte, error) {
	v, err := decodeKey(sec, key)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("key %q not found", key)
	}
	if s, ok := v.(string); ok {
		return []byte(s), nil
	}
	return json.Marshal(v)
}

// writePrivateFile writes data to path with 0o600 permissions on Unix.
// On Windows the file is created with the user's default ACL inherited
// from the parent directory; the parent ~/.vault-tui already restricts
// access to the user.
func writePrivateFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func (s *SecretExportScreen) View() string {
	var b strings.Builder
	bc := components.Breadcrumb{
		Mount:      s.mount.Path,
		Segments:   components.SplitPath(s.path),
		MountStyle: s.theme.Subtitle,
		PathStyle:  s.theme.Field,
		SepStyle:   s.theme.Hint,
	}
	b.WriteString(bc.Render())
	b.WriteString("\n\n")

	switch s.step {
	case exportSelectFormat:
		b.WriteString(s.theme.Subtitle.Render("Select format:"))
		b.WriteString("\n\n")
		labels := map[config.ExportFormat]string{
			config.ExportFormatJSON:      "JSON  (entire secret)",
			config.ExportFormatYAML:      "YAML  (entire secret)",
			config.ExportFormatDotenv:    ".env  (entire secret)",
			config.ExportFormatSingleKey: "Single key (raw plaintext)",
		}
		for i, f := range s.formats {
			marker := "  "
			if i == s.fmtIdx {
				marker = "▶ "
			}
			line := marker + labels[f]
			if f == config.ExportFormatSingleKey {
				if s.selKey != "" {
					line += "  " + s.theme.Hint.Render("→ "+s.selKey)
				} else {
					line += "  " + s.theme.Hint.Render("(select a key first)")
				}
			}
			if i == s.fmtIdx {
				line = s.theme.Selection.Render(marker + labels[f])
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	case exportEnterPath:
		b.WriteString(s.theme.Subtitle.Render("Output path:"))
		b.WriteString("\n\n")
		b.WriteString(s.pathInput.View())
		b.WriteString("\n")
	case exportConfirmOverwrite:
		b.WriteString(s.theme.Warning.Render(fmt.Sprintf("File exists: %s", s.pathInput.Value())))
		b.WriteString("\n\n")
		b.WriteString("Overwrite? (y/n)")
	}

	if s.saving {
		b.WriteString("\n")
		b.WriteString(s.theme.Hint.Render("writing..."))
	}
	if s.err != nil {
		b.WriteString("\n")
		b.WriteString(s.theme.Error.Render(s.err.Error()))
	}
	if s.doneMsg != "" {
		b.WriteString("\n")
		b.WriteString(s.theme.Success.Render(s.doneMsg))
	}
	return b.String()
}
