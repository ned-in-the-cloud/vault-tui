package tui

// Global key bindings as plain strings. We compare against tea.KeyPressMsg.
// Keeping these constants centralized makes them easy to change later.
const (
	KeyQuit      = "ctrl+c"
	KeyQuitAlt   = "ctrl+q"
	KeyBack      = "esc"
	KeyHelp      = "?"
	KeyShowCmd   = "alt+x"
	KeyEnter     = "enter"
	KeyTabFwd    = "tab"
	KeyTabBack   = "shift+tab"
	KeyDown      = "down"
	KeyUp        = "up"
	KeySpace     = " "
	KeyNamespace = "N"
	KeyRenew     = "r"
	KeyAutoRenew = "a"
	KeyTokenView = "t"
	KeyEngines   = "e"
	KeySettings  = "s"
)
