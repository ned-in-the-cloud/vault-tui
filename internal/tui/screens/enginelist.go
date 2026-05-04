package screens

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/vault"
)

// EngineListScreen lists secrets engines mounted on the connected
// Vault server. KV mounts are selectable (open the path browser); other
// types are listed as informational only with a "(not yet supported)"
// hint, matching the Phase 6 extensibility goal.
type EngineListScreen struct {
	ctx     *tui.AppContext
	theme   tui.Theme
	mounts  []*vault.MountInfo
	loading bool
	err     error
	idx     int
}

// NewEngineListScreen builds the screen and immediately schedules a
// ListMounts call via Init.
func NewEngineListScreen(ctx *tui.AppContext, theme tui.Theme) tui.Screen {
	return &EngineListScreen{ctx: ctx, theme: theme, loading: true}
}

func (s *EngineListScreen) Title() string { return "Secrets engines" }
func (s *EngineListScreen) HelpHint() string {
	return "↑/↓ select  enter open  N namespace  R refresh  esc back"
}

func (s *EngineListScreen) Init() tea.Cmd { return s.loadCmd() }

// --- messages ---

type mountsLoadedMsg struct {
	mounts []*vault.MountInfo
	err    error
}

func (s *EngineListScreen) loadCmd() tea.Cmd {
	vc := s.ctx.Vault
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		m, err := vc.ListMounts(ctx)
		if err != nil {
			return mountsLoadedMsg{err: err}
		}
		out := make([]*vault.MountInfo, 0, len(m))
		for _, mi := range m {
			out = append(out, mi)
		}
		sort.Slice(out, func(i, j int) bool {
			// KV mounts first, then alphabetical.
			ki, kj := vault.IsKV(out[i]), vault.IsKV(out[j])
			if ki != kj {
				return ki
			}
			return out[i].Path < out[j].Path
		})
		return mountsLoadedMsg{mounts: out}
	}
}

func (s *EngineListScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case mountsLoadedMsg:
		s.loading = false
		s.err = m.err
		s.mounts = m.mounts
		s.idx = 0
		return s, nil
	case namespaceChangedMsg:
		s.loading = true
		s.err = nil
		return s, s.loadCmd()
	case tea.KeyPressMsg:
		return s.handleKey(m.String())
	}
	return s, nil
}

func (s *EngineListScreen) handleKey(key string) (tui.Screen, tea.Cmd) {
	switch key {
	case "up", "k":
		if s.idx > 0 {
			s.idx--
		}
	case "down", "j":
		if s.idx < len(s.mounts)-1 {
			s.idx++
		}
	case "enter":
		if s.idx < 0 || s.idx >= len(s.mounts) {
			return s, nil
		}
		mi := s.mounts[s.idx]
		v := vault.KVVersion(mi)
		if v == 0 {
			s.err = fmt.Errorf("%q (%s) is not yet supported", mi.Path, mi.Type)
			return s, nil
		}
		next := NewSecretListScreen(s.ctx, s.theme, mi, v, "")
		return s, func() tea.Msg { return tui.PushScreenMsg{Screen: next} }
	case "R":
		s.loading = true
		s.err = nil
		return s, s.loadCmd()
	case tui.KeyNamespace:
		return s, func() tea.Msg {
			return tui.PushScreenMsg{Screen: NewNamespacePrompt(s.ctx, s.theme)}
		}
	}
	return s, nil
}

// namespaceChangedMsg is published by the namespace prompt when the
// user accepts a new namespace; engine list listens for it to reload.
type namespaceChangedMsg struct{}

func (s *EngineListScreen) View() string {
	var b strings.Builder
	b.WriteString(s.theme.Subtitle.Render("Secrets engines"))
	b.WriteString("\n\n")
	if s.loading {
		b.WriteString(s.theme.Hint.Render("loading mounts..."))
		return b.String()
	}
	if s.err != nil {
		b.WriteString(s.theme.Error.Render(s.err.Error()))
		b.WriteString("\n\n")
	}
	if len(s.mounts) == 0 {
		b.WriteString(s.theme.Hint.Render("(no mounts visible)"))
		return b.String()
	}
	for i, mi := range s.mounts {
		ver := vault.KVVersion(mi)
		marker := "  "
		if i == s.idx {
			marker = "▶ "
		}
		var line string
		if ver > 0 {
			line = fmt.Sprintf("%s%-20s kv v%d   %s", marker, mi.Path, ver, mi.Description)
		} else {
			line = fmt.Sprintf("%s%-20s %-7s %s", marker, mi.Path, mi.Type, "(not yet supported)")
		}
		if i == s.idx {
			line = s.theme.Selection.Render(line)
		} else if ver == 0 {
			line = s.theme.Hint.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
