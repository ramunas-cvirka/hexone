package ui

import (
	"hexone/fm"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
)

type fileAction uint8

const (
	fileActionFocusNextPane fileAction = iota
	fileActionFocusPrevPane
	fileActionMoveUp
	fileActionMoveDown
	fileActionMoveLeft
	fileActionMoveRight
	fileActionPageUp
	fileActionPageDown
	fileActionHome
	fileActionEnd
	fileActionActivate
)

type fileKeyBinding struct {
	Name     key.Name
	Required key.Modifiers
}

type fileActionBinding struct {
	Action   fileAction
	Bindings []fileKeyBinding
}

type fileKeyMap struct {
	entries []fileActionBinding
	filters []event.Filter
}

func newFileKeyMap(cfg *fm.Config) fileKeyMap {
	if cfg == nil {
		cfg = fm.DefaultConfig()
	}

	source := cfg.KeyBindings
	specs := []struct {
		action   fileAction
		raw      string
		fallback string
	}{
		{action: fileActionFocusNextPane, raw: source.FocusNextPane, fallback: "tab"},
		{action: fileActionFocusPrevPane, raw: source.FocusPrevPane, fallback: "shift+tab"},
		{action: fileActionMoveUp, raw: source.MoveUp, fallback: "up"},
		{action: fileActionMoveDown, raw: source.MoveDown, fallback: "down"},
		{action: fileActionMoveLeft, raw: source.MoveLeft, fallback: "left"},
		{action: fileActionMoveRight, raw: source.MoveRight, fallback: "right"},
		{action: fileActionPageUp, raw: source.PageUp, fallback: "pgup"},
		{action: fileActionPageDown, raw: source.PageDown, fallback: "pgdown"},
		{action: fileActionHome, raw: source.Home, fallback: "home"},
		{action: fileActionEnd, raw: source.End, fallback: "end"},
		{action: fileActionActivate, raw: source.Activate, fallback: "enter"},
	}

	out := fileKeyMap{
		entries: make([]fileActionBinding, 0, len(specs)),
		filters: make([]event.Filter, 0, len(specs)),
	}
	for _, spec := range specs {
		bindings, ok := parseFileKeyBindings(spec.raw)
		if !ok {
			bindings, _ = parseFileKeyBindings(spec.fallback)
		}
		out.entries = append(out.entries, fileActionBinding{
			Action:   spec.action,
			Bindings: bindings,
		})
		for _, binding := range bindings {
			out.filters = append(out.filters, key.Filter{
				Name:     binding.Name,
				Required: binding.Required,
				Optional: binding.Required,
			})
		}
	}
	return out
}

func (m fileKeyMap) Filters() []event.Filter {
	return m.filters
}

func (m fileKeyMap) Resolve(ev key.Event) (fileAction, bool) {
	for _, entry := range m.entries {
		for _, binding := range entry.Bindings {
			if ev.Name != binding.Name {
				continue
			}
			if ev.Modifiers != binding.Required {
				continue
			}
			return entry.Action, true
		}
	}
	return 0, false
}

func parseFileKeyBindings(raw string) ([]fileKeyBinding, bool) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return nil, false
	}

	parts := strings.Split(raw, "+")
	binding := fileKeyBinding{}
	keyNameSet := false

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "shift":
			binding.Required |= key.ModShift
		case "ctrl", "control":
			binding.Required |= key.ModCtrl
		case "alt":
			binding.Required |= key.ModAlt
		case "super", "win":
			binding.Required |= key.ModSuper
		case "cmd", "command":
			binding.Required |= key.ModCommand
		default:
			name, ok := fileKeyName(part)
			if !ok {
				return nil, false
			}
			binding.Name = name
			keyNameSet = true
		}
	}

	if !keyNameSet {
		return nil, false
	}
	bindings := []fileKeyBinding{binding}
	if binding.Name == key.NameEnter {
		bindings = appendUniqueKeyBinding(bindings, fileKeyBinding{
			Name:     key.NameReturn,
			Required: binding.Required,
		})
	} else if binding.Name == key.NameReturn {
		bindings = appendUniqueKeyBinding(bindings, fileKeyBinding{
			Name:     key.NameEnter,
			Required: binding.Required,
		})
	}
	return bindings, true
}

func fileKeyName(part string) (key.Name, bool) {
	switch part {
	case "up", "uparrow":
		return key.NameUpArrow, true
	case "down", "downarrow":
		return key.NameDownArrow, true
	case "left", "leftarrow":
		return key.NameLeftArrow, true
	case "right", "rightarrow":
		return key.NameRightArrow, true
	case "pgup", "pageup":
		return key.NamePageUp, true
	case "pgdown", "pgdn", "pagedown":
		return key.NamePageDown, true
	case "home":
		return key.NameHome, true
	case "end":
		return key.NameEnd, true
	case "enter", "return":
		return key.NameEnter, true
	case "tab":
		return key.NameTab, true
	default:
		return "", false
	}
}

func fileActionKey(action fileAction) string {
	switch action {
	case fileActionFocusNextPane:
		return "focus-next-pane"
	case fileActionFocusPrevPane:
		return "focus-prev-pane"
	case fileActionMoveUp:
		return "move-up"
	case fileActionMoveDown:
		return "move-down"
	case fileActionMoveLeft:
		return "move-left"
	case fileActionMoveRight:
		return "move-right"
	case fileActionPageUp:
		return "page-up"
	case fileActionPageDown:
		return "page-down"
	case fileActionHome:
		return "home"
	case fileActionEnd:
		return "end"
	case fileActionActivate:
		return "activate"
	default:
		return ""
	}
}

func appendUniqueKeyBinding(dst []fileKeyBinding, binding fileKeyBinding) []fileKeyBinding {
	for _, existing := range dst {
		if existing.Name == binding.Name && existing.Required == binding.Required {
			return dst
		}
	}
	return append(dst, binding)
}

func fileActionRepeatable(action fileAction) bool {
	switch action {
	case fileActionMoveUp, fileActionMoveDown, fileActionMoveLeft, fileActionMoveRight, fileActionPageUp, fileActionPageDown:
		return true
	default:
		return false
	}
}

func fileActionCommand(action fileAction) string {
	switch action {
	case fileActionMoveUp:
		return "↑"
	case fileActionMoveDown:
		return "↓"
	case fileActionMoveLeft:
		return "←"
	case fileActionMoveRight:
		return "→"
	case fileActionPageUp:
		return "⇞"
	case fileActionPageDown:
		return "⇟"
	case fileActionHome:
		return "⇱"
	case fileActionEnd:
		return "⇲"
	case fileActionActivate:
		return "⏎"
	default:
		return ""
	}
}
