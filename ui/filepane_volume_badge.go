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
	"slices"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
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

	// filePaneVolumeLookupPollInterval is how often the frame loop wakes while a
	// lookup is in flight: a goroutine finishing cannot wake Gio by itself, so
	// the pump has to come back and look. Free space is not latency-critical, so
	// this is deliberately far coarser than the 50ms pumpFilePaneLoads uses for
	// directory listings.
	filePaneVolumeLookupPollInterval = 250 * time.Millisecond

	// filePaneVolumeLookupIdleGrace is how long a pane keeps polling after the
	// last frame that asked for its free space. Past it a pane nobody asks about
	// — one in a background tab, or every pane in a configuration that displays
	// free space nowhere — stops costing a round trip.
	//
	// The multiplier is what keeps the pane from switching itself off mid-cycle.
	// An idle window's only frames are the poll wakeups, and the pump schedules
	// the next one from nextRefreshAt, which a landing result pushes out by a
	// full refresh interval. So a frame can end with no wakeup scheduled whenever
	// a result lands more than (grace - refreshInterval) after the last frame
	// that asked. At 2x that margin is one refresh interval, which a result
	// landing late in a slow lookup can exceed; at 3x it is 30s, longer than the
	// 25s filePaneVolumeLookupMaxWait budget that bounds how late a result can
	// land at all, so the gap is closed rather than merely narrowed.
	filePaneVolumeLookupIdleGrace = 3 * filePaneVolumeBadgeRefreshInterval

	// filePaneVolumeLookupMaxWait bounds an in-flight lookup. A lookup can park
	// in an uninterruptible os.Stat on a dead mount, or in reconnectSFTPClient's
	// synchronous SSH dial; past this budget the pane stops waiting on it, which
	// is not the same as replacing it — the goroutine is still in that syscall, so
	// filePaneVolumeMaxAbandonedLookups below decides whether another may start.
	// Comfortably longer than sshConnectBudget (12s) so a legitimate reconnect is
	// never restarted on top of itself.
	filePaneVolumeLookupMaxWait = 25 * time.Second

	// filePaneVolumeMaxAbandonedLookups is the hard ceiling on goroutines a
	// single pane's volume pipeline can have outstanding.
	//
	// Abandoning a lookup does not stop it: cancelLookup cuts the df leg, but
	// os.Stat on a hung NFS/SMB mount and sftp's StatVFS take no context and run
	// to completion, and a remote lookup holds a retain() on the shared SSH
	// connection for its whole life. The per-source rule below already stops a
	// pane wedged on one volume from spawning a fresh goroutine every
	// filePaneVolumeLookupMaxWait; this bounds the one case it does not, a pane
	// walked across several distinct wedged mounts.
	filePaneVolumeMaxAbandonedLookups = 4
)

// errFilePaneVolumeLookupPath marks the one failure the lookup detects before it
// reaches a volume: the pane's directory does not resolve to anything that can be
// measured. Unreachable on Unix, reachable on Windows with a disconnected drive
// letter.
var errFilePaneVolumeLookupPath = errors.New("volume lookup path is unavailable")

// filePaneVolumeResult carries one completed lookup back to the frame loop.
//
// seq is the sequence guard described in ARCHITECTURE.md: pumpVolumeLookup drops
// any result whose seq no longer matches the pane's current lookup, which is how
// a lookup superseded by a directory change — or one still parked in a 12s SSH
// dial after the pane moved on — is ignored rather than overwriting a newer
// reading.
type filePaneVolumeResult struct {
	seq   int
	usage platform.VolumeUsage
	err   error
}

// filePaneVolumeAbandonedLookup is one lookup goroutine the pane has given up on
// but which is still running, and still holding whatever it holds: a retain() on
// the shared SSH connection for a remote lookup, a thread parked in an
// uninterruptible stat for a local one.
//
// source and remote are the key it was started for, so a pane that navigates to
// a *different* volume is not blocked behind a lookup wedged on the old one.
type filePaneVolumeAbandonedLookup struct {
	source string
	remote bool
	done   <-chan struct{}
}

const (
	filePaneVolumeBadgePaddingX   unit.Dp = 8
	filePaneVolumeBadgePaddingY   unit.Dp = 4
	filePaneVolumeBadgeWidthSlack unit.Dp = 4
)

type filePaneVolumeBadgeState struct {
	// lookupSource and lookupRemote are the cache key, and they are deliberately
	// the pane's *raw* directory rather than the resolved lookup path: resolving
	// a local path runs nearestExistingLocalPath, which stats. Keying on the raw
	// string lets a cache hit skip the resolution entirely, which is the whole
	// point — filePaneVolumeBadgeLabel is called once per visible pane per frame.
	//
	// The cost is that a directory which becomes unresolvable without its raw
	// string changing (an unmounted share) is noticed only at the next refresh,
	// so a reading can be up to filePaneVolumeBadgeRefreshInterval stale. That is
	// what the refresh interval is for.
	lookupSource string
	lookupRemote bool
	label        string
	// freeBytes and totalBytes are the raw counts behind label. The status bar
	// reads these rather than the label so its shorter free-space forms are
	// formatted from the same numbers instead of re-parsed from a string.
	freeBytes     uint64
	totalBytes    uint64
	checkedAt     time.Time
	nextRefreshAt time.Time

	// The async pipeline, per ARCHITECTURE.md's start/sequence/pump shape.
	// lookupSeq is bumped by every start and by every cancellation, so a result
	// carrying an older seq is dropped. lookupStart is zero exactly when no
	// lookup is in flight. wantedAt is the last frame time at which a layout path
	// asked for this pane's free space, and is what keeps the pump polling.
	lookupSeq    int
	lookupCancel context.CancelFunc
	lookupStart  time.Time
	resultCh     chan filePaneVolumeResult
	wantedAt     time.Time

	// lookupDone is closed by the in-flight lookup goroutine as its last act, so
	// "has the goroutine actually exited?" is answerable without reading anything
	// the goroutine writes. It deliberately does not ride on resultCh: that send
	// is dropped when the buffer is full, and a liveness signal that can go
	// missing would let the cap below leak.
	lookupDone <-chan struct{}

	// abandoned holds the lookups this pane has stopped waiting on but whose
	// goroutines are still running, and is what keeps a wedged mount from
	// spawning one goroutine per filePaneVolumeLookupMaxWait forever. Entries
	// leave it only when their goroutine exits.
	abandoned []filePaneVolumeAbandonedLookup

	measuredLabel    string
	measuredWidth    int
	measuredPxDp     float32
	measuredPxSp     float32
	measuredTypeface font.Typeface
	measuredTextSize unit.Sp
}

var localVolumeUsageFunc = platform.LocalVolumeUsage
var remoteVolumeUsageFunc = remoteVolumeUsage

type statFunc func(string) (os.FileInfo, error)

// volumeLookupStatFunc is the seam nearestExistingLocalPath stats through, so
// tests can count the syscalls the volume lookup actually issues. It is read on
// the frame loop by startVolumeLookup and handed to the goroutine, never read
// from the goroutine itself.
var volumeLookupStatFunc statFunc = os.Stat

func (p *filePaneState) invalidateVolumeBadge() {
	if p == nil {
		return
	}
	// The in-flight lookup, if any, was started for the directory the pane has
	// just left, so it is cancelled rather than allowed to land.
	p.volumeBadge.cancelLookup()
	source, remote := p.filePaneVolumeLookupSource()
	p.volumeBadge.discardReadingOnVolumeChange(source, remote)
	p.volumeBadge.checkedAt = time.Time{}
	p.volumeBadge.nextRefreshAt = time.Time{}
}

// discardReadingOnVolumeChange blanks the cached reading when the pane has moved
// somewhere the reading demonstrably does not describe.
//
// Keeping it is the default, and for same-volume navigation that is right: free
// space is a property of the volume, not the directory, so almost every
// navigation lands on the same number, and blanking on each one would flicker the
// field off and back on throughout ordinary browsing. A pane that has never had a
// reading still reports totalBytes == 0 and renders no free-space field at all.
//
// The exceptions are the ones a pure in-memory comparison can *prove*, because
// this runs on the frame and is not allowed to do I/O:
//
//   - local <-> remote. This is the case that actually misleads. Nothing here is
//     instant: attachPaneSSHSession flips the pane to remote before the listing
//     lands, and an SSH dial takes seconds, so without this the bar shows the
//     LOCAL disk's free space beside a remote path for the whole connect.
//   - a different filepath.VolumeName. On Windows that is the drive letter or the
//     UNC share, so C:\ -> D:\ is caught exactly, off one string comparison. On
//     Unix VolumeName is always "" and the check is a no-op.
//
// A local Unix move across a mount point (/ -> /Volumes/usb) is deliberately NOT
// caught. Deciding it needs a stat of both paths — precisely the blocking call
// this pipeline exists to keep off the frame — and the only I/O-free stand-in,
// comparing leading path elements, would also fire on / -> /usr and reintroduce
// exactly the flicker the keep-by-default rule exists to prevent. The residual
// window is bounded by how long a local lookup takes rather than by an SSH dial.
func (s *filePaneVolumeBadgeState) discardReadingOnVolumeChange(source string, remote bool) {
	if s == nil || !s.volumeChanged(source, remote) {
		return
	}
	s.freeBytes = 0
	s.totalBytes = 0
	s.label = ""
}

// volumeChanged reports whether source is provably on a different volume from
// the one the cached reading describes. See discardReadingOnVolumeChange for why
// "provably" is doing so much work here.
func (s *filePaneVolumeBadgeState) volumeChanged(source string, remote bool) bool {
	if s == nil {
		return false
	}
	return remote != s.lookupRemote || !sameFilePaneVolumeName(s.lookupSource, source, remote)
}

// sameFilePaneVolumeName reports whether two raw lookup sources are on the same
// named volume, erring towards "yes" whenever it cannot tell — an unproven
// difference must not blank a reading, or ordinary browsing flickers.
func sameFilePaneVolumeName(oldSource, newSource string, remote bool) bool {
	if remote {
		// A remote lookup source is a bare POSIX path; the host it belongs to is
		// not in the string, so there is nothing here to compare.
		return true
	}
	oldVolume := filepath.VolumeName(strings.TrimSpace(oldSource))
	newVolume := filepath.VolumeName(strings.TrimSpace(newSource))
	if oldVolume == "" || newVolume == "" {
		return true
	}
	// Windows drive letters are case-insensitive, and the pane's raw directory is
	// whatever string the user or the listing produced.
	return strings.EqualFold(oldVolume, newVolume)
}

// cancelLookup drops the in-flight lookup, if any.
//
// Bumping the sequence is what actually discards it. The goroutine may be parked
// in an uninterruptible os.Stat or an SSH dial and will still run to completion
// and send; the pump then drops the result because the seq no longer matches.
//
// Because the goroutine outlives the cancellation, it is handed to the abandoned
// list rather than forgotten: a lookup nobody is waiting for still holds a
// retain() on the shared SSH connection, and startVolumeLookup refuses to pile a
// second goroutine onto the same wedged source.
func (s *filePaneVolumeBadgeState) cancelLookup() {
	if s == nil {
		return
	}
	if s.lookupCancel != nil {
		s.lookupCancel()
		s.lookupCancel = nil
	}
	if !s.lookupStart.IsZero() {
		s.lookupSeq++
		s.lookupStart = time.Time{}
		if s.lookupDone != nil {
			s.abandoned = append(s.abandoned, filePaneVolumeAbandonedLookup{
				source: s.lookupSource,
				remote: s.lookupRemote,
				done:   s.lookupDone,
			})
		}
	}
	s.lookupDone = nil
}

// sweepAbandonedLookups forgets the abandoned lookups whose goroutines have since
// exited. Called once per pumped frame, so a mount that unwedges lets the pane
// resume without any further prompting.
func (s *filePaneVolumeBadgeState) sweepAbandonedLookups() {
	if s == nil || len(s.abandoned) == 0 {
		return
	}
	live := s.abandoned[:0]
	for _, lookup := range s.abandoned {
		select {
		case <-lookup.done:
			// The goroutine returned: its session clone is released and its
			// retain() on the shared SSH connection is gone.
		default:
			live = append(live, lookup)
		}
	}
	for i := len(live); i < len(s.abandoned); i++ {
		s.abandoned[i] = filePaneVolumeAbandonedLookup{}
	}
	s.abandoned = live
}

// lookupCapReached reports whether a new lookup for source has to wait.
//
// Two rules, and the first is the one that matters: an abandoned lookup for the
// *same* source is still running, so starting another would only add a second
// goroutine parked in the same dead syscall. Navigating to a different volume is
// deliberately not blocked by it. The second rule is the backstop for a pane
// walked across several distinct wedged mounts.
func (s *filePaneVolumeBadgeState) lookupCapReached(source string, remote bool) bool {
	if s == nil {
		return false
	}
	if len(s.abandoned) >= filePaneVolumeMaxAbandonedLookups {
		return true
	}
	for _, lookup := range s.abandoned {
		if lookup.remote == remote && lookup.source == source {
			return true
		}
	}
	return false
}

// filePaneVolumeBadgeLabel returns the last volume reading that landed for a
// pane, as a formatted badge label.
//
// It performs no I/O and never blocks. Both consumers — the floating badge and
// the status bar's free-space field — call it from the layout path, so it only
// reads state that pumpFilePaneVolumeLookups has already applied, and records
// that this frame wanted the number so the pump keeps polling for this pane.
func (ui *UI) filePaneVolumeBadgeLabel(gtx layout.Context, pane *filePaneState) (string, bool) {
	if pane == nil || pane.archiveBrowsing() {
		return "", false
	}

	state := &pane.volumeBadge
	state.wantedAt = gtx.Now
	if state.lookupStart.IsZero() {
		// Unchecked: the archiveBrowsing guard above already ran this frame.
		source, remote := pane.filePaneVolumeLookupSourceUnchecked()
		// The pump owns starting lookups and runs ahead of layout, so the first
		// frame that asks for a reading has to ask for one more frame for the pump
		// to act on. Both gates are what keep that from becoming a self-driving
		// repaint loop: with nothing in flight and no cap in the way, the very next
		// frame starts the lookup and stops asking. The cap gate is not optional —
		// while a lookup is capped nothing is in flight and the refresh stays due,
		// so without it a pane parked on a wedged mount would repaint at the full
		// frame rate for as long as the mount stayed wedged. The pump keeps that
		// pane alive on its own, coarser, wakeup instead.
		if filePaneVolumeLookupDue(state, source, remote, gtx.Now) && !state.lookupCapReached(source, remote) {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if state.label == "" {
		return "", false
	}
	return state.label, true
}

// pumpFilePaneVolumeLookups drains every pane's volume channel, starts the
// lookups that have come due, and schedules the next wakeup. Registered in the
// pump block at the top of ui.Layout.
func (ui *UI) pumpFilePaneVolumeLookups(gtx layout.Context) {
	if ui == nil {
		return
	}
	// Deliberately not allFilePaneTabPanes. That helper builds a slice *and* a
	// map[*filePaneState]struct{} on every call, and this runs on every frame:
	// 60 of each per second, of pure GC churn, to walk a handful of pointers. The
	// dedup below is instead a linear rescan of the panes already visited, which
	// is allocation-free and, at the two or three tabs a real window carries,
	// cheaper than hashing. The semantics are identical — dedup by pointer, tab
	// sets first, then any pane in ui.filePanes not already covered.
	ui.ensureFilePaneTabs()
	next := time.Time{}
	for i := range ui.filePaneTabs {
		for j, pane := range ui.filePaneTabs[i].tabs {
			if pane == nil || ui.filePaneTabPaneVisitedBefore(pane, i, j) {
				continue
			}
			next = earlierFilePaneVolumeWakeup(next, pane.pumpVolumeLookup(gtx.Now))
		}
	}
	for i, pane := range ui.filePanes {
		if pane == nil || ui.filePaneTabPaneVisitedBefore(pane, len(ui.filePaneTabs), 0) {
			continue
		}
		if slices.Contains(ui.filePanes[:i], pane) {
			continue
		}
		next = earlierFilePaneVolumeWakeup(next, pane.pumpVolumeLookup(gtx.Now))
	}
	if next.IsZero() {
		return
	}
	gtx.Execute(op.InvalidateCmd{At: next})
}

// filePaneTabPaneVisitedBefore reports whether pane appears in ui.filePaneTabs
// strictly before tab tabIdx of column colIdx. It is the allocation-free stand-in
// for the dedup map allFilePaneTabPanes builds; pass colIdx == len(filePaneTabs)
// to ask about the tab sets as a whole.
func (ui *UI) filePaneTabPaneVisitedBefore(pane *filePaneState, colIdx, tabIdx int) bool {
	for i := 0; i < colIdx && i < len(ui.filePaneTabs); i++ {
		if slices.Contains(ui.filePaneTabs[i].tabs, pane) {
			return true
		}
	}
	if colIdx >= 0 && colIdx < len(ui.filePaneTabs) {
		return slices.Contains(ui.filePaneTabs[colIdx].tabs[:tabIdx], pane)
	}
	return false
}

func earlierFilePaneVolumeWakeup(next, at time.Time) time.Time {
	if at.IsZero() {
		return next
	}
	if next.IsZero() || at.Before(next) {
		return at
	}
	return next
}

// pumpVolumeLookup advances one pane's volume pipeline and returns when the
// frame loop next has to come back for it, or the zero time when it does not.
func (p *filePaneState) pumpVolumeLookup(now time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	state := &p.volumeBadge
	state.drainVolumeResults(now)
	state.sweepAbandonedLookups()

	if p.archiveBrowsing() {
		// An archive has no volume to measure. filePaneVolumeBadgeLabel refuses
		// these panes outright, so without this guard a pane that entered an
		// archive would keep polling until its wantedAt went stale — and
		// filePaneVolumeLookupSource returns "" for it, which resolves to the
		// working directory and would cache a reading for the wrong volume.
		state.cancelLookup()
		return time.Time{}
	}
	if state.wantedAt.IsZero() || now.Sub(state.wantedAt) > filePaneVolumeLookupIdleGrace {
		// Nothing has asked for this pane's free space recently. Any in-flight
		// lookup is left to finish into its channel; the pane simply stops
		// driving frames for it.
		return time.Time{}
	}
	// Unchecked: the archiveBrowsing guard above already ran this frame.
	source, remote := p.filePaneVolumeLookupSourceUnchecked()
	if state.volumeChanged(source, remote) {
		// Ahead of the in-flight check on purpose. The pane has moved to a volume
		// neither the cached reading nor the running lookup describes, and the
		// move need not have gone through invalidateVolumeBadge to get here —
		// attachPaneSSHSession flips the pane to remote seconds before any
		// listing lands, and waiting out filePaneVolumeLookupMaxWait would leave
		// the local disk's free space beside a remote path for that whole window.
		state.cancelLookup()
		state.discardReadingOnVolumeChange(source, remote)
	}

	if !state.lookupStart.IsZero() {
		if now.Sub(state.lookupStart) <= filePaneVolumeLookupMaxWait {
			return now.Add(filePaneVolumeLookupPollInterval)
		}
		// Over budget: parked in an uninterruptible syscall, or its result was
		// dropped by a full channel. Either way, stop waiting on it.
		state.cancelLookup()
	}

	if !filePaneVolumeLookupDue(state, source, remote, now) {
		return state.nextRefreshAt
	}
	// Both cancelLookup calls above can have just added an entry, and one of them
	// covers the case where the goroutine had already returned and only its result
	// went missing — a full channel. Sweeping again here lets that pane start its
	// replacement on this frame instead of waiting out a retry interval first.
	state.sweepAbandonedLookups()
	if state.lookupCapReached(source, remote) {
		// A goroutine abandoned on this same source is still out there. Come back
		// at the retry cadence rather than the in-flight one: nothing is going to
		// land in the meantime, and the pane is going to sit here for as long as
		// the mount stays wedged.
		return now.Add(filePaneVolumeBadgeRetryInterval)
	}
	p.startVolumeLookup(source, remote, now)
	return now.Add(filePaneVolumeLookupPollInterval)
}

func (s *filePaneVolumeBadgeState) drainVolumeResults(now time.Time) {
	if s == nil || s.resultCh == nil {
		return
	}
	for {
		select {
		case res := <-s.resultCh:
			if res.seq != s.lookupSeq {
				continue
			}
			s.applyVolumeResult(res, now)
		default:
			return
		}
	}
}

// applyVolumeResult lands a matching result. It clears the cancel func without
// going through cancelLookup, because bumping the sequence here would discard
// the very result being applied on any later re-entry.
func (s *filePaneVolumeBadgeState) applyVolumeResult(res filePaneVolumeResult, now time.Time) {
	if s.lookupCancel != nil {
		s.lookupCancel()
		s.lookupCancel = nil
	}
	s.lookupStart = time.Time{}
	// Not abandoned: the goroutine has produced its result and is on its way out,
	// so it is dropped rather than handed to the cap. The few microseconds it
	// still needs to run its deferred session.close() are not worth blocking the
	// next refresh over.
	s.lookupDone = nil
	s.checkedAt = now
	if res.err != nil || res.usage.TotalBytes == 0 {
		s.freeBytes = 0
		s.totalBytes = 0
		s.label = ""
		s.nextRefreshAt = now.Add(filePaneVolumeBadgeRetryInterval)
		return
	}
	s.freeBytes = res.usage.FreeBytes
	s.totalBytes = res.usage.TotalBytes
	s.label = formatFilePaneVolumeBadgeLabel(res.usage.FreeBytes, res.usage.TotalBytes)
	s.nextRefreshAt = now.Add(filePaneVolumeBadgeRefreshInterval)
}

// startVolumeLookup spawns the lookup goroutine. Everything it touches is either
// copied off the pane here or owned by the goroutine, so it never reads pane
// state concurrently with the frame loop.
//
// The three package-level seams are captured here rather than read inside the
// goroutine, for the same reason: they are ordinary vars that tests reassign, and
// a lookup outliving the test that started it would otherwise read one while the
// test's cleanup writes it.
func (p *filePaneState) startVolumeLookup(source string, remote bool, now time.Time) {
	if p == nil {
		return
	}
	state := &p.volumeBadge
	state.cancelLookup()
	state.lookupSource = source
	state.lookupRemote = remote

	// The session is cloned, not shared. clone() retains the underlying
	// sharedSSHConn, so the pane may be closed, disconnected, or reconnected to a
	// different host while this lookup is still parked in a round trip, and the
	// transport it is using stays alive until the goroutine releases it. This is
	// the same handoff startFileViewerLoad and the delete/copy pipelines use to
	// take a session onto a goroutine.
	var session *paneSSHSession
	if remote {
		session = p.remote.clone()
		if session == nil {
			state.checkedAt = now
			state.freeBytes = 0
			state.totalBytes = 0
			state.label = ""
			state.nextRefreshAt = now.Add(filePaneVolumeBadgeRetryInterval)
			return
		}
	}

	if state.resultCh == nil {
		state.resultCh = make(chan filePaneVolumeResult, 4)
	}
	state.lookupSeq++
	state.lookupStart = now
	seq := state.lookupSeq
	ch := state.resultCh

	ctx, cancel := context.WithCancel(context.Background())
	state.lookupCancel = cancel

	// done is the liveness signal the abandoned-lookup cap reads. Closing it is
	// the goroutine's last act, after the session clone has been released, so a
	// closed done means the retain() on the shared SSH connection is genuinely
	// gone and not merely no longer waited on.
	done := make(chan struct{})
	state.lookupDone = done

	stat := volumeLookupStatFunc
	localUsage := localVolumeUsageFunc
	remoteUsage := remoteVolumeUsageFunc

	go func() {
		defer close(done)
		defer cancel()
		if session != nil {
			defer session.close()
		}
		res := filePaneVolumeResult{seq: seq}
		// Resolving a local path stats, and a stat on a stale SMB or NFS mount is
		// exactly the kind of blocking this pipeline exists to keep off the frame,
		// so the resolution runs here rather than at the call site.
		lookupPath := resolveFilePaneVolumeLookupPath(source, remote, stat)
		switch {
		case lookupPath == "":
			res.err = errFilePaneVolumeLookupPath
		case remote:
			res.usage, res.err = remoteUsage(ctx, session, lookupPath)
		default:
			res.usage, res.err = localUsage(lookupPath)
		}
		sendFilePaneVolumeResult(ch, res)
	}()
}

// sendFilePaneVolumeResult never blocks. The pane may have been closed, or its
// frame loop may be idle, and a volume reading must not pin a goroutine on a
// channel nobody will read.
//
// Dropping when full is recoverable: the pane's filePaneVolumeLookupMaxWait
// budget expires and it starts a fresh lookup. The abandoned-lookup cap does not
// hold that one up, because a goroutine whose send was dropped goes on to close
// its done channel like any other, and the sweep in pumpVolumeLookup clears it on
// the same frame the budget expires.
func sendFilePaneVolumeResult(ch chan filePaneVolumeResult, res filePaneVolumeResult) {
	if ch == nil {
		return
	}
	select {
	case ch <- res:
	default:
	}
}

// filePaneVolumeLookupDue reports whether the cached reading has to be replaced.
//
// checkedAt rather than label is what marks "never looked up": a failed lookup
// leaves the label empty, so keying on the label would retry every frame and
// defeat filePaneVolumeBadgeRetryInterval entirely.
func filePaneVolumeLookupDue(state *filePaneVolumeBadgeState, source string, remote bool, now time.Time) bool {
	if state == nil {
		return false
	}
	if state.checkedAt.IsZero() || state.nextRefreshAt.IsZero() {
		return true
	}
	if state.lookupSource != source || state.lookupRemote != remote {
		return true
	}
	return !now.Before(state.nextRefreshAt)
}

// filePaneVolumeLookupSource returns the pane's unresolved lookup directory and
// whether it is remote. It performs no I/O, which is what makes it usable as the
// per-frame cache key; filePaneVolumeLookupPath turns it into a real path.
func (p *filePaneState) filePaneVolumeLookupSource() (string, bool) {
	if p == nil || p.archiveBrowsing() {
		return "", false
	}
	return p.filePaneVolumeLookupSourceUnchecked()
}

// filePaneVolumeLookupSourceUnchecked is filePaneVolumeLookupSource without the
// archive guard, for the two per-frame callers that have just run that guard
// themselves.
//
// The guard is not free: archiveBrowsing goes through filesys.ArchivePathActive,
// which cleans and splits the path and costs three allocations a call. Both the
// pump and the label reader check it first and then need the source, so sharing
// the one check halves what this pipeline asks of the garbage collector on every
// frame of every visible pane.
func (p *filePaneState) filePaneVolumeLookupSourceUnchecked() (string, bool) {
	if p == nil {
		return "", false
	}
	raw := strings.TrimSpace(p.loadingDir)
	if raw == "" {
		raw = strings.TrimSpace(p.dir)
	}
	return raw, p.remoteConnected()
}

// resolveFilePaneVolumeLookupPath turns a pane's raw lookup directory into a
// path a volume can be measured at. It is a free function rather than a method
// because it runs on the lookup goroutine: it must not read pane fields, and its
// local branch stats, which is the blocking call this whole pipeline exists to
// move off the frame.
func resolveFilePaneVolumeLookupPath(raw string, remote bool, stat statFunc) string {
	if remote {
		if raw == "" {
			raw = "/"
		}
		raw = strings.ReplaceAll(raw, `\`, `/`)
		clean := path.Clean(raw)
		if clean == "." {
			return "/"
		}
		return clean
	}
	if raw == "" {
		raw = "."
	}
	return nearestExistingLocalPath(raw, stat)
}

func remoteVolumeUsage(ctx context.Context, remote *paneSSHSession, lookupPath string) (platform.VolumeUsage, error) {
	usage, err := remoteVolumeUsageStatVFS(remote, lookupPath)
	if err == nil {
		return usage, nil
	}
	if !remoteVolumeUsageNeedsCommandFallback(err) {
		return platform.VolumeUsage{}, err
	}
	return remoteVolumeUsageDF(ctx, remote, lookupPath)
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

func remoteVolumeUsageDF(ctx context.Context, remote *paneSSHSession, lookupPath string) (platform.VolumeUsage, error) {
	usage, err := remoteVolumeUsageDFOnce(ctx, remote, lookupPath)
	if err == nil || !shouldReconnectSSHTransport(err) {
		return usage, err
	}
	if remote == nil {
		return platform.VolumeUsage{}, err
	}
	if reconnectErr := remote.reconnectSFTPClient(nil); reconnectErr != nil {
		return platform.VolumeUsage{}, reconnectErr
	}
	return remoteVolumeUsageDFOnce(ctx, remote, lookupPath)
}

func remoteVolumeUsageDFOnce(ctx context.Context, remote *paneSSHSession, lookupPath string) (platform.VolumeUsage, error) {
	if remote == nil {
		return platform.VolumeUsage{}, errors.New("remote ssh session is not connected")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmdline := "LC_ALL=C df -Pk " + shellQuote(lookupPath) + " 2>/dev/null | awk 'NR==2 {print $2, $4}'"
	content, _, errText := readViewerRemoteCommand(
		// The one interruptible leg of the remote lookup: cancelling the pane's
		// lookup cuts the df short. StatVFS and reconnectSFTPClient's SSH dial
		// have no context to take and run to their own budgets regardless.
		ctx,
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

func nearestExistingLocalPath(raw string, stat statFunc) string {
	if stat == nil {
		stat = os.Stat
	}
	path := strings.TrimSpace(raw)
	if path == "" {
		path = "."
	}
	path = filepath.Clean(path)
	for {
		info, err := stat(path)
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

	// No wakeup is scheduled here: pumpFilePaneVolumeLookups owns the poll
	// cadence, so the badge simply draws whatever reading has landed.
	label, ok := ui.filePaneVolumeBadgeLabel(gtx, sourcePane)
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
			return layout.Inset{
				Left:   filePaneVolumeBadgePaddingX,
				Right:  filePaneVolumeBadgePaddingX,
				Top:    filePaneVolumeBadgePaddingY,
				Bottom: filePaneVolumeBadgePaddingY,
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := ui.filePaneVolumeBadgeTextStyle(th, label)
				lbl.Color = textColor
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
	typeface := ui.mainTypeface()
	textSize := filePaneVolumeBadgeTextSize(th)
	if state.measuredLabel != label ||
		state.measuredWidth <= 0 ||
		state.measuredPxDp != gtx.Metric.PxPerDp ||
		state.measuredPxSp != gtx.Metric.PxPerSp ||
		state.measuredTypeface != typeface ||
		state.measuredTextSize != textSize {
		state.measuredLabel = label
		state.measuredWidth = ui.measureFilePaneVolumeBadgeWidth(th, gtx, label)
		state.measuredPxDp = gtx.Metric.PxPerDp
		state.measuredPxSp = gtx.Metric.PxPerSp
		state.measuredTypeface = typeface
		state.measuredTextSize = textSize
	}
	return state.measuredWidth
}

func filePaneVolumeBadgeTextSize(th *material.Theme) unit.Sp {
	return scaleThemeFontSize(th, 11)
}

func (ui *UI) filePaneVolumeBadgeTextStyle(th *material.Theme, label string) material.LabelStyle {
	lbl := material.Body2(th, label)
	lbl.Font.Typeface = ui.mainTypeface()
	lbl.TextSize = filePaneVolumeBadgeTextSize(th)
	lbl.MaxLines = 1
	return lbl
}

func (ui *UI) measureFilePaneVolumeBadgeWidth(th *material.Theme, gtx layout.Context, label string) int {
	lbl := ui.filePaneVolumeBadgeTextStyle(th, label)
	lbl.Truncator = ""

	width := measureLabelUnconstrained(gtx, lbl).Size.X
	width += gtx.Dp(filePaneVolumeBadgePaddingX + filePaneVolumeBadgePaddingX + filePaneVolumeBadgeWidthSlack)
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
