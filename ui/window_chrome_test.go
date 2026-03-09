package ui

import (
	"testing"

	"gioui.org/io/system"
)

func TestConsumeWindowActionsClearsPendingBits(t *testing.T) {
	ui := &UI{}
	ui.requestWindowAction(system.ActionMinimize)
	ui.requestWindowAction(system.ActionMaximize)

	if got := ui.ConsumeWindowActions(); got != system.ActionMinimize|system.ActionMaximize {
		t.Fatalf("ConsumeWindowActions() = %v, want %v", got, system.ActionMinimize|system.ActionMaximize)
	}
	if got := ui.ConsumeWindowActions(); got != 0 {
		t.Fatalf("ConsumeWindowActions() after drain = %v, want 0", got)
	}
}

func TestConsumeWindowCloseRequestKeepsOtherActions(t *testing.T) {
	ui := &UI{}
	ui.requestWindowAction(system.ActionClose)
	ui.requestWindowAction(system.ActionMaximize)

	if !ui.ConsumeWindowCloseRequest() {
		t.Fatal("close request should be pending")
	}
	if got := ui.ConsumeWindowActions(); got != system.ActionMaximize {
		t.Fatalf("remaining actions = %v, want %v", got, system.ActionMaximize)
	}
}
