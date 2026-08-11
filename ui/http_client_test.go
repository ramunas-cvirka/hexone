// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"errors"
	"hexone/fm"
	"hexone/httpclient"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type httpTestSecretStore struct{ values map[string]string }

func (s *httpTestSecretStore) Get(key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (s *httpTestSecretStore) Set(key, value string) error {
	s.values[key] = value
	return nil
}

func (s *httpTestSecretStore) Delete(key string) error {
	delete(s.values, key)
	return nil
}

func TestHTTPClientStateLoadsAndSavesSeparateYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	if st.loadIssue != nil {
		t.Fatalf("newHTTPClientState: %v", st.loadIssue)
	}
	if st.currentRequest() == nil {
		t.Fatal("no request selected")
	}
	st.method = "PATCH"
	st.urlEd.SetText("{{base_url}}/changed")
	st.headersEd.SetText("X-One: first\nX-One: second\n# X-Off: disabled")
	st.bodyEd.SetText(`{"changed":true}`)

	ui := &UI{httpState: st, httpCollectionsPath: path}
	if err := ui.saveHTTPCollections(); err != nil {
		t.Fatalf("saveHTTPCollections: %v", err)
	}

	loaded, err := httpclient.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	request := loaded.Collections[0].Folders[0].Requests[0]
	if request.Method != "PATCH" || request.URL != "{{base_url}}/changed" || len(request.Headers) != 3 || !request.Headers[2].Disabled {
		t.Fatalf("saved request=%#v", request)
	}
}

func TestHTTPClientRequestExecutionReturnsThroughUIChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	st.sender = func(_ context.Context, request httpclient.Request, environment httpclient.Environment) httpclient.Response {
		if request.Method == "" || len(environment.Variables) == 0 {
			t.Errorf("incomplete request snapshot: %#v %#v", request, environment)
		}
		return httpclient.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Body:       []byte(`{"ok":true}`),
			Size:       11,
			Duration:   12 * time.Millisecond,
		}
	}
	ui := &UI{httpState: st, httpCollectionsPath: path}
	ui.startHTTPRequest()
	deadline := time.Now().Add(time.Second)
	for st.sending && time.Now().Before(deadline) {
		st.pollResult()
		time.Sleep(time.Millisecond)
	}
	if st.sending {
		t.Fatal("request result was not delivered")
	}
	if st.response.StatusCode != 200 || st.responseEd.Text() == "" {
		t.Fatalf("response not reflected in state: %#v body=%q", st.response, st.responseEd.Text())
	}
}

func TestHTTPURLSubmitSendsRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	sent := make(chan httpclient.Request, 1)
	st.sender = func(_ context.Context, request httpclient.Request, _ httpclient.Environment) httpclient.Response {
		sent <- request
		return httpclient.Response{StatusCode: 200, Status: "200 OK"}
	}
	ui := &UI{httpState: st, httpCollectionsPath: path}
	router := new(input.Router)
	th := material.NewTheme()
	frame := func(focus bool) {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(500, 32)),
			Now:         time.Now(),
		}
		if focus {
			gtx.Execute(key.FocusCmd{Tag: &st.urlEd})
		}
		ui.handleHTTPURLSubmit(gtx, st)
		editor := material.Editor(th, &st.urlEd, "")
		editor.Layout(gtx)
		router.Frame(&ops)
	}

	frame(true)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(false)
	select {
	case request := <-sent:
		if request.URL != st.urlEd.Text() {
			t.Fatalf("submitted URL=%q editor=%q", request.URL, st.urlEd.Text())
		}
	case <-time.After(time.Second):
		t.Fatal("pressing Enter in URL editor did not send the request")
	}
}

func TestHTTPClientToolActivationUsesTab3(t *testing.T) {
	ui := NewUI(fm.DefaultConfig())
	ui.activateFunctionBarTool("http", time.Now())
	if ui.Tabs.Value != "tab3" {
		t.Fatalf("active tab=%q want tab3", ui.Tabs.Value)
	}
}

func TestHTTPClientRequestTabsPreserveDraftsAndCloseWithoutDeleting(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	if len(st.openTabs) != 1 || st.activeTab != 0 {
		t.Fatalf("initial tabs=%v active=%d", st.openTabs, st.activeTab)
	}
	first := st.openTabs[0]
	firstRequest := st.requestAt(first)
	if firstRequest == nil {
		t.Fatal("initial request missing")
	}
	originalCount := len(st.file.Collections[0].Folders[0].Requests)
	st.urlEd.SetText("{{base_url}}/draft")

	second := httpRequestRef{collection: 0, folder: 0, request: 1}
	st.selectRequest(second)
	if len(st.openTabs) != 2 || st.activeTab != 1 {
		t.Fatalf("opened tabs=%v active=%d", st.openTabs, st.activeTab)
	}
	if firstRequest.URL != "{{base_url}}/draft" {
		t.Fatalf("first tab draft URL=%q", firstRequest.URL)
	}
	if !st.activateRequestTab(0) || st.urlEd.Text() != "{{base_url}}/draft" {
		t.Fatalf("reactivated first tab URL=%q", st.urlEd.Text())
	}
	if !st.closeRequestTab(0) {
		t.Fatal("closeRequestTab returned false")
	}
	if len(st.openTabs) != 1 || len(st.file.Collections[0].Folders[0].Requests) != originalCount {
		t.Fatalf("closing tab deleted request: tabs=%d requests=%d", len(st.openTabs), len(st.file.Collections[0].Folders[0].Requests))
	}
}

func TestHTTPClientAddTabCreatesSavableScratchRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	before := len(st.openTabs)
	if !st.addScratchRequest() {
		t.Fatal("addScratchRequest returned false")
	}
	if len(st.openTabs) != before+1 || st.activeTab != len(st.openTabs)-1 || !st.dirty {
		t.Fatalf("tabs=%d active=%d dirty=%v", len(st.openTabs), st.activeTab, st.dirty)
	}
	request := st.currentRequest()
	if request == nil || request.Name != "New request" || request.Method != "GET" {
		t.Fatalf("scratch request=%#v", request)
	}
	ui := &UI{httpState: st, httpCollectionsPath: path}
	if err := ui.saveHTTPCollections(); err != nil {
		t.Fatalf("saveHTTPCollections: %v", err)
	}
	loaded, err := httpclient.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	lastCollection := loaded.Collections[len(loaded.Collections)-1]
	if len(lastCollection.Requests) == 0 || lastCollection.Requests[len(lastCollection.Requests)-1].Name != "New request" {
		t.Fatalf("scratch request not saved: %#v", lastCollection.Requests)
	}
}

func TestHTTPClientTabRoundTripsThroughSession(t *testing.T) {
	source := NewUI(fm.DefaultConfig())
	source.Tabs.Value = "tab3"
	session := source.SnapshotSession()
	if session.ActiveTab != "tab3" {
		t.Fatalf("snapshot active tab=%q want tab3", session.ActiveTab)
	}

	target := NewUI(fm.DefaultConfig())
	target.ApplySession(session)
	if target.Tabs.Value != "tab3" {
		t.Fatalf("restored active tab=%q want tab3", target.Tabs.Value)
	}
}

func TestParseKeyValueLinesSupportsDisabledAndDuplicateRows(t *testing.T) {
	got := parseKeyValueLines("X-Test: one\nX-Test: two\n# X-Off: no", ":")
	if len(got) != 3 || got[1].Value != "two" || !got[2].Disabled {
		t.Fatalf("parseKeyValueLines=%#v", got)
	}
}

func TestHTTPCollectionRowsDescribePaintedProtocolTree(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	rows := st.collectionRows()
	if len(rows) != 4 {
		t.Fatalf("row count=%d want 4: %#v", len(rows), rows)
	}
	want := []struct {
		kind        string
		depth       int
		last        bool
		hasChildren bool
	}{
		{kind: "collection", depth: 0, last: true, hasChildren: true},
		{kind: "folder", depth: 1, last: true, hasChildren: true},
		{kind: "request", depth: 2, last: false},
		{kind: "request", depth: 2, last: true},
	}
	for index, expected := range want {
		if rows[index].kind != expected.kind || rows[index].depth != expected.depth || rows[index].last != expected.last || rows[index].hasChildren != expected.hasChildren {
			t.Fatalf("row %d=%#v want kind=%q depth=%d last=%t children=%t", index, rows[index], expected.kind, expected.depth, expected.last, expected.hasChildren)
		}
	}
}

func TestHTTPCollectionTreeGroupsCollapseAndExpand(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	rows := st.collectionRows()
	st.selectTreeGroup(rows[1])
	rows = st.collectionRows()
	if len(rows) != 2 || rows[1].kind != "folder" || rows[1].expanded {
		t.Fatalf("collapsed folder rows=%#v", rows)
	}
	st.selectTreeGroup(rows[1])
	if got := len(st.collectionRows()); got != 4 {
		t.Fatalf("expanded folder row count=%d want 4", got)
	}
}

func TestHTTPCollectionActionsAddPersistableHierarchy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	if !st.addCollection() {
		t.Fatal("addCollection returned false")
	}
	collectionIndex := len(st.file.Collections) - 1
	if !st.addFolderToSelection() {
		t.Fatal("addFolderToSelection returned false")
	}
	if !st.addRequestToSelection() {
		t.Fatal("addRequestToSelection returned false")
	}
	if st.selected.collection != collectionIndex || st.selected.folder != 0 {
		t.Fatalf("new request selection=%#v", st.selected)
	}
	ui := &UI{httpState: st, httpCollectionsPath: path}
	if err := ui.saveHTTPCollections(); err != nil {
		t.Fatal(err)
	}
	loaded, err := httpclient.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	created := loaded.Collections[collectionIndex]
	if len(created.Folders) != 1 || len(created.Folders[0].Requests) != 1 {
		t.Fatalf("created hierarchy=%#v", created)
	}
}

func TestHTTPCompactMethodKeepsPatchVisible(t *testing.T) {
	if got := compactHTTPMethod("PATCH"); got != "PATCH" {
		t.Fatalf("compactHTTPMethod(PATCH)=%q want PATCH", got)
	}
	if got := compactHTTPMethod("DELETE"); got != "DEL" {
		t.Fatalf("compactHTTPMethod(DELETE)=%q want DEL", got)
	}
	if got := compactHTTPMethod("OPTIONS"); got != "OPT" {
		t.Fatalf("compactHTTPMethod(OPTIONS)=%q want OPT", got)
	}
}

func TestHTTPTreeRenameCommitsAndCancelsCollectionAndRequestNames(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	collectionRow := st.collectionRows()[0]
	if !st.beginTreeRename(collectionRow) || !st.treeRenameActive || !st.treeRenameFocus {
		t.Fatal("collection rename did not become active and request focus")
	}
	st.treeRenameEd.SetText("Renamed API")
	if !st.finishTreeRename(true) {
		t.Fatal("collection rename was not committed")
	}
	if got := st.file.Collections[0].Name; got != "Renamed API" {
		t.Fatalf("collection name=%q want Renamed API", got)
	}

	var requestRow httpCollectionRow
	for _, row := range st.collectionRows() {
		if row.kind == "request" {
			requestRow = row
			break
		}
	}
	original, ok := st.treeRenameName(requestRow.kind, requestRow.ref)
	if !ok {
		t.Fatal("default collection has no request row")
	}
	if !st.beginTreeRename(requestRow) {
		t.Fatal("request rename did not start")
	}
	st.treeRenameEd.SetText("Discard me")
	if st.finishTreeRename(false) {
		t.Fatal("cancelled request rename reported a change")
	}
	if got, _ := st.treeRenameName(requestRow.kind, requestRow.ref); got != original {
		t.Fatalf("cancelled request name=%q want %q", got, original)
	}

	if !st.beginTreeRename(requestRow) {
		t.Fatal("second request rename did not start")
	}
	st.treeRenameEd.SetText("Health probe")
	if !st.finishTreeRename(true) {
		t.Fatal("request rename was not committed")
	}
	if got, _ := st.treeRenameName(requestRow.kind, requestRow.ref); got != "Health probe" {
		t.Fatalf("request name=%q want Health probe", got)
	}
}

func TestHTTPTreeRenameEnterCommitsAndEscapeCancels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	ui := &UI{httpState: st, httpCollectionsPath: path}
	router := new(input.Router)
	th := material.NewTheme()
	frame := func(focus bool) {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(240, 24)),
			Now:         time.Now(),
		}
		ui.handleHTTPTreeRename(gtx, st)
		if focus {
			gtx.Execute(key.FocusCmd{Tag: &st.treeRenameEd})
		}
		material.Editor(th, &st.treeRenameEd, "").Layout(gtx)
		router.Frame(&ops)
	}

	collectionRow := st.collectionRows()[0]
	st.beginTreeRename(collectionRow)
	st.treeRenameEd.SetText("Committed with Enter")
	frame(true)
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame(false)
	if st.treeRenameActive || st.file.Collections[0].Name != "Committed with Enter" {
		t.Fatalf("Enter left active=%t name=%q", st.treeRenameActive, st.file.Collections[0].Name)
	}
	loaded, err := httpclient.Load(path)
	if err != nil || loaded.Collections[0].Name != "Committed with Enter" {
		t.Fatalf("Enter did not persist rename: loaded=%#v err=%v", loaded, err)
	}

	requestRow := st.collectionRows()[2]
	original, _ := st.treeRenameName(requestRow.kind, requestRow.ref)
	st.beginTreeRename(requestRow)
	st.treeRenameEd.SetText("Cancelled with Escape")
	frame(true)
	router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame(false)
	got, _ := st.treeRenameName(requestRow.kind, requestRow.ref)
	if st.treeRenameActive || got != original {
		t.Fatalf("Escape left active=%t name=%q want %q", st.treeRenameActive, got, original)
	}
}

func TestHTTPTreeSelectingDifferentRequestCancelsRename(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	rows := st.collectionRows()
	firstRequest := rows[2]
	secondRequest := rows[3]
	original, _ := st.treeRenameName(secondRequest.kind, secondRequest.ref)
	if !st.beginTreeRename(secondRequest) {
		t.Fatal("request rename did not start")
	}
	st.treeRenameEd.SetText("This must be discarded")
	if !st.selectTreeRequest(firstRequest) {
		t.Fatal("selecting a different request did not report a cancelled rename")
	}
	if st.treeRenameActive {
		t.Fatal("rename remained active after selecting another request")
	}
	if st.selected != firstRequest.ref {
		t.Fatalf("selected request=%#v want %#v", st.selected, firstRequest.ref)
	}
	if got, _ := st.treeRenameName(secondRequest.kind, secondRequest.ref); got != original {
		t.Fatalf("cancelled rename changed name to %q want %q", got, original)
	}
}

func TestHTTPCollectionDoubleClickStartsInlineRename(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	ui := &UI{httpState: st}
	requestRow := st.collectionRows()[3]
	originalRequestName, _ := st.treeRenameName(requestRow.kind, requestRow.ref)
	st.beginTreeRename(requestRow)
	st.treeRenameEd.SetText("request draft to discard")
	st.methodMenuOpen = true
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(220, 28)),
			Now:         time.Now(),
		}
		ui.handleHTTPClientClicks(gtx, st)
		st.groupClicks[0].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	click := func(at time.Duration) {
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Time: at, Position: f32.Pt(80, 14)},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Time: at + time.Millisecond, Position: f32.Pt(80, 14)},
		)
		frame()
	}

	frame()
	click(time.Millisecond)
	if st.envEditorOpen || st.environment != 0 {
		t.Fatalf("first click changed environment before double-click window: open=%t selected=%d", st.envEditorOpen, st.environment)
	}
	click(100 * time.Millisecond)
	if !st.treeRenameActive || st.treeRenameKind != "collection" || st.treeRenameRef.collection != 0 {
		t.Fatalf("collection double-click rename active=%t kind=%q ref=%#v", st.treeRenameActive, st.treeRenameKind, st.treeRenameRef)
	}
	if st.collapsedCollections[0] {
		t.Fatal("collection double-click changed expansion state")
	}
	if st.methodMenuOpen || st.envMenuOpen {
		t.Fatal("tree click left a choice menu open")
	}
	if got, _ := st.treeRenameName(requestRow.kind, requestRow.ref); got != originalRequestName {
		t.Fatalf("collection rename committed previous request draft as %q want %q", got, originalRequestName)
	}
}

func TestHTTPEnvironmentSingleClickCyclesAfterDoubleClickWindow(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	st.addEnvironment()
	st.environment = 0
	ui := &UI{httpState: st}
	router := new(input.Router)
	now := time.Unix(10, 0)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(180, 28)),
			Now:         now,
		}
		ui.handleHTTPClientClicks(gtx, st)
		st.envClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	frame()
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Time: time.Millisecond, Position: f32.Pt(80, 14)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Time: 2 * time.Millisecond, Position: f32.Pt(80, 14)},
	)
	frame()
	if st.environment != 0 || !st.envCyclePending {
		t.Fatalf("single click cycled early: selected=%d pending=%t", st.environment, st.envCyclePending)
	}
	now = now.Add(401 * time.Millisecond)
	frame()
	if st.environment != 1 || st.envCyclePending || st.envEditorOpen {
		t.Fatalf("deferred single click selected=%d pending=%t editor=%t", st.environment, st.envCyclePending, st.envEditorOpen)
	}
}

func TestHTTPFolderDoubleClickStartsInlineRename(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	ui := &UI{httpState: st}
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(220, 28)),
			Now:         time.Now(),
		}
		ui.handleHTTPClientClicks(gtx, st)
		st.groupClicks[1].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	click := func(at time.Duration) {
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Time: at, Position: f32.Pt(80, 14)},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Time: at + time.Millisecond, Position: f32.Pt(80, 14)},
		)
		frame()
	}
	frame()
	click(time.Millisecond)
	click(100 * time.Millisecond)
	if !st.treeRenameActive || st.treeRenameKind != "folder" || st.treeRenameRef.folder != 0 {
		t.Fatalf("folder double-click rename active=%t kind=%q ref=%#v", st.treeRenameActive, st.treeRenameKind, st.treeRenameRef)
	}
	st.treeRenameEd.SetText("Renamed examples")
	if !st.finishTreeRename(true) || st.file.Collections[0].Folders[0].Name != "Renamed examples" {
		t.Fatalf("folder rename did not commit: %#v", st.file.Collections[0].Folders[0])
	}
}

func TestHTTPTreeSecondaryClickOpensContextMenu(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	st.methodMenuOpen = true
	st.envMenuOpen = true
	ui := &UI{httpState: st}
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(220, 28)),
			Now:         time.Now(),
		}
		ui.handleHTTPClientClicks(gtx, st)
		st.requestClicks[2].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	frame()
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonSecondary, Position: f32.Pt(80, 14)})
	frame()
	if !st.treeMenuOpen || st.methodMenuOpen || st.envMenuOpen || st.treeMenuRow.kind != "request" || st.treeMenuRow.ref != st.collectionRows()[2].ref {
		t.Fatalf("tree context menu open=%t method=%t environment=%t row=%#v", st.treeMenuOpen, st.methodMenuOpen, st.envMenuOpen, st.treeMenuRow)
	}
}

func TestHTTPTreeContextRunUsesChosenEnvironment(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	st.addEnvironment()
	st.file.Environments[1].Name = "staging"
	st.file.Environments[1].Variables["base_url"] = "https://staging.example.test"
	sentEnvironment := make(chan httpclient.Environment, 1)
	st.sender = func(_ context.Context, _ httpclient.Request, environment httpclient.Environment) httpclient.Response {
		sentEnvironment <- environment
		return httpclient.Response{StatusCode: 200, Status: "200 OK"}
	}
	ui := &UI{httpState: st}
	st.treeMenuOpen = true
	st.treeMenuRow = st.collectionRows()[2]
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(180, 28)),
			Now:         time.Now(),
		}
		ui.handleHTTPClientClicks(gtx, st)
		st.treeMenuRunClicks[1].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	frame()
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(80, 14)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(80, 14)},
	)
	frame()
	select {
	case environment := <-sentEnvironment:
		if environment.Name != "staging" || st.environment != 1 || st.treeMenuOpen {
			t.Fatalf("run environment=%#v selected=%d menu=%t", environment, st.environment, st.treeMenuOpen)
		}
	case <-time.After(time.Second):
		t.Fatal("context-menu run did not send the request")
	}
}

func TestHTTPTreeDeleteRemovesRowsAndReselects(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	requestRow := st.collectionRows()[2]
	if !st.deleteTreeRow(requestRow) {
		t.Fatal("request delete failed")
	}
	if len(st.file.Collections[0].Folders[0].Requests) != 1 || !st.hasSelected || st.currentRequest() == nil {
		t.Fatalf("request delete left state=%#v selected=%t", st.file.Collections[0].Folders[0], st.hasSelected)
	}

	st = newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	folderRow := st.collectionRows()[1]
	if !st.deleteTreeRow(folderRow) || len(st.file.Collections[0].Folders) != 0 {
		t.Fatalf("folder delete left collections=%#v", st.file.Collections)
	}

	st = newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	collectionRow := st.collectionRows()[0]
	if !st.deleteTreeRow(collectionRow) || len(st.file.Collections) == 0 || st.currentRequest() == nil {
		t.Fatalf("collection delete did not create/select a scratch request: %#v", st.file.Collections)
	}
}

func TestHTTPChoiceMenuGlobalDismissClosesOnlyOutside(t *testing.T) {
	st := &httpClientState{methodMenuOpen: true, envMenuOpen: true}
	router := new(input.Router)
	frame := func(menuSurface bool) {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(240, 80)),
			Now:         time.Now(),
		}
		handleHTTPChoiceMenuDismissPresses(gtx, st)
		if menuSurface {
			registerHTTPMenuSurface(gtx, st, gtx.Constraints.Max)
		}
		registerHTTPMenuDismissGlobal(gtx, st)
		router.Frame(&ops)
	}
	frame(false)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(120, 40)})
	frame(false)
	if st.methodMenuOpen || st.envMenuOpen {
		t.Fatalf("outside click left method=%t environment=%t open", st.methodMenuOpen, st.envMenuOpen)
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(120, 40)})
	frame(false)

	st.methodMenuOpen = true
	frame(true)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(120, 40)})
	frame(true)
	if !st.methodMenuOpen {
		t.Fatal("click inside the menu surface closed it before its item could run")
	}
}

func TestHTTPAuthAndAddedEnvironmentPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	st.requestAuth.set(httpclient.Auth{Type: httpclient.AuthBearer, Token: "{{token}}"}, true)
	st.applyEditorsToRequest()
	if !st.addEnvironment() {
		t.Fatal("addEnvironment returned false")
	}
	st.file.Environments[st.environment].Variables["token"] = "secret"
	ui := &UI{httpState: st, httpCollectionsPath: path}
	if err := ui.saveHTTPCollections(); err != nil {
		t.Fatal(err)
	}
	loaded, err := httpclient.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	request := loaded.Collections[st.selected.collection].Folders[st.selected.folder].Requests[st.selected.request]
	if request.Auth.Type != httpclient.AuthBearer || request.Auth.Token != "{{token}}" {
		t.Fatalf("auth=%#v", request.Auth)
	}
	if got := loaded.Environments[len(loaded.Environments)-1].Variables["token"]; got != "secret" {
		t.Fatalf("added environment token=%q", got)
	}
}

func TestHTTPEnvironmentEditorSavesNameAndParameters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	ui := &UI{httpState: st, httpCollectionsPath: path}
	if !st.openEnvironmentEditor(0, true) {
		t.Fatal("environment editor did not open")
	}
	st.envEditorNameEd.SetText("staging")
	st.envEditorVarsEd.SetText("base_url=https://staging.example.test\ntoken=abc=123\n# ignored\ninvalid")
	st.environmentAuth.set(httpclient.Auth{Type: httpclient.AuthAPIKey, Key: "X-API-Key", Value: "{{token}}", In: httpclient.AuthInHeader}, false)
	if !ui.saveEnvironmentEditor(st) {
		t.Fatal("environment editor reported no changes")
	}
	if st.envEditorOpen {
		t.Fatal("environment editor remained open after save")
	}
	loaded, err := httpclient.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	environment := loaded.Environments[0]
	if environment.Name != "staging" || environment.Variables["base_url"] != "https://staging.example.test" || environment.Variables["token"] != "abc=123" {
		t.Fatalf("saved environment=%#v", environment)
	}
	if _, exists := environment.Variables["invalid"]; exists {
		t.Fatalf("invalid parameter line was persisted: %#v", environment.Variables)
	}
	if environment.Auth.Type != httpclient.AuthAPIKey || environment.Auth.Key != "X-API-Key" || environment.Auth.Value != "{{token}}" || environment.Auth.In != httpclient.AuthInHeader {
		t.Fatalf("saved environment auth=%#v", environment.Auth)
	}
}

func TestHTTPAuthEditorSupportsStructuredModesAndInheritance(t *testing.T) {
	var editor httpAuthEditorState
	editor.init()
	tests := []struct {
		name         string
		auth         httpclient.Auth
		allowInherit bool
	}{
		{name: "basic", auth: httpclient.Auth{Type: httpclient.AuthBasic, Username: "{{user}}", Password: "{{password}}"}, allowInherit: true},
		{name: "bearer", auth: httpclient.Auth{Type: httpclient.AuthBearer, Token: "{{token}}"}, allowInherit: true},
		{name: "API key header", auth: httpclient.Auth{Type: httpclient.AuthAPIKey, Key: "X-API-Key", Value: "{{api_key}}", In: httpclient.AuthInHeader}, allowInherit: true},
		{name: "API key query", auth: httpclient.Auth{Type: httpclient.AuthAPIKey, Key: "key", Value: "{{api_key}}", In: httpclient.AuthInQuery}, allowInherit: false},
		{name: "inherit", auth: httpclient.Auth{Type: httpclient.AuthInherit}, allowInherit: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			editor.set(test.auth, test.allowInherit)
			if got := editor.value(test.allowInherit); got != test.auth {
				t.Fatalf("editor auth=%#v want %#v", got, test.auth)
			}
		})
	}
}

func TestHTTPAuthModeControlsSwitchRequestAndEnvironmentModes(t *testing.T) {
	router := new(input.Router)
	requestAuth := httpAuthEditorState{}
	requestAuth.init()
	environmentAuth := httpAuthEditorState{}
	environmentAuth.init()
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(160, 28)),
			Now:         time.Now(),
		}
		handleHTTPAuthEditorClicks(gtx, &requestAuth, true)
		handleHTTPAuthEditorClicks(gtx, &environmentAuth, false)
		requestAuth.typeClicks[2].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	frame()
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(80, 14)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(80, 14)},
	)
	frame()
	if requestAuth.typeName != httpclient.AuthBasic {
		t.Fatalf("request auth type=%q want basic", requestAuth.typeName)
	}

	environmentAuth.typeName = httpclient.AuthAPIKey
	environmentAuth.location = httpclient.AuthInHeader
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(160, 28)),
	}
	environmentAuth.queryClick.Click()
	if !handleHTTPAuthEditorClicks(gtx, &environmentAuth, false) || environmentAuth.location != httpclient.AuthInQuery {
		t.Fatalf("environment API key location=%q want query", environmentAuth.location)
	}
}

func TestHTTPEnvironmentEditorCancelLeavesValuesUntouched(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	original := st.file.Environments[0]
	st.openEnvironmentEditor(0, true)
	st.envEditorNameEd.SetText("discarded")
	st.envEditorVarsEd.SetText("token=discarded")
	st.environmentAuth.set(httpclient.Auth{Type: httpclient.AuthBearer, Token: "discarded"}, false)
	st.closeEnvironmentEditor()
	got := st.file.Environments[0]
	if got.Name != original.Name || formatHTTPEnvironmentVariables(got.Variables) != formatHTTPEnvironmentVariables(original.Variables) || got.Auth != original.Auth {
		t.Fatalf("cancel changed environment from %#v to %#v", original, got)
	}
}

func TestHTTPEnvironmentEditorIgnoresCredentialMetadataForDirtyState(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	st.file.Environments[0].Auth = httpclient.Auth{
		Type: httpclient.AuthBearer, Token: "secret", CredentialID: "opaque-reference",
	}
	st.openEnvironmentEditor(0, true)
	ui := &UI{httpState: st}
	if ui.saveEnvironmentEditor(st) {
		t.Fatal("unchanged environment was reported as changed")
	}
	if st.dirty {
		t.Fatal("unchanged environment marked the collection dirty")
	}
	if got := st.file.Environments[0].Auth.CredentialID; got != "opaque-reference" {
		t.Fatalf("credential reference=%q want preserved", got)
	}
}

func TestHTTPSelectingRequestCancelsEnvironmentEditor(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	original := st.file.Environments[0]
	st.openEnvironmentEditor(0, true)
	st.envEditorNameEd.SetText("discarded")
	st.envEditorVarsEd.SetText("token=discarded")
	requestRow := st.collectionRows()[2]
	if !st.selectTreeRequest(requestRow) {
		t.Fatal("request selection did not report closing the environment editor")
	}
	if st.envEditorOpen {
		t.Fatal("environment editor remained open after request selection")
	}
	got := st.file.Environments[0]
	if got.Name != original.Name || formatHTTPEnvironmentVariables(got.Variables) != formatHTTPEnvironmentVariables(original.Variables) {
		t.Fatalf("request selection saved environment draft from %#v to %#v", original, got)
	}
}

func TestHTTPEnvironmentVariableFormattingIsStable(t *testing.T) {
	formatted := formatHTTPEnvironmentVariables(map[string]string{"token": "secret", "base_url": "https://example.test"})
	if formatted != "base_url=https://example.test\ntoken=secret" {
		t.Fatalf("formatted variables=%q", formatted)
	}
	parsed := parseHTTPEnvironmentVariables(formatted)
	if parsed["base_url"] != "https://example.test" || parsed["token"] != "secret" {
		t.Fatalf("parsed variables=%#v", parsed)
	}
}

func TestHTTPEnvironmentDoubleClickOpensCurrentEditor(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	st.addEnvironment()
	st.environment = 0
	ui := &UI{httpState: st}
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(180, 28)),
			Now:         time.Now(),
		}
		ui.handleHTTPClientClicks(gtx, st)
		st.envClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	click := func(at time.Duration) {
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Time: at, Position: f32.Pt(80, 14)},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Time: at + time.Millisecond, Position: f32.Pt(80, 14)},
		)
		frame()
	}
	frame()
	click(time.Millisecond)
	click(100 * time.Millisecond)
	if !st.envEditorOpen || st.envEditorIndex != 0 || st.environment != 0 {
		t.Fatalf("environment double-click open=%t editor=%d selected=%d", st.envEditorOpen, st.envEditorIndex, st.environment)
	}
}

func TestHTTPNewEnvironmentActionCreatesAndOpensEditor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	st := newHTTPClientState(path)
	ui := &UI{httpState: st, httpCollectionsPath: path}
	originalCount := len(st.file.Environments)
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(80, 28)),
			Now:         time.Now(),
		}
		ui.handleHTTPClientClicks(gtx, st)
		st.addEnvironmentClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	frame()
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(40, 14)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(40, 14)},
	)
	frame()
	if len(st.file.Environments) != originalCount+1 || !st.envEditorOpen || st.envEditorIndex != originalCount || st.envEditorFocus != "name" {
		t.Fatalf("new environment count=%d open=%t editor=%d focus=%q", len(st.file.Environments), st.envEditorOpen, st.envEditorIndex, st.envEditorFocus)
	}
}

func TestHTTPCollectionSplitWidthIsResizableAndClamped(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	width := httpCollectionSplitWidth(gtx, 1000, 310)
	if width != 310 {
		t.Fatalf("regular split width=%d", width)
	}
	if resized := httpCollectionSplitWidth(gtx, 1200, width); resized != width {
		t.Fatalf("window resize changed collection width from %d to %d", width, resized)
	}
	width = httpCollectionSplitWidth(gtx, 1000, 900)
	if width != 550 {
		t.Fatalf("maximum split width=%d", width)
	}
	width = httpCollectionSplitWidth(gtx, 400, 20)
	if width != 160 {
		t.Fatalf("compact split width=%d", width)
	}
}

func TestHTTPCollectionSplitHandleDragChangesPixels(t *testing.T) {
	router := new(input.Router)
	handle := new(httpSplitHandle)
	width := 60
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(200, 100)),
			Source:      router.Source(),
		}
		layoutHTTPSplitPixelsOverlay(gtx, handle, layout.Horizontal, image.Rect(56, 0, 65, 100), &width, 20, 150)
		router.Frame(&ops)
	}

	frame()
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(60, 50)})
	frame()
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(100, 50)})
	frame()
	if width != 100 {
		t.Fatalf("dragged width=%d want 100", width)
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(100, 50)})
	frame()
	if handle.dragging {
		t.Fatal("split handle remained in dragging state after release")
	}
}

func TestHTTPHorizontalSplitHandleDragChangesRatio(t *testing.T) {
	router := new(input.Router)
	handle := new(httpSplitHandle)
	ratio := float32(0.30)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(200, 100)),
			Source:      router.Source(),
		}
		layoutHTTPSplitOverlay(gtx, handle, layout.Vertical, image.Rect(0, 26, 200, 35), &ratio, 100, 0.18, 0.75)
		router.Frame(&ops)
	}

	frame()
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(100, 30)})
	frame()
	if !handle.hovering {
		t.Fatal("request/response split handle did not enter hover state")
	}
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(100, 30)})
	frame()
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(100, 60)})
	frame()
	if ratio < 0.59 || ratio > 0.61 {
		t.Fatalf("dragged ratio=%v want about 0.60", ratio)
	}
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(100, 60)})
	frame()
	if handle.dragging {
		t.Fatal("request/response split handle remained active after release")
	}
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(100, 90)})
	frame()
	if handle.hovering {
		t.Fatal("request/response split handle remained hovered after pointer left")
	}
}

func TestHTTPResponseTabsDoNotCompeteWithSplitHandle(t *testing.T) {
	st := newHTTPClientState(filepath.Join(t.TempDir(), "hexone-http.yaml"))
	ui := &UI{httpState: st}
	th := material.NewTheme()
	router := new(input.Router)
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(900, 600)),
			Source:      router.Source(),
			Now:         time.Now(),
		}
		ui.handleHTTPClientClicks(gtx, st)
		ui.layoutHTTPWorkbench(th, gtx, st)
		router.Frame(&ops)
	}

	frame()
	if st.requestSplitMinX <= 180 || st.requestSplitMaxX <= st.requestSplitMinX {
		t.Fatalf("split rail bounds=%d..%d do not leave the response tabs clear", st.requestSplitMinX, st.requestSplitMaxX)
	}
	tabY := float32(st.requestSplitY)
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(155, tabY)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(155, tabY)},
	)
	frame()
	if st.responseMode != httpResponseRaw {
		t.Fatalf("response mode=%q want raw after clicking its tab", st.responseMode)
	}
	if st.requestSplit.dragging {
		t.Fatal("response tab click started a split drag")
	}

	railX := float32((st.requestSplitMinX + st.requestSplitMaxX) / 2)
	originalRatio := st.requestRatio
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(railX, tabY)})
	frame()
	if !st.requestSplit.hovering {
		t.Fatal("empty response rail is not available as a resize handle")
	}
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(railX, tabY)})
	frame()
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(railX, tabY+30)})
	frame()
	if st.requestRatio <= originalRatio {
		t.Fatalf("split ratio=%v did not increase from %v", st.requestRatio, originalRatio)
	}
}

func TestHTTPSelectorSecondaryClickOpensChoiceMenus(t *testing.T) {
	router := new(input.Router)
	st := &httpClientState{envMenuOpen: true, treeMenuOpen: true}
	frame := func() {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(120, 40)),
			Source:      router.Source(),
		}
		handleHTTPSelectorSecondaryPresses(gtx, st)
		st.methodClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}

	frame()
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonSecondary, Position: f32.Pt(30, 20)})
	frame()
	if !st.methodMenuOpen || st.envMenuOpen || st.treeMenuOpen {
		t.Fatalf("method menu open=%v environment menu open=%v tree menu open=%v", st.methodMenuOpen, st.envMenuOpen, st.treeMenuOpen)
	}
}

func TestHTTPUIUsesCredentialVaultForCollections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone-http.yaml")
	store := &httpTestSecretStore{values: make(map[string]string)}
	ui := NewUI(fm.DefaultConfig())
	ui.httpCollectionsPath = path
	ui.httpCredentials = httpCredentialState{store: store, initialized: true}
	st := ui.ensureHTTPClientState()
	if st.vault == nil || st.loadIssue != nil {
		t.Fatalf("secure HTTP state vault=%v load issue=%v", st.vault, st.loadIssue)
	}
	st.file.Environments[0].Variables["token"] = "environment-secret"
	request := st.currentRequest()
	request.Auth = httpclient.Auth{Type: httpclient.AuthBearer, Token: "request-secret"}
	st.syncEditorsFromRequest()
	if err := ui.saveHTTPCollections(); err != nil {
		t.Fatal(err)
	}
	st.updateDirty()
	if st.dirty {
		t.Fatal("secure save remained dirty because of credential metadata")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"environment-secret", "request-secret", "localhost:8080"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("HTTP YAML contains protected value %q:\n%s", secret, data)
		}
	}

	reloaded := newHTTPClientStateWithVault(path, httpclient.NewVault(store))
	if reloaded.loadIssue != nil {
		t.Fatal(reloaded.loadIssue)
	}
	if got := reloaded.file.Environments[0].Variables["token"]; got != "environment-secret" {
		t.Fatalf("reloaded environment token=%q", got)
	}
	if got := reloaded.currentRequest().Auth.Token; got != "request-secret" {
		t.Fatalf("reloaded request token=%q", got)
	}
}
