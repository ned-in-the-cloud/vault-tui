package screens

import (
	"context"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/tui/components"
	"github.com/ned1313/vault-tui/internal/vault"
)

// SecretListScreen browses a single KV mount's logical path tree. Each
// instance is anchored to one path; drilling into a folder pushes a
// new SecretListScreen. Pop/Backspace returns to the parent.
type SecretListScreen struct {
	ctx     *tui.AppContext
	theme   tui.Theme
	mount   *vault.MountInfo
	version int
	path    string

	keys    []string
	loading bool
	err     error
	idx     int

	filter   string
	filtered []string
}

// NewSecretListScreen builds the browser for the mount at the given
// logical path.
func NewSecretListScreen(ctx *tui.AppContext, theme tui.Theme, mi *vault.MountInfo, version int, path string) tui.Screen {
	return &SecretListScreen{
		ctx:     ctx,
		theme:   theme,
		mount:   mi,
		version: version,
		path:    path,
		loading: true,
	}
}

func (s *SecretListScreen) Title() string { return "Secrets" }
func (s *SecretListScreen) HelpHint() string {
	return "↑/↓ navigate  enter open  backspace up  / filter  R refresh  esc back"
}

func (s *SecretListScreen) Init() tea.Cmd { return s.loadCmd() }

// --- messages ---

type keysLoadedMsg struct {
	path string
	keys []string
	err  error
}

func (s *SecretListScreen) loadCmd() tea.Cmd {
	vc := s.ctx.Vault
	mount := s.mount.Path
	version := s.version
	key := s.path
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ks, err := vc.ListKV(ctx, mount, version, key)
		if err != nil {
			return keysLoadedMsg{path: key, err: err}
		}
		sort.Slice(ks, func(i, j int) bool {
			fi, fj := strings.HasSuffix(ks[i], "/"), strings.HasSuffix(ks[j], "/")
			if fi != fj {
				return fi // folders first
			}
			return ks[i] < ks[j]
		})
		return keysLoadedMsg{path: key, keys: ks}
	}
}

func (s *SecretListScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case keysLoadedMsg:
		s.loading = false
		s.err = m.err
		s.keys = m.keys
		s.idx = 0
		s.applyFilter()
		return s, nil
	case tea.KeyPressMsg:
		return s.handleKey(m.String())
	}
	return s, nil
}

func (s *SecretListScreen) handleKey(key string) (tui.Screen, tea.Cmd) {
	switch key {
	case "up", "k":
		if s.idx > 0 {
			s.idx--
		}
	case "down", "j":
		if s.idx < len(s.filtered)-1 {
			s.idx++
		}
	case "enter":
		if s.idx < 0 || s.idx >= len(s.filtered) {
			return s, nil
		}
		k := s.filtered[s.idx]
		if strings.HasSuffix(k, "/") {
			child := strings.TrimSuffix(s.path, "/")
			if child != "" {
				child += "/"
			}
			child += k
			child = strings.TrimSuffix(child, "/")
			next := NewSecretListScreen(s.ctx, s.theme, s.mount, s.version, child)
			return s, func() tea.Msg { return tui.PushScreenMsg{Screen: next} }
		}
		// Leaf: push the secret view screen.
		leaf := strings.TrimSuffix(s.path, "/")
		if leaf != "" {
			leaf += "/"
		}
		leaf += k
		next := NewSecretViewScreen(s.ctx, s.theme, s.mount, s.version, leaf)
		return s, func() tea.Msg { return tui.PushScreenMsg{Screen: next} }
	case "backspace":
		return s, func() tea.Msg { return tui.PopScreenMsg{} }
	case "R":
		s.loading = true
		s.err = nil
		return s, s.loadCmd()
	case "/":
		// Toggle a tiny inline filter prompt: for now pre-seed with
		// current filter so a second slash clears it.
		if s.filter != "" {
			s.filter = ""
			s.applyFilter()
		}
		return s, nil
	}
	return s, nil
}

func (s *SecretListScreen) applyFilter() {
	if s.filter == "" {
		s.filtered = s.keys
		return
	}
	out := make([]string, 0, len(s.keys))
	q := strings.ToLower(s.filter)
	for _, k := range s.keys {
		if strings.Contains(strings.ToLower(k), q) {
			out = append(out, k)
		}
	}
	s.filtered = out
}

func (s *SecretListScreen) View() string {
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

	if s.loading {
		b.WriteString(s.theme.Hint.Render("loading..."))
		return b.String()
	}
	if s.err != nil {
		b.WriteString(s.theme.Error.Render(s.err.Error()))
		b.WriteString("\n\n")
	}
	if len(s.filtered) == 0 {
		b.WriteString(s.theme.Hint.Render("(empty)"))
		return b.String()
	}
	for i, k := range s.filtered {
		marker := "  "
		if i == s.idx {
			marker = "▶ "
		}
		isFolder := strings.HasSuffix(k, "/")
		var line string
		if isFolder {
			line = marker + s.theme.Field.Render(k)
		} else {
			line = marker + k
		}
		if i == s.idx {
			line = s.theme.Selection.Render(stripStyle(marker + k))
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// stripStyle returns a plain version of a label for use inside a
// selection style (which already supplies foreground/background).
func stripStyle(s string) string { return s }
