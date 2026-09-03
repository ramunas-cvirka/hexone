// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"hexone/filesys"
	"hexone/fm"
	"hexone/ui/platform"
	"hexone/ui/widget/table"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestFormatFilePaneVolumeBadgeLabel(t *testing.T) {
	free := (uint64(6323) * (1 << 30)) / 100
	total := uint64(512 * (1 << 30))
	if got := formatFilePaneVolumeBadgeLabel(free, total); got != "63.23 GB free / 512.00 GB" {
		t.Fatalf("formatFilePaneVolumeBadgeLabel() = %q", got)
	}
}

// volumeTestContext builds the layout.Context a frame hands both the pump and
// the layout path.
func volumeTestContext(now time.Time) layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 80)),
	}
}

// volumeFrame runs one whole frame's worth of the volume pipeline in the real
// order: the pump ui.Layout runs at frame start, then the layout-path read. Every
// volume test drives frames through this rather than calling the reader directly,
// because the reader alone no longer does any work.
func volumeFrame(ui *UI, pane *filePaneState, now time.Time) (string, bool) {
	gtx := volumeTestContext(now)
	ui.pumpFilePaneVolumeLookups(gtx)
	return ui.filePaneVolumeBadgeLabel(gtx, pane)
}

// settleVolumeLookup runs frames until a *fresh* lookup has started and landed,
// so a test can assert on the reading rather than on the pipeline's timing. It
// waits on the sequence number rather than on checkedAt, so a reading left over
// from an earlier phase of the test cannot satisfy it. gtx.Now advances by a poll
// interval per iteration; the sleep is real, since the lookup runs on a goroutine.
func settleVolumeLookup(t *testing.T, ui *UI, pane *filePaneState, now time.Time) time.Time {
	t.Helper()
	startSeq := pane.volumeBadge.lookupSeq
	deadline := time.Now().Add(5 * time.Second)
	for frame := 0; ; frame++ {
		frameNow := now.Add(time.Duration(frame) * filePaneVolumeLookupPollInterval)
		volumeFrame(ui, pane, frameNow)
		state := &pane.volumeBadge
		if state.lookupSeq != startSeq && state.lookupStart.IsZero() && !state.checkedAt.IsZero() {
			return frameNow
		}
		if time.Now().After(deadline) {
			t.Fatalf("volume lookup did not land after %d frames", frame)
		}
		time.Sleep(time.Millisecond)
	}
}

func volumePane(t *testing.T, dir string) (*UI, *filePaneState) {
	t.Helper()
	ui := NewUI(nil)
	pane := newFilePaneState(dir, nil)
	ui.filePanes = []*filePaneState{pane}
	ui.activeFilePane = 0
	return ui, pane
}

// TestFilePaneVolumeBadgeLandsThroughThePump is the shape proof: the frame that
// first asks for free space gets no reading, the lookup runs off-frame, and the
// pump lands it on a later frame.
func TestFilePaneVolumeBadgeLandsThroughThePump(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()

	tempDir := t.TempDir()
	release := make(chan struct{})
	var gotPath string
	localVolumeUsageFunc = func(path string) (platform.VolumeUsage, error) {
		gotPath = path
		<-release
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}

	ui, pane := volumePane(t, tempDir)
	now := time.Unix(1700000000, 0)

	// Frame 1: nothing has been asked for yet, so the pump has nothing to start.
	if label, ok := volumeFrame(ui, pane, now); ok || label != "" {
		t.Fatalf("first frame label = %q, ok = %v, want empty and not ok", label, ok)
	}
	if !pane.volumeBadge.lookupStart.IsZero() {
		t.Fatal("first frame started a lookup before anything asked for one")
	}

	// Frame 2: the pump sees the request from frame 1 and starts the lookup. The
	// lookup is blocked on release, so this frame proves the layout path does not
	// wait for it.
	if label, ok := volumeFrame(ui, pane, now.Add(16*time.Millisecond)); ok || label != "" {
		t.Fatalf("second frame label = %q, ok = %v, want empty and not ok", label, ok)
	}
	if pane.volumeBadge.lookupStart.IsZero() {
		t.Fatal("second frame did not start the lookup")
	}
	if pane.volumeBadge.totalBytes != 0 {
		t.Fatalf("totalBytes = %d before the lookup landed, want 0", pane.volumeBadge.totalBytes)
	}

	close(release)
	landed := settleVolumeLookup(t, ui, pane, now.Add(32*time.Millisecond))

	label, ok := volumeFrame(ui, pane, landed)
	if !ok || label != "64.00 GB free / 512.00 GB" {
		t.Fatalf("landed label = %q, ok = %v", label, ok)
	}
	if want := filepath.Clean(tempDir); gotPath != want {
		t.Fatalf("lookup path = %q, want %q", gotPath, want)
	}
	if pane.volumeBadge.freeBytes != 64<<30 || pane.volumeBadge.totalBytes != 512<<30 {
		t.Fatalf("cached counts = %d/%d", pane.volumeBadge.freeBytes, pane.volumeBadge.totalBytes)
	}
}

// TestFilePaneVolumeBadgeLayoutNeverBlocks is the point of the whole pipeline: a
// lookup that never returns must not hold up a frame. The lookup below is parked
// on a channel the test controls, so this asserts the structure rather than a
// wall-clock threshold.
func TestFilePaneVolumeBadgeLayoutNeverBlocks(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()

	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	defer close(release)
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return platform.VolumeUsage{FreeBytes: 1 << 30, TotalBytes: 2 << 30}, nil
	}

	ui, pane := volumePane(t, t.TempDir())
	now := time.Unix(1700000000, 0)

	volumeFrame(ui, pane, now)
	volumeFrame(ui, pane, now.Add(16*time.Millisecond))
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the lookup goroutine never started")
	}

	// The lookup is now wedged. Sixty more frames must all complete.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for frame := range 60 {
			volumeFrame(ui, pane, now.Add(time.Duration(frame+2)*16*time.Millisecond))
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("frames blocked on the wedged volume lookup")
	}
}

// TestFilePaneVolumeBadgeStatsOncePerRefresh pins the hoist from step one, which
// survives independently of the async work: the lookup path is resolved behind
// the cache check, not in front of it. Before the hoist the usage call was cached
// but nearestExistingLocalPath was not, so an idle pane issued one os.Stat per
// frame — 60/s per visible pane during a scroll or a resize drag, blocking on a
// stale network mount.
func TestFilePaneVolumeBadgeStatsOncePerRefresh(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	oldStat := volumeLookupStatFunc
	defer func() {
		localVolumeUsageFunc = oldLookup
		volumeLookupStatFunc = oldStat
	}()

	var lookupCount, statCount atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		lookupCount.Add(1)
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}
	volumeLookupStatFunc = func(name string) (os.FileInfo, error) {
		statCount.Add(1)
		return os.Stat(name)
	}

	ui, pane := volumePane(t, t.TempDir())
	now := time.Unix(1700000000, 0)
	landed := settleVolumeLookup(t, ui, pane, now)

	for frame := range 120 {
		label, ok := volumeFrame(ui, pane, landed.Add(time.Duration(frame)*16*time.Millisecond))
		if !ok || label != "64.00 GB free / 512.00 GB" {
			t.Fatalf("frame %d: label = %q, ok = %v", frame, label, ok)
		}
	}
	if got := lookupCount.Load(); got != 1 {
		t.Fatalf("volume usage calls across 120 frames = %d, want 1", got)
	}
	if got := statCount.Load(); got != 1 {
		t.Fatalf("os.Stat calls across 120 frames = %d, want 1", got)
	}

	// The refresh deadline must resolve the path again: the pane may have been
	// unmounted under us since the last reading.
	settleVolumeLookup(t, ui, pane, landed.Add(filePaneVolumeBadgeRefreshInterval))
	if got, statGot := lookupCount.Load(), statCount.Load(); got != 2 || statGot != 2 {
		t.Fatalf("after refresh: lookups = %d, stats = %d, want 2 and 2", got, statGot)
	}
}

// TestFilePaneVolumeBadgeHonoursRetryIntervalAfterFailure pins the other half of
// the step-one hoist. A failed lookup leaves the label empty, and the old cache
// check keyed on the label, so every frame retried — which on a broken SFTP pane
// meant a blocking StatVFS, a df, and an SSH redial every single frame.
func TestFilePaneVolumeBadgeHonoursRetryIntervalAfterFailure(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	oldStat := volumeLookupStatFunc
	defer func() {
		localVolumeUsageFunc = oldLookup
		volumeLookupStatFunc = oldStat
	}()

	var lookupCount, statCount atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		lookupCount.Add(1)
		return platform.VolumeUsage{}, errors.New("volume is gone")
	}
	volumeLookupStatFunc = func(name string) (os.FileInfo, error) {
		statCount.Add(1)
		return os.Stat(name)
	}

	ui, pane := volumePane(t, t.TempDir())
	now := time.Unix(1700000000, 0)
	landed := settleVolumeLookup(t, ui, pane, now)

	for frame := range 60 {
		if _, ok := volumeFrame(ui, pane, landed.Add(time.Duration(frame)*16*time.Millisecond)); ok {
			t.Fatalf("frame %d: failed lookup reported ok", frame)
		}
	}
	if got := lookupCount.Load(); got != 1 {
		t.Fatalf("volume usage calls across 60 frames after a failure = %d, want 1", got)
	}
	if got := statCount.Load(); got != 1 {
		t.Fatalf("os.Stat calls across 60 frames after a failure = %d, want 1", got)
	}

	settleVolumeLookup(t, ui, pane, landed.Add(filePaneVolumeBadgeRetryInterval))
	if got := lookupCount.Load(); got != 2 {
		t.Fatalf("volume usage calls at the retry deadline = %d, want 2", got)
	}
}

// TestFilePaneVolumeBadgeDiscardsSupersededResult pins the sequence guard. A
// lookup started for one directory must not overwrite the reading for the
// directory the pane moved to while it was still in flight.
func TestFilePaneVolumeBadgeDiscardsSupersededResult(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()

	stale := make(chan struct{})
	entered := make(chan struct{}, 1)
	var calls atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		if calls.Add(1) == 1 {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-stale
			return platform.VolumeUsage{FreeBytes: 1 << 30, TotalBytes: 4 << 30}, nil
		}
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}

	ui, pane := volumePane(t, t.TempDir())
	now := time.Unix(1700000000, 0)

	volumeFrame(ui, pane, now)
	volumeFrame(ui, pane, now.Add(16*time.Millisecond))
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first lookup goroutine never started")
	}
	staleSeq := pane.volumeBadge.lookupSeq

	// The pane navigates away, which cancels the in-flight lookup.
	pane.dir = t.TempDir()
	pane.invalidateVolumeBadge()
	if pane.volumeBadge.lookupSeq == staleSeq {
		t.Fatal("navigating away did not supersede the in-flight lookup")
	}

	// Release the superseded lookup and let the replacement land.
	close(stale)
	landed := settleVolumeLookup(t, ui, pane, now.Add(32*time.Millisecond))

	// Drain a few more frames so the superseded result is definitely seen and
	// dropped rather than merely not yet delivered.
	for frame := range 20 {
		volumeFrame(ui, pane, landed.Add(time.Duration(frame)*16*time.Millisecond))
		time.Sleep(time.Millisecond)
	}
	if got := pane.volumeBadge.totalBytes; got != 512<<30 {
		t.Fatalf("totalBytes = %d, want the replacement lookup's 512 GB", got)
	}
	if got := pane.volumeBadge.label; got != "64.00 GB free / 512.00 GB" {
		t.Fatalf("label = %q, want the replacement lookup's reading", got)
	}
}

// TestFilePaneVolumeBadgeStopsPollingWhenNothingAsks pins the demand gate. The
// pump walks every pane in every tab, so without it a pane in a background tab —
// or every pane in a configuration that shows free space nowhere — would keep
// paying for a round trip every 15 seconds forever.
func TestFilePaneVolumeBadgeStopsPollingWhenNothingAsks(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()
	var calls atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		calls.Add(1)
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}

	ui, pane := volumePane(t, t.TempDir())
	now := time.Unix(1700000000, 0)
	landed := settleVolumeLookup(t, ui, pane, now)
	if got := calls.Load(); got != 1 {
		t.Fatalf("lookups after the first landing = %d, want 1", got)
	}

	// Frames keep coming, but nothing asks for this pane's free space any more.
	quiet := landed.Add(filePaneVolumeLookupIdleGrace + time.Second)
	for frame := range 10 {
		gtx := volumeTestContext(quiet.Add(time.Duration(frame) * filePaneVolumeBadgeRefreshInterval))
		ui.pumpFilePaneVolumeLookups(gtx)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("lookups after the pane went unwatched = %d, want 1", got)
	}

	// Asking again resumes the poll.
	settleVolumeLookup(t, ui, pane, quiet.Add(10*filePaneVolumeBadgeRefreshInterval))
	if got := calls.Load(); got != 2 {
		t.Fatalf("lookups after the pane was asked about again = %d, want 2", got)
	}
}

// TestFilePaneVolumeBadgeSkipsArchiveBrowsingPanes pins a gap the pump opened
// that the old synchronous path could not have: filePaneVolumeBadgeLabel refuses
// an archive pane, but the pump walks panes on its own, and an archive pane's
// lookup source is empty — which resolves to the working directory and would
// cache a reading for entirely the wrong volume.
func TestFilePaneVolumeBadgeSkipsArchiveBrowsingPanes(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()
	var calls atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		calls.Add(1)
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}

	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	if err := os.WriteFile(archive, []byte("not really a zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	ui, pane := volumePane(t, dir)
	now := time.Unix(1700000000, 0)
	settleVolumeLookup(t, ui, pane, now)
	if got := calls.Load(); got != 1 {
		t.Fatalf("lookups for the plain directory = %d, want 1", got)
	}

	pane.dir = archive
	if !pane.archiveBrowsing() {
		t.Fatalf("test setup: %q is not treated as an archive", archive)
	}
	pane.invalidateVolumeBadge()

	for frame := range 40 {
		label, ok := volumeFrame(ui, pane, now.Add(time.Duration(frame+1)*filePaneVolumeBadgeRefreshInterval))
		if ok || label != "" {
			t.Fatalf("frame %d: archive pane reported free space %q", frame, label)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("lookups while browsing an archive = %d, want no further lookups", got)
	}
}

// TestFilePaneVolumeBadgeAbandonsAnOverdueLookup pins the in-flight budget. A
// lookup can park in an uninterruptible stat, or lose its result to a full
// channel; without the budget the pane would wait on it forever and never refresh
// again.
//
// Abandoning it is not the same as replacing it, and the difference is the point
// of the cap: the goroutine is still parked in that syscall, so the pane waits for
// it to actually return before starting another against the same source.
func TestFilePaneVolumeBadgeAbandonsAnOverdueLookup(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()

	release := make(chan struct{})
	entered := make(chan struct{}, 4)
	var calls atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return platform.VolumeUsage{}, errors.New("never lands")
	}

	ui, pane := volumePane(t, t.TempDir())
	now := time.Unix(1700000000, 0)
	volumeFrame(ui, pane, now)
	volumeFrame(ui, pane, now.Add(16*time.Millisecond))
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the lookup goroutine never started")
	}

	// Still inside the budget: no second lookup piles on top of the first, and
	// the pane is still waiting on it.
	volumeFrame(ui, pane, now.Add(filePaneVolumeLookupMaxWait))
	if got := calls.Load(); got != 1 {
		t.Fatalf("lookups inside the in-flight budget = %d, want 1", got)
	}
	if pane.volumeBadge.lookupStart.IsZero() {
		t.Fatal("the pane stopped waiting on the lookup before its budget expired")
	}

	// Past the budget the pane stops waiting on it, but does not replace it.
	overdue := now.Add(filePaneVolumeLookupMaxWait + time.Second)
	for frame := range 8 {
		volumeFrame(ui, pane, overdue.Add(time.Duration(frame)*filePaneVolumeBadgeRetryInterval))
	}
	if !pane.volumeBadge.lookupStart.IsZero() {
		t.Fatal("the pane is still waiting on the overdue lookup")
	}
	if got := len(pane.volumeBadge.abandoned); got != 1 {
		t.Fatalf("abandoned lookups = %d, want the overdue one tracked", got)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("lookups after the budget expired = %d, want no replacement while the first is still running", got)
	}

	// Once it finally returns, the pane is free to try again.
	close(release)
	resumed := overdue.Add(8 * filePaneVolumeBadgeRetryInterval)
	settleVolumeLookup(t, ui, pane, resumed)
	if got := calls.Load(); got != 2 {
		t.Fatalf("lookups after the abandoned goroutine exited = %d, want 2", got)
	}
	if got := len(pane.volumeBadge.abandoned); got != 0 {
		t.Fatalf("abandoned lookups after the goroutine exited = %d, want 0", got)
	}
}

// TestFilePaneVolumeBadgeCapsGoroutinesOnAWedgedSource is why the cap exists.
// The in-flight budget on its own let a displayed pane start one fresh goroutine
// every filePaneVolumeLookupMaxWait for as long as it was on screen — eight over
// three minutes, roughly 144 an hour — each parked in the same uninterruptible
// syscall the last one was, and each remote one holding a retain() on the shared
// SSH connection so the transport could never be released.
func TestFilePaneVolumeBadgeCapsGoroutinesOnAWedgedSource(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()

	release := make(chan struct{})
	defer close(release)
	var started, finished atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		started.Add(1)
		<-release
		finished.Add(1)
		return platform.VolumeUsage{}, errors.New("never lands")
	}

	ui, pane := volumePane(t, t.TempDir())
	base := time.Unix(1700000000, 0)
	const step = 100 * time.Millisecond
	for frame := range int((3 * time.Minute) / step) {
		volumeFrame(ui, pane, base.Add(time.Duration(frame)*step))
		if frame%10 == 0 {
			// Give the goroutine a chance to actually reach the fake, so a lookup
			// the pane started is counted rather than merely queued.
			time.Sleep(200 * time.Microsecond)
		}
	}
	time.Sleep(20 * time.Millisecond)

	if got := finished.Load(); got != 0 {
		t.Fatalf("test setup: %d lookups returned, want the mount to stay wedged", got)
	}
	if got := started.Load(); got != 1 {
		t.Fatalf("lookup goroutines started across three simulated minutes on a wedged mount = %d, want 1", got)
	}
	if got := len(pane.volumeBadge.abandoned); got > filePaneVolumeMaxAbandonedLookups {
		t.Fatalf("outstanding abandoned lookups = %d, want at most %d", got, filePaneVolumeMaxAbandonedLookups)
	}
}

// TestFilePaneVolumeBadgeCapIsPerSource pins the other half of the cap. Blocking
// on "a lookup is outstanding" alone would wedge the badge for a pane that walked
// away from the dead mount, which is exactly what a user does when a share stops
// responding.
func TestFilePaneVolumeBadgeCapIsPerSource(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()

	release := make(chan struct{})
	defer close(release)
	wedged := t.TempDir()
	var wedgedCalls, healthyCalls atomic.Int64
	localVolumeUsageFunc = func(path string) (platform.VolumeUsage, error) {
		if path == filepath.Clean(wedged) {
			wedgedCalls.Add(1)
			<-release
			return platform.VolumeUsage{}, errors.New("never lands")
		}
		healthyCalls.Add(1)
		return platform.VolumeUsage{FreeBytes: 64 << 30, TotalBytes: 512 << 30}, nil
	}

	ui, pane := volumePane(t, wedged)
	now := time.Unix(1700000000, 0)
	// Wedge it, then blow the in-flight budget so the pane is sitting on the cap.
	volumeFrame(ui, pane, now)
	volumeFrame(ui, pane, now.Add(16*time.Millisecond))
	overdue := now.Add(filePaneVolumeLookupMaxWait + time.Second)
	volumeFrame(ui, pane, overdue)
	if got := len(pane.volumeBadge.abandoned); got != 1 {
		t.Fatalf("abandoned lookups on the wedged source = %d, want 1", got)
	}

	// The pane walks away from the dead mount. The lookup for its new directory
	// is a different key, so the cap does not apply to it.
	pane.dir = t.TempDir()
	pane.invalidateVolumeBadge()
	landed := settleVolumeLookup(t, ui, pane, overdue.Add(time.Second))
	if label, ok := volumeFrame(ui, pane, landed); !ok || label != "64.00 GB free / 512.00 GB" {
		t.Fatalf("label after moving off the wedged mount = %q, ok = %v", label, ok)
	}
	if got := healthyCalls.Load(); got != 1 {
		t.Fatalf("lookups for the healthy directory = %d, want 1", got)
	}
	if got := wedgedCalls.Load(); got != 1 {
		t.Fatalf("lookups for the wedged directory = %d, want no replacement", got)
	}
}

// TestFilePaneVolumeLookupIdleGraceOutlastsAnInFlightLookup pins the margin the
// demand grace has to carry.
//
// The pump schedules its next wakeup from nextRefreshAt, and a landing result
// pushes that a full refresh interval into the future. If it lands past the grace,
// the frame ends with no wakeup scheduled at all and an idle window quietly stops
// refreshing free space until something else happens to cause a frame. So the
// grace has to cover a whole refresh interval on top of the longest a lookup is
// allowed to stay in flight — which is what filePaneVolumeLookupMaxWait bounds.
func TestFilePaneVolumeLookupIdleGraceOutlastsAnInFlightLookup(t *testing.T) {
	if filePaneVolumeLookupIdleGrace <= filePaneVolumeBadgeRefreshInterval+filePaneVolumeLookupMaxWait {
		t.Fatalf("idle grace %v must exceed the refresh interval %v plus the in-flight budget %v",
			filePaneVolumeLookupIdleGrace,
			filePaneVolumeBadgeRefreshInterval,
			filePaneVolumeLookupMaxWait)
	}
}

// TestFilePaneVolumeBadgeKeepsReadingAcrossSameVolumeNavigation is the no-flicker
// half of the invalidation rule. Free space is a property of the volume, not the
// directory, so almost every navigation lands on the same number and blanking the
// field on each one would flash it off and back on throughout ordinary browsing.
func TestFilePaneVolumeBadgeKeepsReadingAcrossSameVolumeNavigation(t *testing.T) {
	oldLookup := localVolumeUsageFunc
	defer func() { localVolumeUsageFunc = oldLookup }()

	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int64
	localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
		if calls.Add(1) > 1 {
			// Every lookup after the first is wedged, so anything the badge
			// reports from here on can only have come from the cached reading.
			<-release
		}
		return platform.VolumeUsage{FreeBytes: 1 << 30, TotalBytes: 100 << 30}, nil
	}

	ui, pane := volumePane(t, t.TempDir())
	now := time.Unix(1700000000, 0)
	landed := settleVolumeLookup(t, ui, pane, now)
	const want = "1.00 GB free / 100.00 GB"
	if label, ok := volumeFrame(ui, pane, landed); !ok || label != want {
		t.Fatalf("landed label = %q, ok = %v", label, ok)
	}

	pane.dir = t.TempDir()
	pane.invalidateVolumeBadge()
	for frame := range 10 {
		label, ok := volumeFrame(ui, pane, landed.Add(time.Duration(frame+1)*16*time.Millisecond))
		if !ok || label != want {
			t.Fatalf("frame %d after same-volume navigation: label = %q, ok = %v, want the previous reading kept",
				frame, label, ok)
		}
	}
}

// TestFilePaneVolumeBadgeBlanksReadingAcrossRemotenessChange is the other half.
// Keeping the reading here shows one machine's free space beside the other
// machine's path, and not briefly: attachPaneSSHSession flips the pane to remote
// before any listing lands, so the stale number would sit there for the whole SSH
// dial. Unlike a same-volume move this costs nothing to detect — the pane's
// remoteness is already in memory.
func TestFilePaneVolumeBadgeBlanksReadingAcrossRemotenessChange(t *testing.T) {
	t.Run("local to remote", func(t *testing.T) {
		oldLocal := localVolumeUsageFunc
		oldRemote := remoteVolumeUsageFunc
		defer func() {
			localVolumeUsageFunc = oldLocal
			remoteVolumeUsageFunc = oldRemote
		}()

		release := make(chan struct{})
		defer close(release)
		localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
			return platform.VolumeUsage{FreeBytes: 1 << 30, TotalBytes: 100 << 30}, nil
		}
		remoteVolumeUsageFunc = func(context.Context, *paneSSHSession, string) (platform.VolumeUsage, error) {
			// Stands in for the SSH dial. How long it takes is the whole point:
			// whatever the bar shows meanwhile, it shows for seconds.
			<-release
			return platform.VolumeUsage{FreeBytes: 2 << 30, TotalBytes: 200 << 30}, nil
		}

		ui, pane := volumePane(t, t.TempDir())
		now := time.Unix(1700000000, 0)
		landed := settleVolumeLookup(t, ui, pane, now)
		if label, ok := volumeFrame(ui, pane, landed); !ok || label != "1.00 GB free / 100.00 GB" {
			t.Fatalf("local label = %q, ok = %v", label, ok)
		}

		remote := &paneSSHSession{conn: newSharedSSHConn(sshClientBundle{})}
		defer remote.close()
		pane.remote = remote
		pane.loadingDir = "/srv/projects"

		for frame := range 10 {
			label, ok := volumeFrame(ui, pane, landed.Add(time.Duration(frame+1)*16*time.Millisecond))
			if ok || label != "" {
				t.Fatalf("frame %d of the SSH connect: label = %q, want the local disk's reading blanked",
					frame, label)
			}
		}
	})

	t.Run("remote to local", func(t *testing.T) {
		oldLocal := localVolumeUsageFunc
		oldRemote := remoteVolumeUsageFunc
		defer func() {
			localVolumeUsageFunc = oldLocal
			remoteVolumeUsageFunc = oldRemote
		}()

		release := make(chan struct{})
		defer close(release)
		localVolumeUsageFunc = func(string) (platform.VolumeUsage, error) {
			<-release
			return platform.VolumeUsage{FreeBytes: 1 << 30, TotalBytes: 100 << 30}, nil
		}
		remoteVolumeUsageFunc = func(context.Context, *paneSSHSession, string) (platform.VolumeUsage, error) {
			return platform.VolumeUsage{FreeBytes: 2 << 30, TotalBytes: 200 << 30}, nil
		}

		remote := &paneSSHSession{conn: newSharedSSHConn(sshClientBundle{})}
		defer remote.close()
		ui, pane := volumePane(t, "/srv/projects")
		pane.remote = remote
		now := time.Unix(1700000000, 0)
		landed := settleVolumeLookup(t, ui, pane, now)
		if label, ok := volumeFrame(ui, pane, landed); !ok || label != "2.00 GB free / 200.00 GB" {
			t.Fatalf("remote label = %q, ok = %v", label, ok)
		}

		// Disconnect, exactly as disconnectPaneSSH does it.
		pane.remote = nil
		pane.dir = t.TempDir()
		for frame := range 10 {
			label, ok := volumeFrame(ui, pane, landed.Add(time.Duration(frame+1)*16*time.Millisecond))
			if ok || label != "" {
				t.Fatalf("frame %d after disconnecting: label = %q, want the remote host's reading blanked",
					frame, label)
			}
		}
	})
}

// TestCloseFilePaneTabCancelsVolumeLookup pins the cancel in closeFilePaneTab.
// The goroutine exits cleanly without it — the sequence guard drops whatever it
// produces — but cancelling cuts the df leg the moment the tab goes away, instead
// of letting it run out its remote timeout against a session nobody is using.
func TestCloseFilePaneTabCancelsVolumeLookup(t *testing.T) {
	oldRemote := remoteVolumeUsageFunc
	defer func() { remoteVolumeUsageFunc = oldRemote }()

	entered := make(chan context.Context, 1)
	release := make(chan struct{})
	defer close(release)
	remoteVolumeUsageFunc = func(ctx context.Context, _ *paneSSHSession, _ string) (platform.VolumeUsage, error) {
		entered <- ctx
		<-release
		return platform.VolumeUsage{}, errors.New("cancelled")
	}

	ui := NewUI(nil)
	ui.filePanes = []*filePaneState{newFilePaneState(t.TempDir(), nil)}
	ui.activeFilePane = 0
	ui.ensureFilePaneTabs()

	closing := newFilePaneState("/srv/projects", nil)
	closing.remote = &paneSSHSession{conn: newSharedSSHConn(sshClientBundle{})}
	ui.filePaneTabs[0].tabs = append(ui.filePaneTabs[0].tabs, closing)

	now := time.Unix(1700000000, 0)
	// The first frame records the demand, the second lets the pump act on it.
	for frame := range 2 {
		gtx := volumeTestContext(now.Add(time.Duration(frame) * 16 * time.Millisecond))
		ui.pumpFilePaneVolumeLookups(gtx)
		ui.filePaneVolumeBadgeLabel(gtx, closing)
	}
	var ctx context.Context
	select {
	case ctx = <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the remote volume lookup never started")
	}
	select {
	case <-ctx.Done():
		t.Fatal("the lookup context was cancelled before the tab closed")
	default:
	}

	if !ui.closeFilePaneTab(0, 1) {
		t.Fatal("closeFilePaneTab did not close the tab")
	}
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("closing the tab left the volume lookup's context running")
	}
}

func TestFilePaneVolumeBadgeUsesRemotePaneUsage(t *testing.T) {
	oldRemoteLookup := remoteVolumeUsageFunc
	defer func() { remoteVolumeUsageFunc = oldRemoteLookup }()

	remote := &paneSSHSession{conn: newSharedSSHConn(sshClientBundle{})}
	defer remote.close()

	var calls atomic.Int64
	paths := make(chan string, 8)
	sameConn := make(chan bool, 8)
	remoteVolumeUsageFunc = func(ctx context.Context, got *paneSSHSession, path string) (platform.VolumeUsage, error) {
		calls.Add(1)
		if ctx == nil {
			t.Error("remote volume lookup got a nil context")
		}
		// The lookup must run against a clone, not the pane's own session: the
		// clone holds its own reference on the shared connection so the pane can
		// be torn down underneath it.
		sameConn <- got != nil && got != remote && got.conn == remote.conn
		paths <- path
		return platform.VolumeUsage{FreeBytes: 128 << 30, TotalBytes: 512 << 30}, nil
	}

	ui, pane := volumePane(t, "/srv/projects")
	pane.remote = remote
	now := time.Unix(1700000000, 0)

	landed := settleVolumeLookup(t, ui, pane, now)
	label, ok := volumeFrame(ui, pane, landed)
	if !ok || label != "128.00 GB free / 512.00 GB" {
		t.Fatalf("remote label = %q, ok = %v", label, ok)
	}
	if got := <-paths; got != "/srv/projects" {
		t.Fatalf("remote lookup path = %q, want %q", got, "/srv/projects")
	}
	if !<-sameConn {
		t.Fatal("remote lookup did not run against a clone of the pane's session")
	}

	for frame := range 60 {
		if got, ok := volumeFrame(ui, pane, landed.Add(time.Duration(frame)*16*time.Millisecond)); !ok || got != label {
			t.Fatalf("frame %d: cached remote label = %q, ok = %v", frame, got, ok)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("remote lookups across 60 cached frames = %d, want 1", got)
	}

	settleVolumeLookup(t, ui, pane, landed.Add(filePaneVolumeBadgeRefreshInterval))
	if got := calls.Load(); got != 2 {
		t.Fatalf("remote lookups after the refresh deadline = %d, want 2", got)
	}
}

// TestFilePaneVolumeBadgeSurvivesPaneTeardown pins the teardown contract: a pane
// closed while its lookup is still in flight must not take the SSH transport out
// from under the goroutine, and the goroutine must not resurrect anything.
func TestFilePaneVolumeBadgeSurvivesPaneTeardown(t *testing.T) {
	oldRemoteLookup := remoteVolumeUsageFunc
	defer func() { remoteVolumeUsageFunc = oldRemoteLookup }()

	release := make(chan struct{})
	entered := make(chan *paneSSHSession, 1)
	remoteVolumeUsageFunc = func(_ context.Context, got *paneSSHSession, _ string) (platform.VolumeUsage, error) {
		entered <- got
		<-release
		// Touching the session after the pane is gone must still be safe: the
		// clone holds a reference on the shared connection.
		_ = got.sftpClient()
		return platform.VolumeUsage{FreeBytes: 1 << 30, TotalBytes: 2 << 30}, nil
	}

	remote := &paneSSHSession{conn: newSharedSSHConn(sshClientBundle{})}
	ui, pane := volumePane(t, "/srv/projects")
	pane.remote = remote
	now := time.Unix(1700000000, 0)

	volumeFrame(ui, pane, now)
	volumeFrame(ui, pane, now.Add(16*time.Millisecond))
	var session *paneSSHSession
	select {
	case session = <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the remote lookup goroutine never started")
	}
	if session == remote {
		t.Fatal("the lookup captured the pane's own session instead of a clone")
	}

	// The pane is closed while the lookup is still parked, exactly as
	// closeFilePaneTab does it.
	pane.remote = nil
	remote.close()
	ui.filePanes = nil

	close(release)
	// The goroutine finishes into a channel nobody reads. Nothing to assert
	// beyond not panicking or racing; -race and the goroutine's own session
	// access above are the real checks.
	time.Sleep(50 * time.Millisecond)
}

func TestApplyListingWithRestoreInvalidatesVolumeBadge(t *testing.T) {
	dir := t.TempDir()
	pane := newFilePaneState(dir, nil)
	pane.volumeBadge.label = "cached"
	pane.volumeBadge.nextRefreshAt = time.Now().Add(time.Minute)

	pane.applyListingWithRestore(filesys.Listing{Dir: dir}, "", "", 0, layout.Position{}, false, "")

	if pane.volumeBadge.nextRefreshAt != (time.Time{}) {
		t.Fatalf("nextRefreshAt = %v, want zero", pane.volumeBadge.nextRefreshAt)
	}
}

func TestFilePaneVolumeBadgeOffsetPinsToBottomInnerCorner(t *testing.T) {
	paneSize := image.Pt(320, 200)
	badgeSize := image.Pt(120, 20)

	leftInactive := filePaneVolumeBadgeOffset(0, 1, paneSize, badgeSize)
	if want := image.Pt(201, 180); leftInactive != want {
		t.Fatalf("left inactive offset = %v, want %v", leftInactive, want)
	}

	rightInactive := filePaneVolumeBadgeOffset(1, 0, paneSize, badgeSize)
	if want := image.Pt(0, 180); rightInactive != want {
		t.Fatalf("right inactive offset = %v, want %v", rightInactive, want)
	}
}

func TestFilePaneVolumeBadgeWidthCacheTracksTextStyle(t *testing.T) {
	ui := &UI{}
	pane := newFilePaneState(t.TempDir(), nil)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(640, 80)),
	}
	label := "999.99 GB free / 9999.99 GB"

	small := material.NewTheme()
	small.TextSize = 12
	large := material.NewTheme()
	large.TextSize = 24

	smallWidth := ui.filePaneVolumeBadgeWidth(small, gtx, pane, label)
	if got, want := pane.volumeBadge.measuredTextSize, filePaneVolumeBadgeTextSize(small); got != want {
		t.Fatalf("small measuredTextSize = %v, want %v", got, want)
	}

	largeWidth := ui.filePaneVolumeBadgeWidth(large, gtx, pane, label)
	if largeWidth <= smallWidth {
		t.Fatalf("large font cached width = %d, want greater than small width %d", largeWidth, smallWidth)
	}
	if got, want := pane.volumeBadge.measuredTextSize, filePaneVolumeBadgeTextSize(large); got != want {
		t.Fatalf("large measuredTextSize = %v, want %v", got, want)
	}

	ui.typeface = font.Typeface("serif")
	_ = ui.filePaneVolumeBadgeWidth(large, gtx, pane, label)
	if got, want := pane.volumeBadge.measuredTypeface, ui.mainTypeface(); got != want {
		t.Fatalf("measuredTypeface = %q, want %q", got, want)
	}
}

func TestFilePaneVolumeBadgeSourcePaneUsesActivePane(t *testing.T) {
	ui := NewUI(nil)
	left := newFilePaneState(t.TempDir(), nil)
	right := newFilePaneState(t.TempDir(), nil)
	ui.filePanes = []*filePaneState{left, right}

	ui.activeFilePane = 0
	if got := ui.filePaneVolumeBadgeSourcePane(1, right, false); got != left {
		t.Fatalf("source pane = %p, want active left pane %p", got, left)
	}

	ui.activeFilePane = 1
	if got := ui.filePaneVolumeBadgeSourcePane(0, left, false); got != right {
		t.Fatalf("source pane = %p, want active right pane %p", got, right)
	}

	if got := ui.filePaneVolumeBadgeSourcePane(1, right, true); got != nil {
		t.Fatalf("active pane badge source = %p, want nil", got)
	}
}

func TestFilePaneVolumeBadgeSourcePaneKeepsMirroringExtractingPane(t *testing.T) {
	ui := NewUI(nil)
	left := newFilePaneState(t.TempDir(), nil)
	right := newFilePaneState(t.TempDir(), nil)
	ui.filePanes = []*filePaneState{left, right}
	ui.activeFilePane = 0

	now := time.Unix(1700000000, 0)
	ui.archiveExtract = &archiveExtractState{
		pane:        0,
		archivePath: "bundle.zip",
		startedAt:   now.Add(-time.Second),
		progress: filesys.CopyProgress{
			BytesDone:   50 << 20,
			BytesTotal:  100 << 20,
			CurrentPath: filepath.Join("bundle.zip", "movie.mkv"),
		},
	}

	if got := ui.filePaneVolumeBadgeSourcePane(1, right, false); got != left {
		t.Fatalf("badge source mirrored from extracting active pane = %p, want left pane %p", got, left)
	}

	if got := ui.filePaneVolumeBadgeSourcePane(0, left, true); got != nil {
		t.Fatalf("active extracting pane badge source = %p, want nil", got)
	}
}

func TestFilePaneVolumeBadgesVisibleWhenTerminalOpenButPaneFocused(t *testing.T) {
	terminal := newTerminalSession(nil)
	terminal.setActive(true)
	ui := &UI{terminal: terminal}

	if ui.filePaneVolumeBadgesHidden(layout.Context{}) {
		t.Fatal("volume badges should stay visible when the terminal drawer is open but not focused")
	}
}

func TestFilePaneVolumeBadgesStayHiddenAcrossTerminalTabSwitch(t *testing.T) {
	first := newTerminalSession(nil)
	second := newTerminalSession(nil)
	first.setActive(true)
	first.focusKeyboard()
	ui := &UI{
		terminal: first,
		terminalTabs: terminalTabSet{
			sessions: []*terminalSession{first, second},
			active:   0,
		},
	}

	if !ui.filePaneVolumeBadgesHidden(layout.Context{}) {
		t.Fatal("volume badges should be hidden while the terminal is focused")
	}
	if !ui.activateTerminalTab(1) {
		t.Fatal("activateTerminalTab should switch terminal tabs")
	}
	if !ui.filePaneVolumeBadgesHidden(layout.Context{}) {
		t.Fatal("volume badges became visible during terminal tab handoff")
	}
	if first.active() || !second.active() {
		t.Fatal("terminal active state was not transferred to the selected tab")
	}
}

// TestVolumeBadgeSuppressedWhenFreeFieldEnabled pins the badge/bar handoff: the
// floating badge and the status bar are two presentations of one number — the
// ACTIVE pane's volume — so exactly one of them shows at a time, and every way
// of not showing the bar falls back to the badge.
//
// The original spec §3 decision was that suppression is global: read
// StatusBar.Enabled and the field list, never hide_in_full, on the theory that a
// pane whose bar is hidden should simply show no free space. Live user review
// retired that (see the dated note in the design doc): the two no-bar paths
// behaved differently — switching the bar off gave the badge back, while hiding
// it in full mode dropped free space out of the window altogether. The
// "hide in full, full pane" row below is the one that changed.
//
// Every row drives filePaneVolumeBadgesHidden, the predicate
// layoutFilePaneVolumeBadges actually consults, so a predicate nobody wired up
// would still fail here.
func TestVolumeBadgeSuppressedWhenFreeFieldEnabled(t *testing.T) {
	freeFields := []string{fm.StatusBarFieldSize, fm.StatusBarFieldDate, fm.StatusBarFieldFree}
	noFreeFields := []string{fm.StatusBarFieldSize, fm.StatusBarFieldDate}
	tests := []struct {
		name       string
		enabled    bool
		hideInFull bool
		mode       table.Mode
		fields     []string
		// wantHidden is "the badge is suppressed", i.e. the active pane's bar is
		// carrying free space itself.
		wantHidden bool
	}{
		{"free field on, brief pane", true, false, table.ModeBrief, freeFields, true},
		{"free field on, full pane", true, false, table.ModeFull, freeFields, true},
		{"hide in full, brief pane keeps its bar", true, true, table.ModeBrief, freeFields, true},
		{"hide in full, full pane falls back to the badge", true, true, table.ModeFull, freeFields, false},
		{"bar disabled, brief pane", false, false, table.ModeBrief, freeFields, false},
		{"bar disabled, full pane", false, true, table.ModeFull, freeFields, false},
		{"free field off, brief pane", true, false, table.ModeBrief, noFreeFields, false},
		{"free field off, full pane hidden", true, true, table.ModeFull, noFreeFields, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fm.DefaultConfig()
			cfg.StatusBar.Enabled = tc.enabled
			cfg.StatusBar.HideInFull = tc.hideInFull
			cfg.StatusBar.Fields = tc.fields
			ui := NewUI(cfg)
			for i, pane := range ui.filePanes {
				if pane == nil || pane.table == nil {
					t.Fatalf("pane %d has no table to set a view mode on", i)
				}
				pane.table.SetMode(tc.mode)
			}
			if got := ui.filePaneVolumeBadgesHidden(layout.Context{}); got != tc.wantHidden {
				t.Fatalf("filePaneVolumeBadgesHidden() = %v, want %v", got, tc.wantHidden)
			}
			// Checked alongside the wiring so a failure says which layer moved.
			if got := ui.filePaneStatusBarShowsFreeSpace(); got != tc.wantHidden {
				t.Fatalf("filePaneStatusBarShowsFreeSpace() = %v, want %v", got, tc.wantHidden)
			}
		})
	}
}

// TestVolumeBadgeFollowsActivePaneViewMode pins which pane decides. The badge is
// drawn on the inactive panes but sourced from the active one
// (filePaneVolumeBadgeSourcePane), so it is the ACTIVE pane's bar that makes it
// redundant — an inactive neighbour still carrying a bar does not, and the two
// panes can be in different view modes.
func TestVolumeBadgeFollowsActivePaneViewMode(t *testing.T) {
	cfg := fm.DefaultConfig()
	cfg.StatusBar.Enabled = true
	cfg.StatusBar.HideInFull = true
	cfg.StatusBar.Fields = []string{fm.StatusBarFieldSize, fm.StatusBarFieldFree}
	ui := NewUI(cfg)
	if len(ui.filePanes) < 2 {
		t.Fatalf("pane count = %d, want at least 2", len(ui.filePanes))
	}
	ui.filePanes[0].table.SetMode(table.ModeFull)
	ui.filePanes[1].table.SetMode(table.ModeBrief)

	ui.activeFilePane = 0
	if ui.filePaneVolumeBadgesHidden(layout.Context{}) {
		t.Fatal("the active pane hides its bar in full mode, so the badge must come back even though the other pane still shows one")
	}

	ui.activeFilePane = 1
	if !ui.filePaneVolumeBadgesHidden(layout.Context{}) {
		t.Fatal("the active pane is in brief mode and its bar carries free space, so the badge must stay suppressed")
	}
}

// TestVolumeBadgeGateNilSafe covers the degenerate shapes the gate can be asked
// about: no UI, no config, no panes, a nil pane, a pane with no table. None of
// them is a bar carrying free space, so none of them may suppress the badge.
func TestVolumeBadgeGateNilSafe(t *testing.T) {
	var nilUI *UI
	if nilUI.filePaneStatusBarShowsFreeSpace() {
		t.Fatal("a nil UI shows no free space anywhere")
	}

	cfg := fm.DefaultConfig()
	cfg.StatusBar.Enabled = true
	cfg.StatusBar.Fields = []string{fm.StatusBarFieldFree}

	ui := &UI{}
	if ui.filePaneStatusBarShowsFreeSpace() {
		t.Fatal("a UI with no config shows no free space in a status bar")
	}

	ui.fmCfg = cfg
	if ui.filePaneStatusBarShowsFreeSpace() {
		t.Fatal("no panes means no status bar to carry free space")
	}
	if ui.filePaneVolumeBadgesHidden(layout.Context{}) {
		t.Fatal("badges must not be suppressed by a status bar with no pane to render in")
	}

	// An out-of-range active index over a nil pane: activePane clamps, and the
	// visibility rule rejects the nil pane it hands back.
	ui.filePanes = []*filePaneState{nil}
	ui.activeFilePane = 7
	if ui.filePaneStatusBarShowsFreeSpace() {
		t.Fatal("a nil active pane carries no status bar")
	}

	// A pane that exists but has no table has no view mode to test hide_in_full
	// against, so it cannot be showing a bar either.
	ui.filePanes = []*filePaneState{{}}
	ui.activeFilePane = 0
	if ui.filePaneStatusBarShowsFreeSpace() {
		t.Fatal("a pane with no table carries no status bar")
	}
}
