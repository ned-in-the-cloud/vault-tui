package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Breadcrumb renders a navigation trail of path segments separated by
// styled chevrons. The mount segment is highlighted to distinguish it
// from logical sub-paths.
type Breadcrumb struct {
	Mount    string
	Segments []string

	MountStyle lipgloss.Style
	PathStyle  lipgloss.Style
	SepStyle   lipgloss.Style
}

// Render produces the breadcrumb string.
func (b Breadcrumb) Render() string {
	sep := b.SepStyle.Render(" › ")
	parts := []string{b.MountStyle.Render(b.Mount + "/")}
	for _, s := range b.Segments {
		if s == "" {
			continue
		}
		parts = append(parts, b.PathStyle.Render(s))
	}
	return strings.Join(parts, sep)
}

// SplitPath splits a logical Vault key path like "foo/bar/baz" into
// segments, dropping empties.
func SplitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// JoinPath rejoins segments with single slashes.
func JoinPath(segments []string) string {
	return strings.Join(segments, "/")
}
