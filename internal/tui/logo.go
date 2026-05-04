package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// vaultLogoLines is an ASCII rendering of the HashiCorp Vault wordmark
// with a stylized "V" shield. Designed to fit within ~60 columns so it
// renders cleanly inside narrow terminals.
var vaultLogoLines = []string{
`____   _________   ____ ___.____  ___________`,
`\   \ /   /  _  \ |    |   \    | \__    ___/`,
` \   Y   /  /_\  \|    |   /    |   |    |   `,
`  \     /    |    \    |  /|    |___|    |   `,
`   \___/\____|__  /______/ |_______ \____|   `,
`                \/                 \/        `,
}

// RenderLogo returns the Vault ASCII logo styled with the theme's brand
// colors. A subtle gradient is applied across the lines for visual flair.
func (t Theme) RenderLogo() string {
	// Vault brand-adjacent palette: bright yellow fading to gold.
	shades := []string{
		"#FFEC6E",
		"#FFE259",
		"#FCD34D",
		"#FBBF24",
		"#F59E0B",
	}
	var b strings.Builder
	for i, line := range vaultLogoLines {
		c := lipgloss.Color(shades[i%len(shades)])
		b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(line))
		b.WriteString("\n")
	}
	tag := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A1A1AA")).
		Italic(true).
		Render("        a terminal UI for HashiCorp Vault")
	b.WriteString(tag)
	return b.String()
}
