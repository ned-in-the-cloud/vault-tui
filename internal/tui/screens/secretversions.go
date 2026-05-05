package screens

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/tui/components"
	"github.com/ned1313/vault-tui/internal/vault"
)

// SecretVersionsScreen lists the version history for a KV-v2 secret.
type SecretVersionsScreen struct {
	ctx   *tui.AppContext
	theme tui.Theme
	mount *vault.MountInfo
	path  string

	versions   []vault.KVVersionMeta
	curVersion int
	loading    bool
	err        error
	idx        int
	statusMsg  string
}

func NewSecretVersionsScreen(ctx *tui.AppContext, theme tui.Theme, mi *vault.MountInfo, path string) tui.Screen {
	return &SecretVersionsScreen{ctx: ctx, theme: theme, mount: mi, path: path, loading: true}
}

func (s *SecretVersionsScreen) Title() string { return "Versions" }
func (s *SecretVersionsScreen) HelpHint() string {
	return "↑/↓ navigate  u undelete  d delete  D destroy  R refresh  esc back"
}

func (s *SecretVersionsScreen) Init() tea.Cmd { return s.loadCmd() }

type versionsLoadedMsg struct {
	versions   []vault.KVVersionMeta
	curVersion int
	err        error
}

func (s *SecretVersionsScreen) loadCmd() tea.Cmd {
	mount := strings.TrimSuffix(s.mount.Path, "/")
	path := s.path
	vc := s.ctx.Vault
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		eng := vc.KVv2(mount)
		md, err := eng.GetMetadata(ctx, path)
		if err != nil {
			return versionsLoadedMsg{err: err}
		}
		out := make([]vault.KVVersionMeta, 0, len(md.Versions))
		for _, v := range md.Versions {
			out = append(out, v)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Version > out[j].Version })
		return versionsLoadedMsg{versions: out, curVersion: md.CurrentVersion}
	}
}

func (s *SecretVersionsScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case versionsLoadedMsg:
		s.loading = false
		s.err = m.err
		s.versions = m.versions
		s.curVersion = m.curVersion
		if s.idx >= len(s.versions) {
			s.idx = 0
		}
		return s, nil
	case opCompletedMsg:
		if m.err != nil {
			s.err = m.err
			return s, nil
		}
		s.statusMsg = m.msg
		s.loading = true
		return s, s.loadCmd()
	case tea.KeyPressMsg:
		return s.handleKey(m.String())
	}
	return s, nil
}

func (s *SecretVersionsScreen) selected() *vault.KVVersionMeta {
	if s.idx < 0 || s.idx >= len(s.versions) {
		return nil
	}
	return &s.versions[s.idx]
}

func (s *SecretVersionsScreen) handleKey(key string) (tui.Screen, tea.Cmd) {
	switch key {
	case "up", "k":
		if s.idx > 0 {
			s.idx--
		}
	case "down", "j":
		if s.idx < len(s.versions)-1 {
			s.idx++
		}
	case "R":
		s.loading = true
		return s, tea.Batch(s.loadCmd(), refreshTokenInfoCmd(s.ctx))
	case "u":
		v := s.selected()
		if v == nil {
			return s, nil
		}
		ver := v.Version
		return s, s.runOp(func(ctx context.Context) error {
			return s.ctx.Vault.KVv2(strings.TrimSuffix(s.mount.Path, "/")).UndeleteVersions(ctx, s.path, []int{ver})
		}, fmt.Sprintf("version %d undeleted", ver))
	case "d":
		v := s.selected()
		if v == nil {
			return s, nil
		}
		ver := v.Version
		return s, PushConfirm(s.ctx, s.theme, ConfirmRequest{
			Title:    fmt.Sprintf("Delete version %d", ver),
			Body:     fmt.Sprintf("Soft-delete %s/%s version %d? It can be undeleted later.", s.mount.Path, s.path, ver),
			Level:    components.DangerNormal,
			Category: ConfirmDeleteVersion,
			OnConfirm: s.runOp(func(ctx context.Context) error {
				return s.ctx.Vault.KVv2(strings.TrimSuffix(s.mount.Path, "/")).DeleteVersions(ctx, s.path, []int{ver})
			}, fmt.Sprintf("version %d deleted", ver)),
		})
	case "D":
		v := s.selected()
		if v == nil {
			return s, nil
		}
		ver := v.Version
		return s, PushConfirm(s.ctx, s.theme, ConfirmRequest{
			Title:    fmt.Sprintf("Destroy version %d", ver),
			Body:     fmt.Sprintf("Permanently destroy %s/%s version %d? Data is unrecoverable.", s.mount.Path, s.path, ver),
			Level:    components.DangerDestructive,
			Category: ConfirmDestroyVersion,
			OnConfirm: s.runOp(func(ctx context.Context) error {
				return s.ctx.Vault.KVv2(strings.TrimSuffix(s.mount.Path, "/")).DestroyVersions(ctx, s.path, []int{ver})
			}, fmt.Sprintf("version %d destroyed", ver)),
		})
	}
	return s, nil
}

func (s *SecretVersionsScreen) runOp(op func(context.Context) error, ok string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := op(ctx); err != nil {
			return opCompletedMsg{err: err}
		}
		return opCompletedMsg{msg: ok}
	}
}

func (s *SecretVersionsScreen) View() string {
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
	if len(s.versions) == 0 {
		b.WriteString(s.theme.Hint.Render("(no versions)"))
		return b.String()
	}
	header := fmt.Sprintf("  %-7s %-25s %-12s", "version", "created", "status")
	b.WriteString(s.theme.Hint.Render(header))
	b.WriteString("\n")
	for i, v := range s.versions {
		marker := "  "
		if i == s.idx {
			marker = "▶ "
		}
		status := "Active"
		if v.Destroyed {
			status = "Destroyed"
		} else if !v.DeletionTime.IsZero() {
			status = "Deleted"
		}
		if v.Version == s.curVersion && status == "Active" {
			status = "Current"
		}
		line := fmt.Sprintf("%s%-7d %-25s %-12s", marker,
			v.Version,
			v.CreatedTime.Local().Format("2006-01-02 15:04:05"),
			status,
		)
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
