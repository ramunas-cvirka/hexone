// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
)

type dialogKeyboardFocusState struct {
	tag       uiEventTag
	wantFocus bool
}

type dialogActionVisualState struct {
	Focused bool
	Default bool
}

func (st *dialogKeyboardFocusState) attach(gtx layout.Context) {
	if st == nil {
		return
	}
	event.Op(gtx.Ops, &st.tag)
	if st.wantFocus {
		gtx.Execute(key.FocusCmd{Tag: &st.tag})
		st.wantFocus = false
	}
}

func (st *dialogKeyboardFocusState) focusKeyboard() {
	if st == nil {
		return
	}
	st.wantFocus = true
}

func dialogTabStep(mods key.Modifiers) (int, bool) {
	switch mods {
	case 0:
		return 1, true
	case key.ModShift:
		return -1, true
	default:
		return 0, false
	}
}

func dialogWrappedIndex(current, count, step int) int {
	if count <= 0 {
		return 0
	}
	if current < 0 || current >= count {
		if step < 0 {
			return count - 1
		}
		return 0
	}
	next := current + step
	for next < 0 {
		next += count
	}
	for next >= count {
		next -= count
	}
	return next
}
