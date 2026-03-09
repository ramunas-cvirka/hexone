package table

import (
	"testing"
	"time"

	"gioui.org/widget"
)

func TestResetPointerStateClearsClickMemory(t *testing.T) {
	tbl := New(nil)
	tbl.rowClicks = []widget.Clickable{{}, {}}
	tbl.lastClickRow = 1
	tbl.lastClickAt = time.Now()

	tbl.ResetPointerState()

	if tbl.lastClickRow != -1 {
		t.Fatalf("lastClickRow = %d, want -1", tbl.lastClickRow)
	}
	if !tbl.lastClickAt.IsZero() {
		t.Fatal("lastClickAt should be cleared")
	}
	if len(tbl.rowClicks) != 2 {
		t.Fatalf("row click count = %d, want 2", len(tbl.rowClicks))
	}
}
