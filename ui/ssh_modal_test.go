// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"hexone/fm"
)

func TestSSHModalFocusedEditorKeepsPasteShortcut(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("ssh modal should be open")
	}
	st := ui.sshModal
	th := material.NewTheme()
	router := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(1024, 720),
		},
	}

	layoutModal := func() {
		gtx.Ops.Reset()
		ui.layoutSSHModal(th, gtx)
	}
	frame := func() {
		layoutModal()
		router.Frame(gtx.Ops)
	}

	frame()
	gtx.Execute(key.FocusCmd{Tag: &st.hostEdit})
	frame()
	frame()
	frame()
	if !gtx.Focused(&st.hostEdit) {
		t.Fatal("host editor did not gain focus")
	}

	router.Queue(key.Event{Name: "V", Modifiers: key.ModShortcut, State: key.Press})
	layoutModal()
	if !router.ClipboardRequested() {
		t.Fatal("Ctrl/Cmd+V should reach the focused SSH editor")
	}
}
