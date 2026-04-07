// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"errors"
	"fmt"
	"hexone/ui/platform"
	"image"
	"image/color"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/pkg/sftp"
)

const (
	filePaneVolumeBadgeRefreshInterval = 15 * time.Second
	filePaneVolumeBadgeRetryInterval   = 4 * time.Second
	filePaneVolumeBadgeRemoteTimeout   = 4 * time.Second
)

type filePaneVolumeBadgeState struct {
	lookupPath    string
	label         string
	checkedAt     time.Time
	nextRefreshAt time.Time
	measuredLabel string
	measuredWidth int
	measuredPxDp  float32
	measuredPxSp  float32
}

var localVolumeUsageFunc = platform.LocalVolumeUsage
var remoteVolumeUsageFunc = remoteVolumeUsage

func (p *filePaneState) invalidateVolumeBadge() {
	if p == nil {
		return
	}
	p.volumeBadge.checkedAt = time.Time{}
	p.volumeBadge.nextRefreshAt = time.Time{}
}

func (ui *UI) filePaneVolumeBadgeLabel(pane *filePaneState, now time.Time) (string, time.Time, bool) {
	if pane == nil || pane.archiveBrowsing() {
		return "", time.Time{}, false
	}

	lookupPath := pane.filePaneVolumeLookupPath()
	if lookupPath == "" {
		return "", now.Add(filePaneVolumeBadgeRetryInterval), false
	}

	state := &pane.volumeBadge
	if state.label == "" || state.lookupPath != lookupPath || state.nextRefreshAt.IsZero() || !now.Before(state.nextRefreshAt) {
		usage, err := platform.VolumeUsage{}, error(nil)
		if pane.remoteConnected() {
			usage, err = remoteVolumeUsageFunc(pane.remote, lookupPath)
		} else {
			usage, err = localVolumeUsageFunc(lookupPath)
		}
		state.lookupPath = lookupPath
		state.checkedAt = now
		if err != nil || usage.TotalBytes == 0 {
			state.label = ""
			state.nextRefreshAt = now.Add(filePaneVolumeBadgeRetryInterval)
			return "", state.nextRefreshAt, false
		}
		state.label = formatFilePaneVolumeBadgeLabel(usage.FreeBytes, usage.TotalBytes)
		state.nextRefreshAt = now.Add(filePaneVolumeBadgeRefreshInterval)
	}

	if state.label == "" {
		return "", state.nextRefreshAt, false
	}
	return state.label, state.nextRefreshAt, true
}

func (p *filePaneState) filePaneVolumeLookupPath() string {
	if p == nil || p.archiveBrowsing() {
		return ""
	}

	raw := strings.TrimSpace(p.loadingDir)
	if raw == "" {
		raw = strings.TrimSpace(p.dir)
	}
	if p.remoteConnected() {
		if raw == "" {
			raw = "/"
		}
		clean := path.Clean(raw)
		if clean == "." {
			return "/"
		}
		return clean
	}
	if raw == "" {
		raw = "."
	}
	return nearestExistingLocalPath(raw)
}

func remoteVolumeUsage(remote *paneSSHSession, lookupPath string) (platform.VolumeUsage, error) {
	usage, err := remoteVolumeUsageStatVFS(remote, lookupPath)
	if err == nil {
		return usage, nil
	}
	if !remoteVolumeUsageNeedsCommandFallback(err) {
		return platform.VolumeUsage{}, err
	}
	return remoteVolumeUsageDF(remote, lookupPath)
}

func remoteVolumeUsageStatVFS(remote *paneSSHSession, lookupPath string) (platform.VolumeUsage, error) {
	if remote == nil {
		return platform.VolumeUsage{}, errors.New("sftp session is not connected")
	}
	client := remote.sftpClient()
	if client == nil {
		if err := remote.reconnectSFTPClient(nil); err != nil {
			return platform.VolumeUsage{}, err
		}
		client = remote.sftpClient()
	}
	if client == nil {
		return platform.VolumeUsage{}, errors.New("sftp session is not connected")
	}

	vfs, err := client.StatVFS(lookupPath)
	if err == nil {
		return platform.VolumeUsage{FreeBytes: vfs.FreeSpace(), TotalBytes: vfs.TotalSpace()}, nil
	}
	if !shouldReconnectSSHTransport(err) {
		return platform.VolumeUsage{}, err
	}
	if reconnectErr := remote.reconnectSFTPClient(client); reconnectErr != nil {
		return platform.VolumeUsage{}, reconnectErr
	}
	client = remote.sftpClient()
	if client == nil {
		return platform.VolumeUsage{}, errors.New("sftp session is not connected")
	}
	vfs, err = client.StatVFS(lookupPath)
	if err != nil {
		return platform.VolumeUsage{}, err
	}
	return platform.VolumeUsage{FreeBytes: vfs.FreeSpace(), TotalBytes: vfs.TotalSpace()}, nil
}

func remoteVolumeUsageNeedsCommandFallback(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sftp.ErrSSHFxOpUnsupported) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unsupported extension") || strings.Contains(msg, "operation unsupported")
}

func remoteVolumeUsageDF(remote *paneSSHSession, lookupPath string) (platform.VolumeUsage, error) {
	usage, err := remoteVolumeUsageDFOnce(remote, lookupPath)
	if err == nil || !shouldReconnectSSHTransport(err) {
		return usage, err
	}
	if remote == nil {
		return platform.VolumeUsage{}, err
	}
	if reconnectErr := remote.reconnectSFTPClient(nil); reconnectErr != nil {
		return platform.VolumeUsage{}, reconnectErr
	}
	return remoteVolumeUsageDFOnce(remote, lookupPath)
}

func remoteVolumeUsageDFOnce(remote *paneSSHSession, lookupPath string) (platform.VolumeUsage, error) {
	if remote == nil {
		return platform.VolumeUsage{}, errors.New("remote ssh session is not connected")
	}
	cmdline := "LC_ALL=C df -Pk " + shellQuote(lookupPath) + " 2>/dev/null | awk 'NR==2 {print $2, $4}'"
	content, _, errText := readViewerRemoteCommand(
		context.Background(),
		remote,
		cmdline,
		resolveViewerShell("sh", true),
		256,
		time.Now(),
		filePaneVolumeBadgeRemoteTimeout,
		false,
		nil,
	)
	if strings.TrimSpace(errText) != "" {
		return platform.VolumeUsage{}, errors.New(errText)
	}
	return parseRemoteVolumeUsageDF(content)
}

func parseRemoteVolumeUsageDF(raw string) (platform.VolumeUsage, error) {
	fields := strings.Fields(raw)
	if len(fields) < 2 {
		return platform.VolumeUsage{}, errors.New("remote df returned no filesystem usage")
	}

	totalBlocks, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return platform.VolumeUsage{}, err
	}
	freeBlocks, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return platform.VolumeUsage{}, err
	}
	return platform.VolumeUsage{
		FreeBytes:  freeBlocks * 1024,
		TotalBytes: totalBlocks * 1024,
	}, nil
}

func nearestExistingLocalPath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		path = "."
	}
	path = filepath.Clean(path)
	for {
		info, err := os.Stat(path)
		if err == nil {
			if info.IsDir() {
				return path
			}
			return filepath.Dir(path)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

func formatFilePaneVolumeBadgeLabel(freeBytes, totalBytes uint64) string {
	if totalBytes == 0 {
		return ""
	}
	if freeBytes > totalBytes {
		freeBytes = totalBytes
	}
	return fmt.Sprintf("%s free / %s", formatFilePaneVolumeBytes(freeBytes), formatFilePaneVolumeBytes(totalBytes))
}

func formatFilePaneVolumeBytes(bytes uint64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	type unitDef struct {
		name string
		size uint64
	}

	units := []unitDef{
		{name: "PB", size: 1 << 50},
		{name: "TB", size: 1 << 40},
		{name: "GB", size: 1 << 30},
		{name: "MB", size: 1 << 20},
		{name: "KB", size: 1 << 10},
	}

	for _, unit := range units {
		if bytes < unit.size {
			continue
		}
		value := float64(bytes) / float64(unit.size)
		return fmt.Sprintf("%.2f %s", value, unit.name)
	}
	return fmt.Sprintf("%d B", bytes)
}

func filePaneVolumeBadgeOffset(idx, activeIdx int, paneSize, badgeSize image.Point) image.Point {
	x := 0
	if idx < activeIdx {
		x = paneSize.X - badgeSize.X
		if badgeSize.X < paneSize.X {
			x++
		}
	}
	if x < 0 {
		x = 0
	}
	y := paneSize.Y - badgeSize.Y
	if y < 0 {
		y = 0
	}
	return image.Pt(x, y)
}

func (ui *UI) filePaneVolumeBadgeSourcePane(idx int, pane *filePaneState, active bool) *filePaneState {
	if ui == nil || active || pane == nil {
		return nil
	}
	if ui.activeFilePane < 0 || ui.activeFilePane >= len(ui.filePanes) {
		return nil
	}
	return ui.filePanes[ui.activeFilePane]
}

func (ui *UI) layoutFilePaneVolumeBadge(th *material.Theme, gtx layout.Context, idx int, pane *filePaneState, active bool, palette filePanePalette) layout.Dimensions {
	if active || pane == nil || pane.ctxMenuOpen || pane.driveMenuOpen || pane.favoriteMenuOpen {
		return layout.Dimensions{}
	}

	sourcePane := ui.filePaneVolumeBadgeSourcePane(idx, pane, active)
	if sourcePane == nil {
		return layout.Dimensions{}
	}

	label, nextRefreshAt, ok := ui.filePaneVolumeBadgeLabel(sourcePane, gtx.Now)
	if nextRefreshAt.After(gtx.Now) {
		gtx.Execute(op.InvalidateCmd{At: nextRefreshAt})
	}
	if !ok {
		return layout.Dimensions{}
	}

	maxWidth := gtx.Constraints.Max.X
	if maxWidth < gtx.Dp(unit.Dp(132)) {
		return layout.Dimensions{}
	}

	width := ui.filePaneVolumeBadgeWidth(th, gtx, sourcePane, label)
	if width > maxWidth {
		width = maxWidth
	}

	bg, border, textColor := filePaneVolumeBadgeColors(palette)
	attachedLeft := idx > ui.activeFilePane
	m := op.Record(gtx.Ops)
	badgeGtx := gtx
	badgeGtx.Constraints.Min = image.Point{}
	dims := fixedWidth(badgeGtx, width, func(gtx layout.Context) layout.Dimensions {
		return layoutFilePaneAttachedBadge(gtx, bg, border, attachedLeft, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleThemeFontSize(th, 11)
				lbl.Color = textColor
				lbl.MaxLines = 1
				lbl.Truncator = "…"
				return lbl.Layout(gtx)
			})
		})
	})
	call := m.Stop()

	offset := filePaneVolumeBadgeOffset(
		idx,
		ui.activeFilePane,
		gtx.Constraints.Max,
		dims.Size,
	)
	stack := op.Offset(offset).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max, Baseline: dims.Baseline}
}

func (ui *UI) filePaneVolumeBadgeWidth(th *material.Theme, gtx layout.Context, pane *filePaneState, label string) int {
	if pane == nil {
		return ui.measureFilePaneVolumeBadgeWidth(th, gtx, label)
	}

	state := &pane.volumeBadge
	if state.measuredLabel != label || state.measuredWidth <= 0 || state.measuredPxDp != gtx.Metric.PxPerDp || state.measuredPxSp != gtx.Metric.PxPerSp {
		state.measuredLabel = label
		state.measuredWidth = ui.measureFilePaneVolumeBadgeWidth(th, gtx, label)
		state.measuredPxDp = gtx.Metric.PxPerDp
		state.measuredPxSp = gtx.Metric.PxPerSp
	}
	return state.measuredWidth
}

func (ui *UI) measureFilePaneVolumeBadgeWidth(th *material.Theme, gtx layout.Context, label string) int {
	lbl := material.Body2(th, label)
	lbl.Font.Typeface = ui.mainTypeface()
	lbl.TextSize = scaleThemeFontSize(th, 11)
	lbl.MaxLines = 1
	lbl.Truncator = ""

	width := measureLabelUnconstrained(gtx, lbl).Size.X
	width += gtx.Dp(unit.Dp(16))
	minWidth := gtx.Dp(unit.Dp(84))
	if width < minWidth {
		width = minWidth
	}
	return width
}

func filePaneVolumeBadgeColors(palette filePanePalette) (bg, border, text color.NRGBA) {
	bg = palette.PaneBg
	bg.A = 255

	text = palette.PaneFg
	if text.A == 0 {
		text = color.NRGBA{R: 242, G: 246, B: 250, A: 255}
	}
	border = mixNRGBA(text, bg, 0.38)
	if contrastScore(bg, border) < 1.22 {
		border = text
	}
	border.A = 168
	return bg, border, text
}

func layoutFilePaneAttachedBadge(gtx layout.Context, bg, border color.NRGBA, attachedLeft bool, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()

	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	rect := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
	paint.FillShape(gtx.Ops, bg, clip.Rect(rect).Op())
	if border.A != 0 {
		paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(0, 0, dims.Size.X, 1)).Op())
		if attachedLeft {
			paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(dims.Size.X-1, 0, dims.Size.X, dims.Size.Y)).Op())
		} else {
			paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(0, 0, 1, dims.Size.Y)).Op())
		}
	}

	call.Add(gtx.Ops)
	return dims
}
