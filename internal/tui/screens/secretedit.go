package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/ned1313/vault-tui/internal/secure"
	"github.com/ned1313/vault-tui/internal/tui"
	"github.com/ned1313/vault-tui/internal/tui/components"
	"github.com/ned1313/vault-tui/internal/vault"
)

// SecretEditScreen handles both Create (new secret) and Update
// (overwrite/patch existing secret) flows for v1 and v2 KV mounts.
//
// For v2 update, the user can toggle between Overwrite (replace all
// keys) and Patch (merge with current). v1 supports Overwrite only.
type SecretEditScreen struct {
	ctx     *tui.AppContext
	theme   tui.Theme
	mount   *vault.MountInfo
	version int

	create bool
	// For Patch mode in v2 we need to know the existing key set to label
	// the form correctly.
	existingKeys []string

	// inputs[0] is the path; inputs[1..] alternate key/value.
	pathInput textinput.Model
	pairs     []kvInput
	focus     int // -1 = pathInput, 0..len(pairs)-1 = pair focus index

	patchMode bool // v2 only
	saving    bool
	err       error
}

type kvInput struct {
	key   textinput.Model
	value textinput.Model
	// keyFocused: when true, focus is on the key input; otherwise value.
	keyFocused bool
}

// NewSecretEditScreen builds the edit form. When create is true, path is
// editable and starts at parentPath; when false, path is fixed and a
// patch/overwrite toggle is offered for v2.
func NewSecretEditScreen(ctx *tui.AppContext, theme tui.Theme, mi *vault.MountInfo, version int, parentPath string, existingKeys []string, create bool) tui.Screen {
	pi := textinput.New()
	pi.Prompt = "path > "
	pi.CharLimit = 256
	if create {
		pi.SetValue(parentPath)
		pi.Focus()
	} else {
		pi.SetValue(parentPath)
	}

	s := &SecretEditScreen{
		ctx:          ctx,
		theme:        theme,
		mount:        mi,
		version:      version,
		create:       create,
		existingKeys: existingKeys,
		pathInput:    pi,
		focus:        -1,
	}
	if create {
		// Start with focus on the path input so the user can type the
		// leaf name first; pairs are blurred until tab advances past
		// the path.
		s.pairs = []kvInput{newKVInput()}
		s.pairs[0].keyFocused = true
		s.pairs[0].key.Blur()
		s.pathInput.Focus()
		s.focus = -1
	} else {
		// Update mode: pre-populate one empty pair; for patch mode the
		// form represents only changed keys.
		s.pairs = []kvInput{newKVInput()}
		s.pairs[0].keyFocused = true
		s.pairs[0].key.Focus()
		s.focus = 0
	}
	return s
}

func newKVInput() kvInput {
	k := textinput.New()
	k.Prompt = "key   > "
	k.CharLimit = 256
	v := textinput.New()
	v.Prompt = "value > "
	v.EchoMode = textinput.EchoPassword
	v.EchoCharacter = '•'
	v.CharLimit = 4096
	return kvInput{key: k, value: v, keyFocused: true}
}

func (s *SecretEditScreen) Title() string {
	if s.create {
		return "Create secret"
	}
	if s.patchMode {
		return "Patch secret"
	}
	return "Update secret"
}

func (s *SecretEditScreen) HelpHint() string {
	hints := []string{"tab next", "shift+tab prev", "ctrl+a add pair", "ctrl+d remove pair", "ctrl+s save"}
	if !s.create && s.version == 2 {
		hints = append(hints, "ctrl+t toggle patch")
	}
	hints = append(hints, "esc cancel")
	return strings.Join(hints, "  ·  ")
}

func (s *SecretEditScreen) Init() tea.Cmd { return textinput.Blink }

func (s *SecretEditScreen) Update(msg tea.Msg) (tui.Screen, tea.Cmd) {
	switch m := msg.(type) {
	case opCompletedMsg:
		s.saving = false
		if m.err != nil {
			s.err = m.err
			if cmd := pushReauthIfInvalidToken(s.ctx, s.theme, m.err); cmd != nil {
				return s, cmd
			}
			return s, nil
		}
		return s, func() tea.Msg { return tui.PopScreenMsg{} }
	case tea.KeyPressMsg:
		return s.handleKey(m)
	}
	// Forward to focused input.
	return s.routeMsgToFocus(msg)
}

func (s *SecretEditScreen) handleKey(m tea.KeyPressMsg) (tui.Screen, tea.Cmd) {
	switch m.String() {
	case "tab":
		s.advance(1)
		return s, nil
	case "shift+tab":
		s.advance(-1)
		return s, nil
	case "ctrl+a":
		s.pairs = append(s.pairs, newKVInput())
		return s, nil
	case "ctrl+d":
		if len(s.pairs) > 1 && s.focus >= 0 && s.focus < len(s.pairs) {
			// Zero before drop.
			s.pairs[s.focus].key.SetValue("")
			s.pairs[s.focus].value.SetValue("")
			s.pairs = append(s.pairs[:s.focus], s.pairs[s.focus+1:]...)
			if s.focus >= len(s.pairs) {
				s.focus = len(s.pairs) - 1
			}
			s.refocus()
		}
		return s, nil
	case "ctrl+t":
		if !s.create && s.version == 2 {
			s.patchMode = !s.patchMode
			s.rebuildPairsForMode()
		}
		return s, nil
	case "ctrl+s":
		return s.submit()
	}
	return s.routeMsgToFocus(m)
}

func (s *SecretEditScreen) routeMsgToFocus(msg tea.Msg) (tui.Screen, tea.Cmd) {
	var cmd tea.Cmd
	if s.focus == -1 {
		s.pathInput, cmd = s.pathInput.Update(msg)
		return s, cmd
	}
	if s.focus < 0 || s.focus >= len(s.pairs) {
		return s, nil
	}
	p := &s.pairs[s.focus]
	if p.keyFocused {
		p.key, cmd = p.key.Update(msg)
	} else {
		p.value, cmd = p.value.Update(msg)
	}
	return s, cmd
}

func (s *SecretEditScreen) advance(delta int) {
	// Sequence: pathInput (only in create) -> pair0.key -> pair0.value -> pair1.key -> ...
	totalSteps := len(s.pairs) * 2
	if s.create {
		totalSteps += 1
	}
	cur := s.currentStep()
	cur = (cur + delta + totalSteps) % totalSteps
	s.setStep(cur)
}

func (s *SecretEditScreen) currentStep() int {
	if s.create && s.focus == -1 {
		return 0
	}
	off := 0
	if s.create {
		off = 1
	}
	if s.focus < 0 {
		return off
	}
	step := off + s.focus*2
	if !s.pairs[s.focus].keyFocused {
		step++
	}
	return step
}

func (s *SecretEditScreen) setStep(step int) {
	// Blur all.
	s.pathInput.Blur()
	for i := range s.pairs {
		s.pairs[i].key.Blur()
		s.pairs[i].value.Blur()
	}
	if s.create && step == 0 {
		s.focus = -1
		s.pathInput.Focus()
		return
	}
	off := 0
	if s.create {
		off = 1
	}
	rel := step - off
	pi := rel / 2
	keyFocused := rel%2 == 0
	if pi >= len(s.pairs) {
		pi = len(s.pairs) - 1
	}
	if pi < 0 {
		pi = 0
	}
	s.focus = pi
	s.pairs[pi].keyFocused = keyFocused
	if keyFocused {
		s.pairs[pi].key.Focus()
	} else {
		s.pairs[pi].value.Focus()
	}
}

func (s *SecretEditScreen) refocus() {
	s.pathInput.Blur()
	for i := range s.pairs {
		s.pairs[i].key.Blur()
		s.pairs[i].value.Blur()
	}
	if s.focus < 0 {
		s.pathInput.Focus()
		return
	}
	if s.focus >= len(s.pairs) {
		return
	}
	if s.pairs[s.focus].keyFocused {
		s.pairs[s.focus].key.Focus()
	} else {
		s.pairs[s.focus].value.Focus()
	}
}

func (s *SecretEditScreen) submit() (tui.Screen, tea.Cmd) {
	path := strings.Trim(s.pathInput.Value(), "/ ")
	if path == "" {
		s.err = fmt.Errorf("path is required")
		return s, nil
	}

	// Build SecureString map and zero the textinputs.
	data := make(map[string]*secure.SecureString, len(s.pairs))
	for i := range s.pairs {
		k := strings.TrimSpace(s.pairs[i].key.Value())
		if k == "" {
			continue
		}
		v := s.pairs[i].value.Value()
		// In patch mode, an empty value means "leave this key unchanged"
		// (the row was seeded from the existing secret with a placeholder).
		if s.patchMode && !s.create && v == "" {
			continue
		}
		// JSON-encode strings so PutFromSecure roundtrips through json
		// decoders cleanly. Use json.Marshal so quotes and escapes are
		// handled safely.
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			s.err = err
			return s, nil
		}
		data[k] = secure.NewSecureString(jsonBytes)
		// Zero the textinput value now that it is wrapped.
		s.pairs[i].value.SetValue("")
	}
	if len(data) == 0 {
		s.err = fmt.Errorf("at least one key is required")
		return s, nil
	}

	mount := strings.TrimSuffix(s.mount.Path, "/")
	version := s.version
	patch := s.patchMode
	create := s.create
	vc := s.ctx.Vault

	s.saving = true
	s.err = nil
	return s, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		if version == 2 {
			eng := vc.KVv2(mount)
			if patch && !create {
				// Decode SecureStrings into a plain map for Patch.
				plain, perr := decodeForPatch(data)
				defer destroyPairs(data)
				if perr != nil {
					return opCompletedMsg{err: perr}
				}
				err = eng.Patch(ctx, path, plain, nil)
			} else {
				err = vault.PutFromSecureV2(ctx, eng, path, data, nil)
			}
		} else {
			eng := vc.KVv1(mount)
			err = vault.PutFromSecure(ctx, eng, path, data)
		}
		if err != nil {
			return opCompletedMsg{err: err}
		}
		return opCompletedMsg{msg: "saved"}
	}
}

func decodeForPatch(data map[string]*secure.SecureString) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(data))
	for k, ss := range data {
		var v interface{}
		if err := ss.WithBytes(func(b []byte) error {
			return json.Unmarshal(b, &v)
		}); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

func destroyPairs(data map[string]*secure.SecureString) {
	for _, ss := range data {
		ss.Destroy()
	}
}

// rebuildPairsForMode resets the editable key/value rows to match the
// currently selected mode. In Patch mode each existing key is seeded
// as its own row with a "(unchanged)" value placeholder so the user
// can either type a new value to update it or leave it empty to keep
// the prior value. A trailing empty row is appended so new keys can
// be added without the user manually pressing ctrl+a first. In
// Overwrite mode (or when there are no existing keys to seed) the
// form collapses back to a single empty pair.
func (s *SecretEditScreen) rebuildPairsForMode() {
	if s.patchMode && !s.create && len(s.existingKeys) > 0 {
		pairs := make([]kvInput, 0, len(s.existingKeys)+1)
		for _, k := range s.existingKeys {
			p := newKVInput()
			p.key.SetValue(k)
			p.value.Placeholder = "(unchanged)"
			pairs = append(pairs, p)
		}
		// Trailing empty row so the user can add a brand-new key without
		// having to press ctrl+a first.
		pairs = append(pairs, newKVInput())
		s.pairs = pairs
	} else {
		s.pairs = []kvInput{newKVInput()}
	}
	s.focus = 0
	s.pairs[0].keyFocused = true
	s.refocus()
}

func (s *SecretEditScreen) View() string {
	var b strings.Builder
	bc := components.Breadcrumb{
		Mount:      s.mount.Path,
		Segments:   components.SplitPath(s.pathInput.Value()),
		MountStyle: s.theme.Subtitle,
		PathStyle:  s.theme.Field,
		SepStyle:   s.theme.Hint,
	}
	b.WriteString(bc.Render())
	b.WriteString("\n\n")

	if s.create {
		b.WriteString(s.pathInput.View())
		b.WriteString("\n\n")
	}

	if !s.create && s.version == 2 {
		mode := "Overwrite (replace all keys)"
		if s.patchMode {
			mode = "Patch (merge changes)"
		}
		b.WriteString(s.theme.Hint.Render("mode: " + mode + "  (ctrl+t to toggle)"))
		b.WriteString("\n\n")
	}

	for i := range s.pairs {
		marker := "  "
		if i == s.focus {
			marker = "▶ "
		}
		b.WriteString(marker + s.pairs[i].key.View() + "\n")
		b.WriteString("    " + s.pairs[i].value.View() + "\n")
	}

	b.WriteString("\n")
	if s.saving {
		b.WriteString(s.theme.Hint.Render("saving..."))
	} else if s.err != nil {
		b.WriteString(s.theme.Error.Render(s.err.Error()))
	}

	return b.String()
}
