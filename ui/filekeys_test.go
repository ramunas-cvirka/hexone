// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"testing"

	"gioui.org/io/key"
	"hexone/fm"
)

func TestFileKeyMapUsesFixedNavigationAndFunctionKeys(t *testing.T) {
	m := newFileKeyMap(fm.DefaultConfig())

	tests := []struct {
		name string
		ev   key.Event
		want fileAction
	}{
		{name: "focus next pane", ev: key.Event{Name: key.NameTab, State: key.Press}, want: fileActionFocusNextPane},
		{name: "focus prev pane", ev: key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press}, want: fileActionFocusPrevPane},
		{name: "move up", ev: key.Event{Name: key.NameUpArrow, State: key.Press}, want: fileActionMoveUp},
		{name: "page down", ev: key.Event{Name: key.NamePageDown, State: key.Press}, want: fileActionPageDown},
		{name: "home", ev: key.Event{Name: key.NameHome, State: key.Press}, want: fileActionHome},
		{name: "activate", ev: key.Event{Name: key.NameEnter, State: key.Press}, want: fileActionActivate},
		{name: "parent", ev: key.Event{Name: key.NameDeleteBackward, State: key.Press}, want: fileActionParent},
		{name: "view", ev: key.Event{Name: key.NameF3, State: key.Press}, want: fileActionView},
		{name: "copy", ev: key.Event{Name: key.NameF5, State: key.Press}, want: fileActionCopy},
		{name: "move", ev: key.Event{Name: key.NameF6, State: key.Press}, want: fileActionRenameMove},
		{name: "create", ev: key.Event{Name: key.NameF7, State: key.Press}, want: fileActionCreate},
		{name: "delete", ev: key.Event{Name: key.NameF8, State: key.Press}, want: fileActionDelete},
	}

	for _, tc := range tests {
		got, ok := m.Resolve(tc.ev)
		if !ok {
			t.Fatalf("%s: key %q did not resolve", tc.name, tc.ev.Name)
		}
		if got != tc.want {
			t.Fatalf("%s: action=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestInsertActionUsesFilePaneRepeat(t *testing.T) {
	if !fileActionRepeatable(fileActionMarkSelectNext) {
		t.Fatal("Insert selection action should repeat while held")
	}
}
