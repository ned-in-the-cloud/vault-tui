package components

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestSplitPath(t *testing.T) {
	cases := map[string][]string{
		"":           nil,
		"/":          nil,
		"foo":        {"foo"},
		"foo/bar":    {"foo", "bar"},
		"/a/b/c/":    {"a", "b", "c"},
		"app/db/cfg": {"app", "db", "cfg"},
	}
	for in, want := range cases {
		got := SplitPath(in)
		if !equal(got, want) {
			t.Errorf("SplitPath(%q) = %v want %v", in, got, want)
		}
	}
}

func TestBreadcrumbRender(t *testing.T) {
	bc := Breadcrumb{
		Mount:      "secret",
		Segments:   []string{"app", "db"},
		MountStyle: lipgloss.NewStyle(),
		PathStyle:  lipgloss.NewStyle(),
		SepStyle:   lipgloss.NewStyle(),
	}
	got := bc.Render()
	want := "secret/ › app › db"
	if got != want {
		t.Errorf("Render = %q want %q", got, want)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
