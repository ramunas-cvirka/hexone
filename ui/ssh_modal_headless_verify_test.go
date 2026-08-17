// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build sshverify

package ui

import (
	"fmt"
	"hexone/fm"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestHeadlessSSHModalScrollbarAndHover(t *testing.T) {
	const width, height = 1024, 720
	outDir := os.Getenv("SSH_MODAL_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer win.Release()

	cfg := fm.DefaultConfig()
	for i := 1; i <= 24; i++ {
		cfg.SSH.Setups = append(cfg.SSH.Setups, fm.SSHSetup{
			Host: fmt.Sprintf("node-%02d.internal", i),
			Port: 22,
			User: "deploy",
		})
	}
	ui := NewUI(cfg)
	ui.openSSHModal()
	if ui.sshModal == nil {
		t.Fatal("SSH modal did not open")
	}
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	router := new(input.Router)
	base := time.Now()
	frame := func(at time.Time) *image.RGBA {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Now:         at,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, height)),
		}
		ui.Layout(th, gtx)
		router.Frame(&ops)
		if err := win.Frame(&ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		return img
	}
	write := func(name string, img image.Image) {
		file, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, img); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	img := frame(base)
	img = frame(base.Add(160 * time.Millisecond))
	if got := ui.sshModal.setupList.Position.Count; got >= len(cfg.SSH.Setups) {
		t.Fatalf("setup list shows %d of %d rows; fixture did not overflow", got, len(cfg.SSH.Setups))
	}
	write("ssh-setups-scrollbar.png", img)

	// The second saved setup sits around y=300 in the centered 760x280 dialog.
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(230, 300)})
	frame(base.Add(176 * time.Millisecond))
	write("ssh-setups-row-hover-mid.png", frame(base.Add(220*time.Millisecond)))
	write("ssh-setups-row-hover.png", frame(base.Add(320*time.Millisecond)))

	// The remove action shares the row but sits immediately left of the scrollbar.
	router.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(380, 300)})
	frame(base.Add(336 * time.Millisecond))
	write("ssh-setups-remove-hover.png", frame(base.Add(480*time.Millisecond)))

	// Clicking low in the visible scrollbar track pages the real Gio list.
	beforeScroll := ui.sshModal.setupList.Position.First
	router.Queue(pointer.Event{Kind: pointer.Press, Position: f32.Pt(398, 446), Buttons: pointer.ButtonPrimary})
	frame(base.Add(496 * time.Millisecond))
	router.Queue(pointer.Event{Kind: pointer.Release, Position: f32.Pt(398, 446)})
	frame(base.Add(512 * time.Millisecond))
	img = frame(base.Add(528 * time.Millisecond))
	if got := ui.sshModal.setupList.Position.First; got <= beforeScroll {
		t.Fatalf("scrollbar click left first row at %d, want > %d", got, beforeScroll)
	}
	write("ssh-setups-scrollbar-paged.png", img)
	t.Logf("wrote SSH modal frames to %s", outDir)
}
