package tui

import "charm.land/lipgloss/v2"

// Theme bundles the styles used across the app. Built once at startup
// and shared by all screens.
type Theme struct {
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Body      lipgloss.Style
	Hint      lipgloss.Style
	Error     lipgloss.Style
	Warning   lipgloss.Style
	Success   lipgloss.Style
	Field     lipgloss.Style
	Selection lipgloss.Style
	Box       lipgloss.Style
	StatusBar lipgloss.Style
}

// DefaultTheme returns a theme tuned for both light and dark terminals,
// using a HashiCorp Vault-inspired palette: bright yellow on dark.
func DefaultTheme() Theme {
	// Vault brand-adjacent palette.
	primary := lipgloss.Color("#FFEC6E") // vault yellow
	accent := lipgloss.Color("#FBBF24")  // amber
	muted := lipgloss.Color("#A1A1AA")
	red := lipgloss.Color("#F87171")
	yellow := lipgloss.Color("#FCD34D")
	green := lipgloss.Color("#34D399")

	return Theme{
		Title:    lipgloss.NewStyle().Bold(true).Foreground(primary),
		Subtitle: lipgloss.NewStyle().Bold(true).Foreground(accent),
		Body:     lipgloss.NewStyle(),
		Hint:     lipgloss.NewStyle().Foreground(muted).Italic(true),
		Error:    lipgloss.NewStyle().Foreground(red).Bold(true),
		Warning:  lipgloss.NewStyle().Foreground(yellow).Bold(true),
		Success:  lipgloss.NewStyle().Foreground(green).Bold(true),
		Field:    lipgloss.NewStyle().Foreground(accent),
		Selection: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(primary).
			Bold(true),
		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Padding(0, 1),
		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(primary).
			Bold(true).
			Padding(0, 1),
	}
}
