package tui

import tea "charm.land/bubbletea/v2"

// Screen is what every page in the TUI implements. It's a strict subset
// of tea.Model so the root model can keep a stack of screens and route
// messages to the topmost one.
type Screen interface {
	Init() tea.Cmd
	Update(tea.Msg) (Screen, tea.Cmd)
	View() string
	// Title is shown in the status bar / window header.
	Title() string
}

// PushScreenMsg pushes a new screen on the stack.
type PushScreenMsg struct{ Screen Screen }

// PopScreenMsg pops the current screen.
type PopScreenMsg struct{}

// ReplaceScreenMsg replaces the entire stack with a single screen.
type ReplaceScreenMsg struct{ Screen Screen }

// QuitMsg requests a clean shutdown.
type QuitMsg struct{}

// ScreenStack is a simple LIFO of screens. The zero value is usable.
type ScreenStack struct {
	stack []Screen
}

func (s *ScreenStack) Push(scr Screen) { s.stack = append(s.stack, scr) }
func (s *ScreenStack) Pop() (Screen, bool) {
	if len(s.stack) == 0 {
		return nil, false
	}
	top := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return top, true
}
func (s *ScreenStack) Replace(scr Screen) {
	s.stack = []Screen{scr}
}
func (s *ScreenStack) Current() Screen {
	if len(s.stack) == 0 {
		return nil
	}
	return s.stack[len(s.stack)-1]
}
func (s *ScreenStack) Len() int { return len(s.stack) }
