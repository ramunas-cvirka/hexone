// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"fmt"
	"hexone/appdata"
	"hexone/httpclient"
	uitheme "hexone/ui/theme"
	"image"
	"image/color"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	httpDetailParams  = "params"
	httpDetailHeaders = "headers"
	httpDetailBody    = "body"
	httpDetailAuth    = "auth"

	httpResponsePretty  = "pretty"
	httpResponseRaw     = "raw"
	httpResponseHeaders = "headers"
)

var httpMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

type httpRequestRef struct {
	collection int
	folder     int
	request    int
}

type httpCollectionRow struct {
	kind              string
	label             string
	ref               httpRequestRef
	expanded          bool
	depth             int
	last              bool
	hasChildren       bool
	ancestorContinues []bool
}

type httpClientResult struct {
	response httpclient.Response
}

type httpSplitHandle struct {
	tag         struct{}
	dragging    bool
	hovering    bool
	pointerID   pointer.ID
	start       float32
	startRatio  float32
	startPixels int
}

type httpClientState struct {
	path        string
	file        *httpclient.File
	loadIssue   error
	environment int
	selected    httpRequestRef
	hasSelected bool
	openTabs    []httpRequestRef
	activeTab   int
	tabScroll   int
	tabClicks   []widget.Clickable
	tabCloses   []widget.Clickable
	tabPrev     widget.Clickable
	tabNext     widget.Clickable
	tabAdd      widget.Clickable
	tabGeometry appTabStripGeometry

	urlEd      widget.Editor
	queryEd    widget.Editor
	headersEd  widget.Editor
	bodyEd     widget.Editor
	authEd     widget.Editor
	responseEd widget.Editor

	detailMode   string
	responseMode string
	method       string
	dirty        bool
	status       string

	collectionList       widget.List
	detailScrollbar      widget.Scrollbar
	responseScrollbar    widget.Scrollbar
	collectionSplit      httpSplitHandle
	requestSplit         httpSplitHandle
	collectionWidth      int
	requestRatio         float32
	requestExtent        float32
	requestSplitY        int
	requestClicks        []widget.Clickable
	groupClicks          []widget.Clickable
	treeRenameEd         widget.Editor
	treeRenameActive     bool
	treeRenameFocus      bool
	treeRenameKind       string
	treeRenameRef        httpRequestRef
	treeRenameOriginal   string
	detailAddClick       widget.Clickable
	addCollectionClick   widget.Clickable
	addRequestClick      widget.Clickable
	treeSelected         httpCollectionRow
	hasTreeSelected      bool
	collapsedCollections map[int]bool
	collapsedFolders     map[[2]int]bool
	methodClick          widget.Clickable
	envClick             widget.Clickable
	addEnvironmentClick  widget.Clickable
	methodMenuOpen       bool
	envMenuOpen          bool
	methodMenuClicks     []widget.Clickable
	envMenuClicks        []widget.Clickable
	envCycleOrigin       int
	envEditorOpen        bool
	envEditorIndex       int
	envEditorNameEd      widget.Editor
	envEditorVarsEd      widget.Editor
	envEditorScrollbar   widget.Scrollbar
	envEditorFocus       string
	envEditorAddClick    widget.Clickable
	envEditorSaveClick   widget.Clickable
	envEditorCancelClick widget.Clickable
	menuDismissTag       byte
	menuSurfaceTag       byte
	treeMenuOpen         bool
	treeMenuRow          httpCollectionRow
	treeMenuRowIndex     int
	treeMenuRenameClick  widget.Clickable
	treeMenuDeleteClick  widget.Clickable
	treeMenuRunClicks    []widget.Clickable
	sendClick            widget.Clickable
	saveClick            widget.Clickable
	detailClicks         [4]widget.Clickable
	responseClicks       [3]widget.Clickable
	hoverID              string
	hoverAnim            segmentedAnimState

	sending  bool
	cancel   context.CancelFunc
	resultCh chan httpClientResult
	sender   httpclient.Sender
	response httpclient.Response
}

func resolveHTTPCollectionsPath() string {
	return appdata.HTTPCollectionsPath()
}

func newHTTPClientState(path string) *httpClientState {
	st := &httpClientState{
		path:                 path,
		detailMode:           httpDetailBody,
		responseMode:         httpResponsePretty,
		resultCh:             make(chan httpClientResult, 1),
		sender:               httpclient.Send,
		requestRatio:         0.42,
		collapsedCollections: make(map[int]bool),
		collapsedFolders:     make(map[[2]int]bool),
		collectionList: widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}
	st.urlEd.SingleLine = true
	st.urlEd.Submit = true
	st.treeRenameEd.SingleLine = true
	st.treeRenameEd.Submit = true
	st.envEditorNameEd.SingleLine = true
	st.envEditorNameEd.Submit = true
	st.responseEd.ReadOnly = true

	file, err := httpclient.LoadOrCreate(path)
	if err != nil {
		st.file = httpclient.DefaultFile()
		st.loadIssue = err
		st.status = "collection load error: " + err.Error()
	} else {
		st.file = file
		st.status = "loaded " + filepath.Base(path)
	}
	st.selectFirstRequest()
	return st
}

func (ui *UI) ensureHTTPClientState() *httpClientState {
	if ui == nil {
		return nil
	}
	if ui.httpState == nil {
		ui.httpState = newHTTPClientState(ui.httpCollectionsPath)
	}
	return ui.httpState
}

func (st *httpClientState) selectFirstRequest() {
	if st == nil || st.file == nil {
		return
	}
	for collectionIndex := range st.file.Collections {
		collection := &st.file.Collections[collectionIndex]
		if len(collection.Requests) > 0 {
			st.selectRequest(httpRequestRef{collection: collectionIndex, folder: -1, request: 0})
			return
		}
		for folderIndex := range collection.Folders {
			if len(collection.Folders[folderIndex].Requests) == 0 {
				continue
			}
			st.selectRequest(httpRequestRef{collection: collectionIndex, folder: folderIndex, request: 0})
			return
		}
	}
	st.addScratchRequest()
}

func (st *httpClientState) requestAt(ref httpRequestRef) *httpclient.Request {
	if st == nil || st.file == nil || ref.collection < 0 || ref.collection >= len(st.file.Collections) {
		return nil
	}
	collection := &st.file.Collections[ref.collection]
	if ref.folder < 0 {
		if ref.request < 0 || ref.request >= len(collection.Requests) {
			return nil
		}
		return &collection.Requests[ref.request]
	}
	if ref.folder >= len(collection.Folders) {
		return nil
	}
	folder := &collection.Folders[ref.folder]
	if ref.request < 0 || ref.request >= len(folder.Requests) {
		return nil
	}
	return &folder.Requests[ref.request]
}

func (st *httpClientState) currentRequest() *httpclient.Request {
	if st == nil || !st.hasSelected {
		return nil
	}
	return st.requestAt(st.selected)
}

func (st *httpClientState) selectRequest(ref httpRequestRef) {
	if st == nil || st.requestAt(ref) == nil {
		return
	}
	if st.hasSelected && st.selected == ref {
		st.treeSelected = httpCollectionRow{kind: "request", ref: ref}
		st.hasTreeSelected = true
		return
	}
	if st.hasSelected {
		st.applyEditorsToRequest()
	}
	st.selected = ref
	st.hasSelected = true
	st.treeSelected = httpCollectionRow{kind: "request", ref: ref}
	st.hasTreeSelected = true
	tabIndex := -1
	for index, open := range st.openTabs {
		if open == ref {
			tabIndex = index
			break
		}
	}
	if tabIndex < 0 {
		st.openTabs = append(st.openTabs, ref)
		tabIndex = len(st.openTabs) - 1
	}
	st.activeTab = tabIndex
	st.tabScroll = tabScrollToActive(st.tabScroll, st.activeTab)
	st.syncEditorsFromRequest()
}

func (st *httpClientState) activateRequestTab(index int) bool {
	if st == nil || index < 0 || index >= len(st.openTabs) || index == st.activeTab {
		return false
	}
	st.applyEditorsToRequest()
	st.activeTab = index
	st.selected = st.openTabs[index]
	st.hasSelected = true
	st.treeSelected = httpCollectionRow{kind: "request", ref: st.selected}
	st.hasTreeSelected = true
	st.tabScroll = tabScrollToActive(st.tabScroll, st.activeTab)
	st.syncEditorsFromRequest()
	return true
}

func (st *httpClientState) closeRequestTab(index int) bool {
	if st == nil || len(st.openTabs) <= 1 || index < 0 || index >= len(st.openTabs) {
		return false
	}
	st.applyEditorsToRequest()
	st.openTabs = append(st.openTabs[:index], st.openTabs[index+1:]...)
	if st.activeTab >= index {
		st.activeTab--
	}
	st.activeTab = clampTabIndex(st.activeTab, len(st.openTabs))
	st.tabScroll = tabScrollAfterClose(st.tabScroll, index, len(st.openTabs))
	st.selected = st.openTabs[st.activeTab]
	st.hasSelected = true
	st.treeSelected = httpCollectionRow{kind: "request", ref: st.selected}
	st.hasTreeSelected = true
	st.syncEditorsFromRequest()
	return true
}

func (st *httpClientState) addScratchRequest() bool {
	if st == nil || st.file == nil || len(st.openTabs) >= tabStripMaxTabsPerPane {
		return false
	}
	collectionIndex := -1
	for index := range st.file.Collections {
		collection := &st.file.Collections[index]
		if collection.ID == "scratch" || strings.EqualFold(collection.Name, "Scratch requests") {
			collectionIndex = index
			break
		}
	}
	if collectionIndex < 0 {
		st.file.Collections = append(st.file.Collections, httpclient.Collection{
			ID:   "scratch",
			Name: "Scratch requests",
		})
		collectionIndex = len(st.file.Collections) - 1
	}
	st.applyEditorsToRequest()
	collection := &st.file.Collections[collectionIndex]
	baseName := "New request"
	name := baseName
	for suffix := 2; requestNameExists(collection, name); suffix++ {
		name = baseName + " " + strconv.Itoa(suffix)
	}
	collection.Requests = append(collection.Requests, httpclient.Request{
		ID:     "request-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Name:   name,
		Method: "GET",
		URL:    "{{base_url}}/",
	})
	ref := httpRequestRef{collection: collectionIndex, folder: -1, request: len(collection.Requests) - 1}
	st.openTabs = append(st.openTabs, ref)
	st.activeTab = len(st.openTabs) - 1
	st.selected = ref
	st.hasSelected = true
	st.tabScroll = tabScrollToActive(st.tabScroll, st.activeTab)
	st.dirty = true
	st.status = "new request · save to keep it"
	st.syncEditorsFromRequest()
	st.dirty = true
	return true
}

func requestNameExists(collection *httpclient.Collection, name string) bool {
	if collection == nil {
		return false
	}
	for _, request := range collection.Requests {
		if request.Name == name {
			return true
		}
	}
	for _, folder := range collection.Folders {
		for _, request := range folder.Requests {
			if request.Name == name {
				return true
			}
		}
	}
	return false
}

func (st *httpClientState) syncEditorsFromRequest() {
	request := st.currentRequest()
	if request == nil {
		return
	}
	st.method = request.Method
	st.urlEd.SetText(request.URL)
	st.queryEd.SetText(formatKeyValueLines(request.Query, "="))
	st.headersEd.SetText(formatKeyValueLines(request.Headers, ": "))
	st.authEd.SetText(request.Auth)
	st.bodyEd.SetText(request.Body)
	if request.Body != "" {
		st.detailMode = httpDetailBody
	} else if request.Auth != "" {
		st.detailMode = httpDetailAuth
	} else {
		st.detailMode = httpDetailHeaders
	}
}

func (st *httpClientState) applyEditorsToRequest() {
	request := st.currentRequest()
	if request == nil {
		return
	}
	request.Method = strings.ToUpper(strings.TrimSpace(st.method))
	request.URL = strings.TrimSpace(st.urlEd.Text())
	request.Query = parseKeyValueLines(st.queryEd.Text(), "=")
	request.Headers = parseKeyValueLines(st.headersEd.Text(), ":")
	request.Auth = strings.TrimSpace(st.authEd.Text())
	request.Body = st.bodyEd.Text()
}

func (st *httpClientState) updateDirty() {
	request := st.currentRequest()
	if request == nil {
		st.dirty = false
		return
	}
	if request.Method != st.method ||
		request.URL != strings.TrimSpace(st.urlEd.Text()) ||
		formatKeyValueLines(request.Query, "=") != strings.TrimSpace(st.queryEd.Text()) ||
		formatKeyValueLines(request.Headers, ": ") != strings.TrimSpace(st.headersEd.Text()) ||
		request.Auth != strings.TrimSpace(st.authEd.Text()) ||
		request.Body != st.bodyEd.Text() {
		st.dirty = true
	}
}

func formatKeyValueLines(items []httpclient.KeyValue, separator string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		prefix := ""
		if item.Disabled {
			prefix = "# "
		}
		lines = append(lines, prefix+item.Name+separator+item.Value)
	}
	return strings.Join(lines, "\n")
}

func parseKeyValueLines(value, separator string) []httpclient.KeyValue {
	lines := strings.Split(value, "\n")
	items := make([]httpclient.KeyValue, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		disabled := strings.HasPrefix(line, "#")
		if disabled {
			line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
		parts := strings.SplitN(line, separator, 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		item := httpclient.KeyValue{Name: name, Disabled: disabled}
		if len(parts) == 2 {
			item.Value = strings.TrimSpace(parts[1])
		}
		items = append(items, item)
	}
	return items
}

func (st *httpClientState) environmentValue() httpclient.Environment {
	if st == nil || st.file == nil || len(st.file.Environments) == 0 {
		return httpclient.Environment{Variables: map[string]string{}}
	}
	if st.environment < 0 || st.environment >= len(st.file.Environments) {
		st.environment = 0
	}
	environment := st.file.Environments[st.environment]
	environment.Variables = cloneStringMapHTTP(environment.Variables)
	return environment
}

func cloneStringMapHTTP(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneHTTPRequest(request httpclient.Request) httpclient.Request {
	request.Query = append([]httpclient.KeyValue(nil), request.Query...)
	request.Headers = append([]httpclient.KeyValue(nil), request.Headers...)
	return request
}

func (ui *UI) saveHTTPCollections() error {
	st := ui.ensureHTTPClientState()
	if st == nil {
		return nil
	}
	if st.loadIssue != nil {
		return fmt.Errorf("refusing to overwrite a collection file that did not load cleanly: %w", st.loadIssue)
	}
	st.applyEditorsToRequest()
	if err := httpclient.Save(st.path, st.file); err != nil {
		st.status = "save failed: " + err.Error()
		return err
	}
	st.dirty = false
	st.status = "saved " + filepath.Base(st.path)
	return nil
}

func (ui *UI) startHTTPRequest() {
	st := ui.ensureHTTPClientState()
	if st == nil || st.sending {
		return
	}
	st.applyEditorsToRequest()
	request := st.currentRequest()
	if request == nil {
		st.status = "no request selected"
		return
	}
	requestCopy := cloneHTTPRequest(*request)
	environment := st.environmentValue()
	ctx, cancel := context.WithCancel(context.Background())
	st.cancel = cancel
	st.sending = true
	st.status = "sending " + requestCopy.Method + " " + httpclient.ExpandVariables(requestCopy.URL, environment.Variables)
	sender := st.sender
	if sender == nil {
		sender = httpclient.Send
	}
	go func() {
		response := sender(ctx, requestCopy, environment)
		select {
		case st.resultCh <- httpClientResult{response: response}:
		default:
		}
		if ui.invalidate != nil {
			ui.invalidate()
		}
	}()
}

func (st *httpClientState) pollResult() {
	if st == nil {
		return
	}
	select {
	case result := <-st.resultCh:
		st.response = result.response
		st.sending = false
		st.cancel = nil
		if result.response.Err != nil {
			st.status = "request failed: " + result.response.Err.Error()
		} else {
			st.status = fmt.Sprintf("%s · %s · %s", result.response.Status, formatHTTPDuration(result.response.Duration), formatHTTPBytes(result.response.Size))
		}
		st.updateResponseEditor()
	default:
	}
}

func (st *httpClientState) updateResponseEditor() {
	if st == nil {
		return
	}
	if st.response.Err != nil {
		st.responseEd.SetText(st.response.Err.Error())
		return
	}
	switch st.responseMode {
	case httpResponseRaw:
		st.responseEd.SetText(string(st.response.Body))
	case httpResponseHeaders:
		st.responseEd.SetText(formatKeyValueLines(st.response.Headers, ": "))
	default:
		body := httpclient.PrettyBody(st.response.Body)
		if st.response.Truncated {
			body += "\n\n[ response truncated at 4 MiB ]"
		}
		st.responseEd.SetText(body)
	}
}

func formatHTTPDuration(duration time.Duration) string {
	if duration < time.Millisecond {
		return duration.Round(time.Microsecond).String()
	}
	return duration.Round(time.Millisecond).String()
}

func formatHTTPBytes(size int64) string {
	switch {
	case size < 1024:
		return strconv.FormatInt(size, 10) + " B"
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(size)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(size)/(1024*1024))
	}
}

func (ui *UI) handleHTTPClientKeys(gtx layout.Context) {
	if ui == nil || ui.Tabs.Value != "tab3" || ui.helpModal != nil || ui.settingsModal != nil || ui.sshModal != nil {
		return
	}
	if ui.httpState != nil && ui.httpState.envEditorOpen {
		return
	}
	anyMods := ^key.Modifiers(0)
	for {
		eventValue, ok := gtx.Event(
			key.Filter{Name: key.NameEnter, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: key.NameReturn, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: key.NameEnter, Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: key.NameReturn, Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "s", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "S", Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Name: "s", Required: key.ModShortcut, Optional: anyMods},
			key.Filter{Name: "S", Required: key.ModShortcut, Optional: anyMods},
		)
		if !ok {
			return
		}
		keyEvent, ok := eventValue.(key.Event)
		if !ok || keyEvent.State != key.Press {
			continue
		}
		switch keyEvent.Name {
		case key.NameEnter, key.NameReturn:
			ui.startHTTPRequest()
		case "s", "S":
			_ = ui.saveHTTPCollections()
		}
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (ui *UI) layoutTab3(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.ensureHTTPClientState()
	if st == nil {
		return layout.Dimensions{}
	}
	st.pollResult()
	ui.handleHTTPURLSubmit(gtx, st)
	ui.handleHTTPTreeRename(gtx, st)
	ui.handleHTTPEnvironmentEditor(gtx, st)
	ui.handleHTTPClientClicks(gtx, st)
	st.updateDirty()
	updateHTTPHoverAnimation(gtx, st)

	return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layoutHTTPOuterSurface(gtx, analyzerSurfaceBg, analyzerRule, func(gtx layout.Context) layout.Dimensions {
			totalWidth := gtx.Constraints.Max.X
			collectionWidth := httpCollectionSplitWidth(gtx, totalWidth, st.collectionWidth)
			st.collectionWidth = collectionWidth
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = collectionWidth
							gtx.Constraints.Max.X = collectionWidth
							return ui.layoutHTTPCollections(th, gtx, st)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.layoutHTTPWorkbenchDivider(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if st.envEditorOpen {
								return ui.layoutHTTPEnvironmentEditor(th, gtx, st)
							}
							return ui.layoutHTTPWorkbench(th, gtx, st)
						}),
					)
				}),
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					halfHit := max(3, gtx.Dp(unit.Dp(4)))
					hit := image.Rect(collectionWidth-halfHit, 0, collectionWidth+halfHit+1, gtx.Constraints.Max.Y)
					minWidth, maxWidth := httpCollectionSplitLimits(gtx, totalWidth)
					layoutHTTPSplitPixelsOverlay(gtx, &st.collectionSplit, layout.Horizontal, hit, &st.collectionWidth, minWidth, maxWidth)
					return layout.Dimensions{Size: gtx.Constraints.Max}
				}),
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					registerHTTPMenuDismissGlobal(gtx, st)
					return layout.Dimensions{Size: gtx.Constraints.Max}
				}),
			)
		})
	})
}

func updateHTTPHoverAnimation(gtx layout.Context, st *httpClientState) {
	if st == nil {
		return
	}
	hoverID := ""
	candidates := []struct {
		id    string
		click *widget.Clickable
	}{
		{"environment", &st.envClick}, {"environment-add", &st.addEnvironmentClick}, {"collection-add", &st.addCollectionClick},
		{"request-add", &st.addRequestClick},
		{"method", &st.methodClick}, {"send", &st.sendClick}, {"save", &st.saveClick},
		{"detail-add", &st.detailAddClick},
		{"env-editor-cancel", &st.envEditorCancelClick}, {"env-editor-save", &st.envEditorSaveClick},
	}
	for index := range st.detailClicks {
		candidates = append(candidates, struct {
			id    string
			click *widget.Clickable
		}{"detail-" + strconv.Itoa(index), &st.detailClicks[index]})
	}
	for index := range st.responseClicks {
		candidates = append(candidates, struct {
			id    string
			click *widget.Clickable
		}{"response-" + strconv.Itoa(index), &st.responseClicks[index]})
	}
	for _, candidate := range candidates {
		if candidate.click.Hovered() {
			hoverID = candidate.id
			break
		}
	}
	if hoverID != st.hoverID {
		st.hoverID = hoverID
		st.hoverAnim.setHover(hoverID, gtx.Now)
		gtx.Execute(op.InvalidateCmd{})
	}
}

func httpHoverFill(gtx layout.Context, st *httpClientState, id string) float32 {
	fill, animating := st.hoverAnim.hoverFill(gtx.Now, id)
	if animating {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	return fill
}

func (ui *UI) layoutHTTPWorkbenchDivider(gtx layout.Context) layout.Dimensions {
	w := max(1, gtx.Dp(unit.Dp(1)))
	h := gtx.Constraints.Max.Y
	lineX := (w - 1) / 2
	startY := httpWireLineY(gtx, ui.tabStripHeight(gtx))
	if startY < h {
		paint.FillShape(gtx.Ops, analyzerRule, clip.Rect(image.Rect(0, startY, w, startY+1)).Op())
	}
	verticalStart := startY + 1
	if h > verticalStart {
		paint.FillShape(gtx.Ops, analyzerRule, clip.Rect(image.Rect(lineX, verticalStart, lineX+1, h)).Op())
	}
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (ui *UI) handleHTTPURLSubmit(gtx layout.Context, st *httpClientState) {
	if ui == nil || st == nil {
		return
	}
	for {
		ev, ok := st.urlEd.Update(gtx)
		if !ok {
			return
		}
		if submit, ok := ev.(widget.SubmitEvent); ok {
			st.urlEd.SetText(submit.Text)
			ui.startHTTPRequest()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func httpCollectionSplitLimits(gtx layout.Context, total int) (int, int) {
	minWidth := gtx.Dp(unit.Dp(160))
	maxWidth := min(int(float32(total)*0.55), total-gtx.Dp(unit.Dp(360)))
	if maxWidth < minWidth {
		maxWidth = max(1, total/2)
		minWidth = min(minWidth, maxWidth)
	}
	return minWidth, maxWidth
}

func httpCollectionSplitWidth(gtx layout.Context, total, requested int) int {
	if total <= 1 {
		return 0
	}
	if requested <= 0 {
		requested = total * 23 / 100
		requested = max(gtx.Dp(unit.Dp(220)), min(gtx.Dp(unit.Dp(300)), requested))
	}
	minWidth, maxWidth := httpCollectionSplitLimits(gtx, total)
	return max(minWidth, min(maxWidth, requested))
}

func (ui *UI) handleHTTPClientClicks(gtx layout.Context, st *httpClientState) {
	handleHTTPChoiceMenuDismissPresses(gtx, st)
	handleHTTPSelectorSecondaryPresses(gtx, st)
	ensureHTTPChoiceClicks(&st.methodMenuClicks, len(httpMethods))
	environmentCount := 0
	if st.file != nil {
		environmentCount = len(st.file.Environments)
	}
	ensureHTTPChoiceClicks(&st.envMenuClicks, environmentCount)
	ensureHTTPChoiceClicks(&st.treeMenuRunClicks, environmentCount)
	for st.treeMenuRenameClick.Clicked(gtx) {
		row := st.treeMenuRow
		st.treeMenuOpen = false
		st.beginTreeRename(row)
	}
	for index := range st.treeMenuRunClicks {
		for st.treeMenuRunClicks[index].Clicked(gtx) {
			row := st.treeMenuRow
			st.treeMenuOpen = false
			if row.kind == "request" && st.requestAt(row.ref) != nil {
				st.environment = index
				st.selectRequest(row.ref)
				ui.startHTTPRequest()
			}
		}
	}
	for st.treeMenuDeleteClick.Clicked(gtx) {
		row := st.treeMenuRow
		st.treeMenuOpen = false
		if st.deleteTreeRow(row) {
			_ = ui.saveHTTPCollections()
		}
	}
	for index := range st.methodMenuClicks {
		for st.methodMenuClicks[index].Clicked(gtx) {
			st.method = httpMethods[index]
			st.methodMenuOpen = false
		}
	}
	for index := range st.envMenuClicks {
		for st.envMenuClicks[index].Clicked(gtx) {
			st.closeEnvironmentEditor()
			st.environment = index
			st.envMenuOpen = false
		}
	}
	for st.addEnvironmentClick.Clicked(gtx) {
		if st.addEnvironment() {
			_ = ui.saveHTTPCollections()
			st.openEnvironmentEditor(st.environment, true)
		}
	}
	for st.addCollectionClick.Clicked(gtx) {
		if st.addCollection() {
			st.beginTreeRename(st.treeSelected)
			_ = ui.saveHTTPCollections()
		}
	}
	for st.addRequestClick.Clicked(gtx) {
		if st.addRequestToSelection() {
			st.beginTreeRename(st.treeSelected)
			_ = ui.saveHTTPCollections()
		}
	}
	for st.methodClick.Clicked(gtx) {
		st.methodMenuOpen = false
		index := 0
		for i, method := range httpMethods {
			if method == st.method {
				index = i
				break
			}
		}
		st.method = httpMethods[(index+1)%len(httpMethods)]
	}
	for {
		click, ok := st.envClick.Update(gtx)
		if !ok {
			break
		}
		st.envMenuOpen = false
		if st.file == nil || len(st.file.Environments) == 0 {
			continue
		}
		if click.NumClicks >= 2 {
			st.environment = st.envCycleOrigin
			st.openEnvironmentEditor(st.environment, true)
			continue
		}
		st.closeEnvironmentEditor()
		st.envCycleOrigin = st.environment
		st.environment = (st.environment + 1) % len(st.file.Environments)
	}
	for st.detailAddClick.Clicked(gtx) {
		st.closeHTTPChoiceMenus()
		var editor *widget.Editor
		line := ""
		switch st.detailMode {
		case httpDetailParams:
			editor, line = &st.queryEd, "key=value"
		case httpDetailHeaders:
			editor, line = &st.headersEd, "Header: value"
		case httpDetailAuth:
			editor, line = &st.authEd, "Bearer {{token}}"
		case httpDetailBody:
			st.detailMode = httpDetailParams
			editor, line = &st.queryEd, "key=value"
		}
		if editor != nil {
			appendHTTPEditorLine(editor, line)
			gtx.Execute(key.FocusCmd{Tag: editor})
		}
	}
	for st.sendClick.Clicked(gtx) {
		st.closeHTTPChoiceMenus()
		ui.startHTTPRequest()
	}
	for st.saveClick.Clicked(gtx) {
		st.closeHTTPChoiceMenus()
		_ = ui.saveHTTPCollections()
	}
	detailModes := []string{httpDetailParams, httpDetailHeaders, httpDetailAuth, httpDetailBody}
	for index := range st.detailClicks {
		for st.detailClicks[index].Clicked(gtx) {
			st.closeHTTPChoiceMenus()
			st.detailMode = detailModes[index]
		}
	}
	responseModes := []string{httpResponsePretty, httpResponseRaw, httpResponseHeaders}
	for index := range st.responseClicks {
		for st.responseClicks[index].Clicked(gtx) {
			st.closeHTTPChoiceMenus()
			st.responseMode = responseModes[index]
			st.updateResponseEditor()
		}
	}

	rows := st.collectionRows()
	st.ensureRequestClicks(len(rows))
	st.ensureGroupClicks(len(rows))
	ui.handleHTTPTreeSecondaryPresses(gtx, st, rows)
	for index := range rows {
		if rows[index].kind == "request" {
			for {
				click, ok := st.requestClicks[index].Update(gtx)
				if !ok {
					break
				}
				st.closeHTTPChoiceMenus()
				if st.selectTreeRequest(rows[index]) {
					gtx.Execute(key.FocusCmd{})
				}
				if click.NumClicks >= 2 {
					st.beginTreeRename(rows[index])
				}
			}
		} else {
			for {
				click, ok := st.groupClicks[index].Update(gtx)
				if !ok {
					break
				}
				st.closeHTTPChoiceMenus()
				if st.envEditorOpen {
					st.closeEnvironmentEditor()
					gtx.Execute(key.FocusCmd{})
				}
				if st.cancelTreeRenameForSelection(rows[index]) {
					gtx.Execute(key.FocusCmd{})
				}
				if (rows[index].kind == "collection" || rows[index].kind == "folder") && click.NumClicks >= 2 {
					// The first click in the pair toggled the group; restore its
					// prior expansion while the double-click enters rename mode.
					if rows[index].kind == "collection" {
						collection := rows[index].ref.collection
						st.collapsedCollections[collection] = !st.collapsedCollections[collection]
					} else {
						folder := [2]int{rows[index].ref.collection, rows[index].ref.folder}
						st.collapsedFolders[folder] = !st.collapsedFolders[folder]
					}
					st.beginTreeRename(rows[index])
					continue
				}
				st.selectTreeGroup(rows[index])
			}
		}
	}
}

func (st *httpClientState) closeHTTPChoiceMenus() {
	if st == nil {
		return
	}
	st.methodMenuOpen = false
	st.envMenuOpen = false
	st.treeMenuOpen = false
}

func handleHTTPChoiceMenuDismissPresses(gtx layout.Context, st *httpClientState) {
	if st == nil {
		return
	}
	insideMenu := false
	for {
		eventValue, ok := gtx.Event(pointer.Filter{Target: &st.menuSurfaceTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pointerEvent, ok := eventValue.(pointer.Event); ok && pointerEvent.Buttons != 0 {
			insideMenu = true
		}
	}
	for {
		eventValue, ok := gtx.Event(pointer.Filter{Target: &st.menuDismissTag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if pointerEvent, ok := eventValue.(pointer.Event); ok && pointerEvent.Buttons != 0 && !insideMenu {
			st.closeHTTPChoiceMenus()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func registerHTTPMenuSurface(gtx layout.Context, st *httpClientState, size image.Point) {
	if st == nil || size.X <= 0 || size.Y <= 0 {
		return
	}
	stack := clip.Rect{Max: size}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &st.menuSurfaceTag)
	pass.Pop()
	stack.Pop()
}

func registerHTTPMenuDismissGlobal(gtx layout.Context, st *httpClientState) {
	if st == nil || (!st.methodMenuOpen && !st.envMenuOpen && !st.treeMenuOpen) {
		return
	}
	stack := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &st.menuDismissTag)
	pass.Pop()
	stack.Pop()
}

func (ui *UI) handleHTTPTreeSecondaryPresses(gtx layout.Context, st *httpClientState, rows []httpCollectionRow) {
	if st == nil {
		return
	}
	for index, row := range rows {
		var target *widget.Clickable
		if row.kind == "request" {
			if index >= len(st.requestClicks) {
				continue
			}
			target = &st.requestClicks[index]
		} else {
			if index >= len(st.groupClicks) {
				continue
			}
			target = &st.groupClicks[index]
		}
		for {
			eventValue, ok := gtx.Event(pointer.Filter{Target: target, Kinds: pointer.Press})
			if !ok {
				break
			}
			pointerEvent, ok := eventValue.(pointer.Event)
			if !ok || !pointerEvent.Buttons.Contain(pointer.ButtonSecondary) {
				continue
			}
			st.closeHTTPChoiceMenus()
			st.closeEnvironmentEditor()
			st.finishTreeRename(false)
			if row.kind == "request" {
				st.selectRequest(row.ref)
			} else {
				st.treeSelected = row
				st.hasTreeSelected = true
			}
			st.treeMenuRow = row
			st.treeMenuRowIndex = index
			st.treeMenuOpen = true
			gtx.Execute(key.FocusCmd{})
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (ui *UI) handleHTTPTreeRename(gtx layout.Context, st *httpClientState) {
	if st == nil || !st.treeRenameActive {
		return
	}
	for {
		eventValue, ok := gtx.Event(key.Filter{Focus: &st.treeRenameEd, Name: key.NameEscape})
		if !ok {
			break
		}
		keyEvent, ok := eventValue.(key.Event)
		if ok && keyEvent.State == key.Press {
			st.finishTreeRename(false)
			gtx.Execute(op.InvalidateCmd{})
			return
		}
	}
	for {
		eventValue, ok := st.treeRenameEd.Update(gtx)
		if !ok {
			break
		}
		submit, ok := eventValue.(widget.SubmitEvent)
		if !ok {
			continue
		}
		st.treeRenameEd.SetText(submit.Text)
		changed := st.finishTreeRename(true)
		if changed {
			_ = ui.saveHTTPCollections()
		}
		gtx.Execute(op.InvalidateCmd{})
		return
	}
}

func (st *httpClientState) selectTreeRequest(row httpCollectionRow) bool {
	if st == nil || row.kind != "request" {
		return false
	}
	cancelledRename := st.cancelTreeRenameForSelection(row)
	if st.envEditorOpen {
		st.closeEnvironmentEditor()
		cancelledRename = true
	}
	st.selectRequest(row.ref)
	return cancelledRename
}

func (st *httpClientState) cancelTreeRenameForSelection(row httpCollectionRow) bool {
	if st == nil || !st.treeRenameActive || st.treeRenameMatches(row) {
		return false
	}
	st.finishTreeRename(false)
	return true
}

func handleHTTPSelectorSecondaryPresses(gtx layout.Context, st *httpClientState) {
	if st == nil {
		return
	}
	selectors := []struct {
		click *widget.Clickable
		open  *bool
	}{
		{click: &st.methodClick, open: &st.methodMenuOpen},
		{click: &st.envClick, open: &st.envMenuOpen},
	}
	for _, selector := range selectors {
		for {
			eventValue, ok := gtx.Event(pointer.Filter{Target: selector.click, Kinds: pointer.Press})
			if !ok {
				break
			}
			pointerEvent, ok := eventValue.(pointer.Event)
			if !ok || !pointerEvent.Buttons.Contain(pointer.ButtonSecondary) {
				continue
			}
			wasOpen := *selector.open
			st.closeHTTPChoiceMenus()
			*selector.open = !wasOpen
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func ensureHTTPChoiceClicks(clicks *[]widget.Clickable, count int) {
	if count < 0 {
		count = 0
	}
	if len(*clicks) != count {
		*clicks = make([]widget.Clickable, count)
	}
}

func (st *httpClientState) collectionRows() []httpCollectionRow {
	if st == nil || st.file == nil {
		return nil
	}
	rows := make([]httpCollectionRow, 0)
	for collectionIndex, collection := range st.file.Collections {
		collectionCollapsed := st.collapsedCollections[collectionIndex]
		collectionLast := collectionIndex == len(st.file.Collections)-1
		rootChildCount := len(collection.Requests) + len(collection.Folders)
		rows = append(rows, httpCollectionRow{
			kind:        "collection",
			label:       collection.Name,
			ref:         httpRequestRef{collection: collectionIndex, folder: -1, request: -1},
			expanded:    rootChildCount > 0 && !collectionCollapsed,
			depth:       0,
			last:        collectionLast,
			hasChildren: rootChildCount > 0,
		})
		if collectionCollapsed {
			continue
		}
		rootChildIndex := 0
		for requestIndex, request := range collection.Requests {
			childLast := rootChildIndex == rootChildCount-1
			rows = append(rows, httpCollectionRow{
				kind:              "request",
				label:             request.Name,
				ref:               httpRequestRef{collection: collectionIndex, folder: -1, request: requestIndex},
				depth:             1,
				last:              childLast,
				ancestorContinues: []bool{!collectionLast},
			})
			rootChildIndex++
		}
		for folderIndex, folder := range collection.Folders {
			folderCollapsed := st.collapsedFolders[[2]int{collectionIndex, folderIndex}]
			folderLast := rootChildIndex == rootChildCount-1
			rows = append(rows, httpCollectionRow{
				kind:              "folder",
				label:             folder.Name,
				ref:               httpRequestRef{collection: collectionIndex, folder: folderIndex, request: -1},
				expanded:          len(folder.Requests) > 0 && !folderCollapsed,
				depth:             1,
				last:              folderLast,
				hasChildren:       len(folder.Requests) > 0,
				ancestorContinues: []bool{!collectionLast},
			})
			if folderCollapsed {
				rootChildIndex++
				continue
			}
			for requestIndex, request := range folder.Requests {
				rows = append(rows, httpCollectionRow{
					kind:              "request",
					label:             request.Name,
					ref:               httpRequestRef{collection: collectionIndex, folder: folderIndex, request: requestIndex},
					depth:             2,
					last:              requestIndex == len(folder.Requests)-1,
					ancestorContinues: []bool{!collectionLast, !folderLast},
				})
			}
			rootChildIndex++
		}
	}
	return rows
}

func (st *httpClientState) ensureRequestClicks(count int) {
	if count <= cap(st.requestClicks) {
		st.requestClicks = st.requestClicks[:count]
		return
	}
	clicks := make([]widget.Clickable, count)
	copy(clicks, st.requestClicks)
	st.requestClicks = clicks
}

func (st *httpClientState) ensureGroupClicks(count int) {
	if count <= cap(st.groupClicks) {
		st.groupClicks = st.groupClicks[:count]
		return
	}
	clicks := make([]widget.Clickable, count)
	copy(clicks, st.groupClicks)
	st.groupClicks = clicks
}

func (st *httpClientState) selectTreeGroup(row httpCollectionRow) {
	if st == nil || (row.kind != "collection" && row.kind != "folder") {
		return
	}
	st.treeSelected = row
	st.hasTreeSelected = true
	if row.kind == "collection" {
		st.collapsedCollections[row.ref.collection] = !st.collapsedCollections[row.ref.collection]
		return
	}
	key := [2]int{row.ref.collection, row.ref.folder}
	st.collapsedFolders[key] = !st.collapsedFolders[key]
}

func (st *httpClientState) beginTreeRename(row httpCollectionRow) bool {
	if st == nil || (row.kind != "collection" && row.kind != "folder" && row.kind != "request") {
		return false
	}
	name, ok := st.treeRenameName(row.kind, row.ref)
	if !ok {
		return false
	}
	if st.treeRenameActive {
		st.finishTreeRename(true)
	}
	st.treeRenameKind = row.kind
	st.treeRenameRef = row.ref
	st.treeRenameOriginal = name
	st.treeRenameEd.SetText(name)
	st.treeRenameEd.SetCaret(0, st.treeRenameEd.Len())
	st.treeRenameActive = true
	st.treeRenameFocus = true
	return true
}

func (st *httpClientState) finishTreeRename(commit bool) bool {
	if st == nil || !st.treeRenameActive {
		return false
	}
	changed := false
	if commit {
		name := strings.TrimSpace(st.treeRenameEd.Text())
		if name == "" {
			name = st.treeRenameOriginal
		}
		changed = st.setTreeRenameName(st.treeRenameKind, st.treeRenameRef, name)
		if changed {
			st.dirty = true
			st.status = "renamed to " + name
		}
	}
	st.treeRenameActive = false
	st.treeRenameFocus = false
	st.treeRenameKind = ""
	st.treeRenameRef = httpRequestRef{}
	st.treeRenameOriginal = ""
	return changed
}

func (st *httpClientState) treeRenameName(kind string, ref httpRequestRef) (string, bool) {
	if st == nil || st.file == nil {
		return "", false
	}
	switch kind {
	case "collection":
		if ref.collection < 0 || ref.collection >= len(st.file.Collections) {
			return "", false
		}
		return st.file.Collections[ref.collection].Name, true
	case "folder":
		if ref.collection < 0 || ref.collection >= len(st.file.Collections) || ref.folder < 0 || ref.folder >= len(st.file.Collections[ref.collection].Folders) {
			return "", false
		}
		return st.file.Collections[ref.collection].Folders[ref.folder].Name, true
	case "request":
		request := st.requestAt(ref)
		if request == nil {
			return "", false
		}
		return request.Name, true
	default:
		return "", false
	}
}

func (st *httpClientState) setTreeRenameName(kind string, ref httpRequestRef, name string) bool {
	current, ok := st.treeRenameName(kind, ref)
	if !ok || current == name {
		return false
	}
	switch kind {
	case "collection":
		st.file.Collections[ref.collection].Name = name
	case "folder":
		st.file.Collections[ref.collection].Folders[ref.folder].Name = name
	case "request":
		st.requestAt(ref).Name = name
	}
	if st.hasTreeSelected && st.treeSelected.kind == kind && st.treeSelected.ref == ref {
		st.treeSelected.label = name
	}
	return true
}

func (st *httpClientState) treeRenameMatches(row httpCollectionRow) bool {
	return st != nil && st.treeRenameActive && st.treeRenameKind == row.kind && st.treeRenameRef == row.ref
}

func (st *httpClientState) deleteTreeRow(row httpCollectionRow) bool {
	if st == nil || st.file == nil || row.ref.collection < 0 || row.ref.collection >= len(st.file.Collections) {
		return false
	}
	st.applyEditorsToRequest()
	deletedName := row.label
	collection := &st.file.Collections[row.ref.collection]
	switch row.kind {
	case "collection":
		deletedName = collection.Name
		st.file.Collections = append(st.file.Collections[:row.ref.collection], st.file.Collections[row.ref.collection+1:]...)
	case "folder":
		if row.ref.folder < 0 || row.ref.folder >= len(collection.Folders) {
			return false
		}
		deletedName = collection.Folders[row.ref.folder].Name
		collection.Folders = append(collection.Folders[:row.ref.folder], collection.Folders[row.ref.folder+1:]...)
	case "request":
		request := st.requestAt(row.ref)
		if request == nil {
			return false
		}
		deletedName = request.Name
		if row.ref.folder < 0 {
			collection.Requests = append(collection.Requests[:row.ref.request], collection.Requests[row.ref.request+1:]...)
		} else {
			folder := &collection.Folders[row.ref.folder]
			folder.Requests = append(folder.Requests[:row.ref.request], folder.Requests[row.ref.request+1:]...)
		}
	default:
		return false
	}
	st.openTabs = nil
	st.activeTab = 0
	st.tabScroll = 0
	st.hasSelected = false
	st.hasTreeSelected = false
	st.collapsedCollections = make(map[int]bool)
	st.collapsedFolders = make(map[[2]int]bool)
	st.selectFirstRequest()
	st.dirty = true
	st.status = "deleted " + deletedName
	return true
}

func (st *httpClientState) selectedTreeTarget() (collection, folder int) {
	if st == nil || st.file == nil || len(st.file.Collections) == 0 {
		return -1, -1
	}
	if st.hasTreeSelected {
		collection = st.treeSelected.ref.collection
		folder = st.treeSelected.ref.folder
		if collection >= 0 && collection < len(st.file.Collections) {
			if folder >= 0 && folder < len(st.file.Collections[collection].Folders) {
				return collection, folder
			}
			return collection, -1
		}
	}
	if st.hasSelected && st.selected.collection >= 0 && st.selected.collection < len(st.file.Collections) {
		return st.selected.collection, st.selected.folder
	}
	return 0, -1
}

func uniqueHTTPCollectionName(collections []httpclient.Collection, base string) string {
	name := base
	for suffix := 2; ; suffix++ {
		exists := false
		for _, collection := range collections {
			if strings.EqualFold(collection.Name, name) {
				exists = true
				break
			}
		}
		if !exists {
			return name
		}
		name = base + " " + strconv.Itoa(suffix)
	}
}

func uniqueHTTPFolderName(folders []httpclient.Folder, base string) string {
	name := base
	for suffix := 2; ; suffix++ {
		exists := false
		for _, folder := range folders {
			if strings.EqualFold(folder.Name, name) {
				exists = true
				break
			}
		}
		if !exists {
			return name
		}
		name = base + " " + strconv.Itoa(suffix)
	}
}

func uniqueHTTPRequestName(requests []httpclient.Request, base string) string {
	name := base
	for suffix := 2; ; suffix++ {
		exists := false
		for _, request := range requests {
			if strings.EqualFold(request.Name, name) {
				exists = true
				break
			}
		}
		if !exists {
			return name
		}
		name = base + " " + strconv.Itoa(suffix)
	}
}

func (st *httpClientState) addCollection() bool {
	if st == nil || st.file == nil {
		return false
	}
	st.applyEditorsToRequest()
	name := uniqueHTTPCollectionName(st.file.Collections, "New collection")
	st.file.Collections = append(st.file.Collections, httpclient.Collection{
		ID:   "collection-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Name: name,
	})
	index := len(st.file.Collections) - 1
	st.treeSelected = httpCollectionRow{kind: "collection", label: name, ref: httpRequestRef{collection: index, folder: -1, request: -1}}
	st.hasTreeSelected = true
	st.dirty = true
	st.status = "added " + name
	return true
}

func (st *httpClientState) addEnvironment() bool {
	if st == nil || st.file == nil {
		return false
	}
	name := "environment"
	for suffix := 2; ; suffix++ {
		exists := false
		for _, environment := range st.file.Environments {
			if strings.EqualFold(environment.Name, name) {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		name = "environment-" + strconv.Itoa(suffix)
	}
	st.file.Environments = append(st.file.Environments, httpclient.Environment{Name: name, Variables: map[string]string{}})
	st.environment = len(st.file.Environments) - 1
	st.dirty = true
	st.status = "added " + name
	return true
}

func (st *httpClientState) openEnvironmentEditor(index int, focusName bool) bool {
	if st == nil || st.file == nil || index < 0 || index >= len(st.file.Environments) {
		return false
	}
	st.finishTreeRename(false)
	environment := st.file.Environments[index]
	st.environment = index
	st.envEditorIndex = index
	st.envEditorNameEd.SetText(environment.Name)
	st.envEditorVarsEd.SetText(formatHTTPEnvironmentVariables(environment.Variables))
	st.envEditorOpen = true
	st.envMenuOpen = false
	st.methodMenuOpen = false
	st.envEditorFocus = "variables"
	if focusName {
		st.envEditorNameEd.SetCaret(0, st.envEditorNameEd.Len())
		st.envEditorFocus = "name"
	}
	return true
}

func (st *httpClientState) closeEnvironmentEditor() {
	if st == nil {
		return
	}
	st.envEditorOpen = false
	st.envEditorFocus = ""
}

func (ui *UI) saveEnvironmentEditor(st *httpClientState) bool {
	if st == nil || !st.envEditorOpen || st.file == nil || st.envEditorIndex < 0 || st.envEditorIndex >= len(st.file.Environments) {
		return false
	}
	environment := &st.file.Environments[st.envEditorIndex]
	name := strings.TrimSpace(st.envEditorNameEd.Text())
	if name == "" {
		name = environment.Name
	}
	variables := parseHTTPEnvironmentVariables(st.envEditorVarsEd.Text())
	changed := environment.Name != name || formatHTTPEnvironmentVariables(environment.Variables) != formatHTTPEnvironmentVariables(variables)
	environment.Name = name
	environment.Variables = variables
	st.environment = st.envEditorIndex
	st.closeEnvironmentEditor()
	if changed {
		st.dirty = true
		st.status = "saved environment " + name
		_ = ui.saveHTTPCollections()
	}
	return changed
}

func formatHTTPEnvironmentVariables(variables map[string]string) string {
	if len(variables) == 0 {
		return ""
	}
	keys := make([]string, 0, len(variables))
	for key := range variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+variables[key])
	}
	return strings.Join(lines, "\n")
}

func parseHTTPEnvironmentVariables(text string) map[string]string {
	variables := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		variables[key] = strings.TrimSpace(value)
	}
	return variables
}

func (ui *UI) handleHTTPEnvironmentEditor(gtx layout.Context, st *httpClientState) {
	if st == nil || !st.envEditorOpen {
		return
	}
	for st.envEditorAddClick.Clicked(gtx) {
		appendHTTPEditorLine(&st.envEditorVarsEd, "key=value")
		st.envEditorFocus = "variables"
		gtx.Execute(op.InvalidateCmd{})
	}
	for st.envEditorSaveClick.Clicked(gtx) {
		ui.saveEnvironmentEditor(st)
		gtx.Execute(key.FocusCmd{})
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	for st.envEditorCancelClick.Clicked(gtx) {
		st.closeEnvironmentEditor()
		gtx.Execute(key.FocusCmd{})
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	for {
		eventValue, ok := st.envEditorNameEd.Update(gtx)
		if !ok {
			break
		}
		if _, ok := eventValue.(widget.SubmitEvent); ok {
			st.envEditorFocus = "variables"
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	for {
		if _, ok := st.envEditorVarsEd.Update(gtx); !ok {
			break
		}
	}
	anyMods := ^key.Modifiers(0)
	for {
		eventValue, ok := gtx.Event(
			key.Filter{Focus: &st.envEditorNameEd, Name: key.NameEscape, Optional: anyMods},
			key.Filter{Focus: &st.envEditorVarsEd, Name: key.NameEscape, Optional: anyMods},
			key.Filter{Focus: &st.envEditorNameEd, Name: key.NameEnter, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Focus: &st.envEditorVarsEd, Name: key.NameEnter, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Focus: &st.envEditorNameEd, Name: key.NameReturn, Required: key.ModCtrl, Optional: anyMods},
			key.Filter{Focus: &st.envEditorVarsEd, Name: key.NameReturn, Required: key.ModCtrl, Optional: anyMods},
		)
		if !ok {
			break
		}
		keyEvent, ok := eventValue.(key.Event)
		if !ok || keyEvent.State != key.Press {
			continue
		}
		if keyEvent.Name == key.NameEscape {
			st.closeEnvironmentEditor()
		} else {
			ui.saveEnvironmentEditor(st)
		}
		gtx.Execute(key.FocusCmd{})
		gtx.Execute(op.InvalidateCmd{})
		return
	}
}

func appendHTTPEditorLine(editor *widget.Editor, line string) {
	if editor == nil || line == "" {
		return
	}
	text := strings.TrimRight(editor.Text(), "\n")
	if text != "" {
		text += "\n"
	}
	editor.SetText(text + line)
}

func (st *httpClientState) addFolderToSelection() bool {
	collectionIndex, _ := st.selectedTreeTarget()
	if collectionIndex < 0 {
		return false
	}
	st.applyEditorsToRequest()
	collection := &st.file.Collections[collectionIndex]
	name := uniqueHTTPFolderName(collection.Folders, "New folder")
	collection.Folders = append(collection.Folders, httpclient.Folder{Name: name})
	folderIndex := len(collection.Folders) - 1
	delete(st.collapsedCollections, collectionIndex)
	st.treeSelected = httpCollectionRow{kind: "folder", label: name, ref: httpRequestRef{collection: collectionIndex, folder: folderIndex, request: -1}}
	st.hasTreeSelected = true
	st.dirty = true
	st.status = "added " + name
	return true
}

func (st *httpClientState) addRequestToSelection() bool {
	collectionIndex, folderIndex := st.selectedTreeTarget()
	if collectionIndex < 0 {
		return false
	}
	st.applyEditorsToRequest()
	request := httpclient.Request{
		ID:     "request-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Method: "GET",
		URL:    "{{base_url}}/",
	}
	var ref httpRequestRef
	if folderIndex >= 0 {
		folder := &st.file.Collections[collectionIndex].Folders[folderIndex]
		request.Name = uniqueHTTPRequestName(folder.Requests, "New request")
		folder.Requests = append(folder.Requests, request)
		ref = httpRequestRef{collection: collectionIndex, folder: folderIndex, request: len(folder.Requests) - 1}
		delete(st.collapsedFolders, [2]int{collectionIndex, folderIndex})
	} else {
		collection := &st.file.Collections[collectionIndex]
		request.Name = uniqueHTTPRequestName(collection.Requests, "New request")
		collection.Requests = append(collection.Requests, request)
		ref = httpRequestRef{collection: collectionIndex, folder: -1, request: len(collection.Requests) - 1}
	}
	delete(st.collapsedCollections, collectionIndex)
	st.selectRequest(ref)
	st.dirty = true
	st.status = "added " + request.Name
	return true
}

func (h *httpSplitHandle) update(gtx layout.Context, axis layout.Axis, ratio *float32, extent, minRatio, maxRatio float32) bool {
	if h == nil || ratio == nil || extent <= 0 {
		return false
	}
	changed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &h.tag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Enter | pointer.Leave,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		position := pe.Position.X
		if axis == layout.Vertical {
			position = pe.Position.Y
		}
		switch pe.Kind {
		case pointer.Enter:
			h.hovering = true
		case pointer.Leave:
			h.hovering = false
		case pointer.Press:
			if pe.Buttons.Contain(pointer.ButtonPrimary) {
				h.dragging = true
				h.pointerID = pe.PointerID
				h.start = position
				h.startRatio = *ratio
				gtx.Execute(pointer.GrabCmd{Tag: &h.tag, ID: pe.PointerID})
			}
		case pointer.Drag:
			if h.dragging && h.pointerID == pe.PointerID {
				next := h.startRatio + (position-h.start)/extent
				next = max(minRatio, min(maxRatio, next))
				if next != *ratio {
					*ratio = next
					changed = true
				}
			}
		case pointer.Release, pointer.Cancel:
			if h.dragging && h.pointerID == pe.PointerID {
				h.dragging = false
			}
		}
	}
	if changed {
		gtx.Execute(op.InvalidateCmd{})
	}
	return changed
}

func (h *httpSplitHandle) updatePixels(gtx layout.Context, axis layout.Axis, value *int, minValue, maxValue int) bool {
	if h == nil || value == nil || maxValue < minValue {
		return false
	}
	changed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &h.tag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		position := pe.Position.X
		if axis == layout.Vertical {
			position = pe.Position.Y
		}
		switch pe.Kind {
		case pointer.Press:
			if pe.Buttons.Contain(pointer.ButtonPrimary) {
				h.dragging = true
				h.pointerID = pe.PointerID
				h.start = position
				h.startPixels = *value
				gtx.Execute(pointer.GrabCmd{Tag: &h.tag, ID: pe.PointerID})
			}
		case pointer.Drag:
			if h.dragging && h.pointerID == pe.PointerID {
				next := h.startPixels + int(position-h.start)
				next = max(minValue, min(maxValue, next))
				if next != *value {
					*value = next
					changed = true
				}
			}
		case pointer.Release, pointer.Cancel:
			if h.dragging && h.pointerID == pe.PointerID {
				h.dragging = false
			}
		}
	}
	if changed {
		gtx.Execute(op.InvalidateCmd{})
	}
	return changed
}

func layoutHTTPSplitOverlay(gtx layout.Context, handle *httpSplitHandle, axis layout.Axis, hit image.Rectangle, ratio *float32, extent, minRatio, maxRatio float32) {
	if handle == nil || hit.Empty() {
		return
	}
	handle.update(gtx, axis, ratio, extent, minRatio, maxRatio)
	if handle.dragging {
		if axis == layout.Horizontal {
			pointer.CursorColResize.Add(gtx.Ops)
		} else {
			pointer.CursorRowResize.Add(gtx.Ops)
		}
	}
	stack := clip.Rect(hit).Push(gtx.Ops)
	event.Op(gtx.Ops, &handle.tag)
	if axis == layout.Horizontal {
		pointer.CursorColResize.Add(gtx.Ops)
	} else {
		pointer.CursorRowResize.Add(gtx.Ops)
	}
	stack.Pop()
}

func layoutHTTPSplitPixelsOverlay(gtx layout.Context, handle *httpSplitHandle, axis layout.Axis, hit image.Rectangle, value *int, minValue, maxValue int) {
	if handle == nil || hit.Empty() {
		return
	}
	handle.updatePixels(gtx, axis, value, minValue, maxValue)
	if handle.dragging {
		if axis == layout.Horizontal {
			pointer.CursorColResize.Add(gtx.Ops)
		} else {
			pointer.CursorRowResize.Add(gtx.Ops)
		}
	}
	stack := clip.Rect(hit).Push(gtx.Ops)
	event.Op(gtx.Ops, &handle.tag)
	if axis == layout.Horizontal {
		pointer.CursorColResize.Add(gtx.Ops)
	} else {
		pointer.CursorRowResize.Add(gtx.Ops)
	}
	stack.Pop()
}

func (ui *UI) layoutHTTPCollections(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	rows := st.collectionRows()
	st.ensureRequestClicks(len(rows))
	st.ensureGroupClicks(len(rows))
	environmentName := "no environment"
	if st.file != nil && len(st.file.Environments) > 0 {
		if st.environment < 0 || st.environment >= len(st.file.Environments) {
			st.environment = 0
		}
		environmentName = st.file.Environments[st.environment].Name
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return fillBgExact(gtx, color.NRGBA{R: 15, G: 23, B: 30, A: 255}, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, ui.tabStripHeight(gtx), func(gtx layout.Context) layout.Dimensions {
							return ui.layoutHTTPEnvironmentLine(th, gtx, ui.mainTypeface(), st, environmentName)
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						style := material.List(th, &st.collectionList)
						style.Track.Color = color.NRGBA{R: 19, G: 34, B: 41, A: 255}
						style.Track.MajorPadding = unit.Dp(1)
						style.Track.MinorPadding = unit.Dp(1)
						style.Indicator.Color = color.NRGBA{R: 73, G: 119, B: 128, A: 255}
						style.Indicator.HoverColor = analyzerAccent
						style.Indicator.MajorMinLen = unit.Dp(18)
						style.Indicator.MinorWidth = unit.Dp(4)
						style.Indicator.CornerRadius = 0
						return layoutHTTPCollectionTree(gtx, style, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
							row := rows[index]
							if row.kind == "request" {
								return ui.layoutHTTPRequestRow(th, gtx, st, row, &st.requestClicks[index])
							}
							return ui.layoutHTTPGroupRow(th, gtx, st, row, &st.groupClicks[index])
						})
					}),
				)
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if !st.envMenuOpen || st.file == nil || len(st.file.Environments) == 0 {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			labels := make([]string, len(st.file.Environments))
			for index := range st.file.Environments {
				labels[index] = st.file.Environments[index].Name
			}
			menuWidth := min(gtx.Dp(unit.Dp(170)), gtx.Constraints.Max.X)
			menuGTX := gtx
			menuGTX.Constraints.Min = image.Pt(menuWidth, 0)
			menuGTX.Constraints.Max = image.Pt(menuWidth, max(0, gtx.Constraints.Max.Y-ui.tabStripHeight(gtx)))
			offset := op.Offset(image.Pt(gtx.Dp(unit.Dp(19)), ui.tabStripHeight(gtx))).Push(gtx.Ops)
			dimensions := ui.layoutHTTPChoiceMenu(th, menuGTX, labels, st.environment, st.envMenuClicks)
			registerHTTPMenuSurface(menuGTX, st, dimensions.Size)
			offset.Pop()
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if !st.treeMenuOpen {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			itemCount := 2
			if st.treeMenuRow.kind == "request" && st.file != nil {
				itemCount += len(st.file.Environments)
			}
			itemHeight := max(gtx.Dp(unit.Dp(24)), gtx.Sp(scaleThemeFontSize(th, 10))+gtx.Dp(unit.Dp(8)))
			menuWidth := min(gtx.Dp(unit.Dp(190)), gtx.Constraints.Max.X)
			menuHeight := min(itemCount*itemHeight, gtx.Constraints.Max.Y)
			y := ui.tabStripHeight(gtx) + st.treeMenuRowIndex*itemHeight
			y = max(0, min(max(0, gtx.Constraints.Max.Y-menuHeight), y))
			x := min(gtx.Dp(unit.Dp(42)), max(0, gtx.Constraints.Max.X-menuWidth))
			menuGTX := gtx
			menuGTX.Constraints.Min = image.Pt(menuWidth, 0)
			menuGTX.Constraints.Max = image.Pt(menuWidth, max(0, gtx.Constraints.Max.Y-y))
			offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
			dimensions := ui.layoutHTTPTreeContextMenu(th, menuGTX, st)
			registerHTTPMenuSurface(menuGTX, st, dimensions.Size)
			offset.Pop()
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
	)
}

func (ui *UI) layoutHTTPTreeContextMenu(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	return layoutHTTPCommandFrame(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.layoutHTTPChoiceMenuItem(th, gtx, &st.treeMenuRenameClick, "Rename", false)
			}),
		}
		if st.treeMenuRow.kind == "request" && st.file != nil {
			for index := range st.file.Environments {
				index := index
				if index >= len(st.treeMenuRunClicks) {
					break
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutHTTPChoiceMenuItem(th, gtx, &st.treeMenuRunClicks[index], "Run with "+st.file.Environments[index].Name, false)
				}))
			}
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutHTTPChoiceMenuItem(th, gtx, &st.treeMenuDeleteClick, "Delete", false)
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (ui *UI) layoutHTTPEnvironmentLine(th *material.Theme, gtx layout.Context, typeface font.Typeface, st *httpClientState, environmentName string) layout.Dimensions {
	return layoutHTTPCollectionWirePanel(gtx, color.NRGBA{R: 15, G: 23, B: 30, A: 255}, analyzerRule, false, -1, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(19), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			controlHeight := min(gtx.Constraints.Max.Y, max(gtx.Dp(unit.Dp(22)), gtx.Sp(scaleThemeFontSize(th, 10))+gtx.Dp(unit.Dp(8))))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, controlHeight, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutHTTPEnvironmentChooser(th, gtx, typeface, st, environmentName)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, controlHeight, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutHTTPWireActionTooltip(th, gtx, typeface, &st.addCollectionClick, "[ + ]", "New collection")
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, controlHeight, func(gtx layout.Context) layout.Dimensions {
						return ui.layoutHTTPWireActionTooltip(th, gtx, typeface, &st.addRequestClick, "[ + ]", "New request")
					})
				}),
			)
		})
	})
}

func (ui *UI) layoutHTTPEnvironmentChooser(th *material.Theme, gtx layout.Context, typeface font.Typeface, st *httpClientState, environmentName string) layout.Dimensions {
	bg := color.NRGBA{R: 15, G: 23, B: 30, A: 255}
	segment := func(click *widget.Clickable, text string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			segmentBG := bg
			textColor := txtColor
			if click.Hovered() {
				segmentBG = color.NRGBA{R: 20, G: 40, B: 48, A: 255}
				textColor = analyzerAccent
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				return fillBgExact(gtx, segmentBG, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(th, text)
					label.Font.Typeface = typeface
					label.Font.Weight = font.Medium
					label.TextSize = scaleThemeFontSize(th, 10)
					label.Color = textColor
					return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutInkVCenteredLabel(gtx, label)
					})
				})
			})
		}
	}
	literal := func(text string, textColor color.NRGBA) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(th, text)
			label.Font.Typeface = typeface
			label.Font.Weight = font.Medium
			label.TextSize = scaleThemeFontSize(th, 10)
			label.Color = textColor
			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layoutInkVCenteredLabel(gtx, label)
			})
		})
	}
	dimensions := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		literal("[", txtColor),
		layout.Rigid(segment(&st.envClick, "ENV: "+environmentName+" ▾")),
		literal("|", analyzerRule),
		layout.Rigid(segment(&st.addEnvironmentClick, "+")),
		literal("]", txtColor),
	)
	if st.envClick.Hovered() {
		ui.deferHTTPActionTooltip(th, gtx, dimensions.Size, "Double-click to edit")
	} else if st.addEnvironmentClick.Hovered() {
		ui.deferHTTPActionTooltip(th, gtx, dimensions.Size, "New environment")
	}
	return dimensions
}

func (ui *UI) layoutHTTPEnvironmentEditor(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	return fillBgExact(gtx, analyzerSurfaceBg, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, ui.tabStripHeight(gtx), func(gtx layout.Context) layout.Dimensions {
					dimensions, _ := layoutHTTPWireLinePanel(gtx, analyzerHeaderBg, analyzerRule, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layoutHTTPWireLabelTight(th, gtx, ui.mainTypeface(), "[ ENVIRONMENT ]", analyzerAccent, analyzerHeaderBg)
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
								}),
							)
						})
					})
					return dimensions
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, httpRequestLineHeight(th, gtx), func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPConnectedRow(gtx, color.NRGBA{R: 11, G: 20, B: 27, A: 255}, analyzerRule, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Body2(th, "NAME")
								label.Font.Typeface = ui.mainTypeface()
								label.Font.Weight = font.Medium
								label.TextSize = scaleThemeFontSize(th, 10)
								label.Color = analyzerAccent
								return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layoutInkVCenteredLabel(gtx, label)
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								if st.envEditorFocus == "name" {
									st.envEditorFocus = ""
									gtx.Execute(key.FocusCmd{Tag: &st.envEditorNameEd})
								}
								gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
								return layoutHTTPURLField(gtx, gtx.Focused(&st.envEditorNameEd), func(gtx layout.Context) layout.Dimensions {
									return layoutHTTPSingleLineEditor(th, gtx, ui, "http-environment-name", &st.envEditorNameEd, "environment name")
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutHTTPCommandSeparator(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutHTTPFlatCommandButton(th, gtx, st, "env-editor-cancel", &st.envEditorCancelClick, "CANCEL", unit.Dp(64), txtColor, false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutHTTPCommandSeparator(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutHTTPFlatCommandButton(th, gtx, st, "env-editor-save", &st.envEditorSaveClick, "SAVE", unit.Dp(52), analyzerAccent, true)
							}),
						)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				count := len(parseHTTPEnvironmentVariables(st.envEditorVarsEd.Text()))
				return fixedHeight(gtx, max(gtx.Dp(unit.Dp(26)), gtx.Sp(scaleThemeFontSize(th, 10))+gtx.Dp(unit.Dp(10))), func(gtx layout.Context) layout.Dimensions {
					dimensions, _ := layoutHTTPWireLinePanel(gtx, analyzerHeaderBg, analyzerRule, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layoutHTTPWireLabelTight(th, gtx, ui.mainTypeface(), "[ VARIABLES: "+strconv.Itoa(count)+" ]", analyzerAccent, analyzerHeaderBg)
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutHTTPWireActionTooltip(th, gtx, ui.mainTypeface(), &st.envEditorAddClick, "[ + ]", "Add parameter")
								}),
							)
						})
					})
					return dimensions
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if st.envEditorFocus == "variables" {
					st.envEditorFocus = ""
					gtx.Execute(key.FocusCmd{Tag: &st.envEditorVarsEd})
				}
				return layoutHTTPMultilineEditor(th, gtx, ui, "http-environment-variables", &st.envEditorVarsEd, &st.envEditorScrollbar, "base_url=https://api.example.test\ntoken=secret", false)
			}),
		)
	})
}

func layoutHTTPCollectionWirePanel(gtx layout.Context, bg, rule color.NRGBA, continueAbove bool, lineY int, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	size := constrainHTTPRecordedSize(gtx, dimensions.Size)
	if size.X < 1 || size.Y < 1 {
		call.Add(gtx.Ops)
		return dimensions
	}
	if lineY < 0 || lineY >= size.Y {
		lineY = httpWireLineY(gtx, size.Y)
	}
	spineX := min(size.X-1, httpCollectionSpineX(gtx))
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
	paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(spineX, lineY, size.X, lineY+1)).Op())
	spineTop := lineY
	if continueAbove {
		spineTop = 0
	}
	paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(spineX, spineTop, spineX+1, size.Y)).Op())
	stack := clip.Rect{Max: size}.Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
	return layout.Dimensions{Size: size}
}

func httpCollectionSpineX(gtx layout.Context) int {
	return max(1, gtx.Dp(unit.Dp(10)))
}

func layoutHTTPCollectionTree(gtx layout.Context, style material.ListStyle, length int, element layout.ListElement) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := style.Layout(gtx, length, element)
	call := recording.Stop()
	size := constrainHTTPRecordedSize(gtx, dimensions.Size)
	if size.X < 1 || size.Y < 1 {
		call.Add(gtx.Ops)
		return dimensions
	}
	spineX := min(size.X-1, httpCollectionSpineX(gtx))
	spineBottom := min(size.Y, gtx.Dp(unit.Dp(11)))
	paint.FillShape(gtx.Ops, analyzerRule, clip.Rect(image.Rect(spineX, 0, spineX+1, spineBottom)).Op())
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

func (ui *UI) layoutHTTPGroupRow(th *material.Theme, gtx layout.Context, st *httpClientState, row httpCollectionRow, click *widget.Clickable) layout.Dimensions {
	selected := st.hasTreeSelected && st.treeSelected.kind == row.kind && st.treeSelected.ref.collection == row.ref.collection && st.treeSelected.ref.folder == row.ref.folder
	bg := color.NRGBA{}
	if selected {
		bg = color.NRGBA{R: 22, G: 50, B: 60, A: 255}
	} else if click.Hovered() {
		bg = color.NRGBA{R: 20, G: 36, B: 44, A: 255}
	}
	layoutRow := func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layoutHTTPTreeRow(gtx, row, unit.Dp(2), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !row.hasChildren {
							return layout.Dimensions{}
						}
						glyph := "▶"
						if row.expanded {
							glyph = "▼"
						}
						label := material.Body2(th, glyph)
						label.Font.Typeface = ui.mainTypeface()
						label.Font.Weight = font.Bold
						label.TextSize = scaleThemeFontSize(th, 11)
						label.Color = analyzerAccent
						return fixedWidth(gtx, gtx.Dp(unit.Dp(15)), func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, label.Layout)
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if st.treeRenameMatches(row) {
							return ui.layoutHTTPTreeRenameEditor(th, gtx, st)
						}
						label := material.Body2(th, row.label)
						label.Font.Typeface = ui.mainTypeface()
						label.Font.Weight = font.Medium
						label.TextSize = scaleThemeFontSize(th, 11)
						label.Color = analyzerAccent
						label.MaxLines = 1
						return label.Layout(gtx)
					}),
				)
			})
		})
	}
	if st.treeRenameMatches(row) {
		return layoutRow(gtx)
	}
	return click.Layout(gtx, layoutRow)
}

func (ui *UI) layoutHTTPRequestRow(th *material.Theme, gtx layout.Context, st *httpClientState, row httpCollectionRow, click *widget.Clickable) layout.Dimensions {
	request := st.requestAt(row.ref)
	if request == nil {
		return layout.Dimensions{}
	}
	selected := st.hasSelected && st.selected == row.ref
	bg := color.NRGBA{}
	if selected {
		bg = color.NRGBA{R: 22, G: 50, B: 60, A: 255}
	} else if click.Hovered() {
		bg = color.NRGBA{R: 20, G: 36, B: 44, A: 255}
	}
	methodColor := httpMethodColor(request.Method)
	layoutRow := func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layoutHTTPTreeRow(gtx, row, unit.Dp(3), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if st.treeRenameMatches(row) {
							return ui.layoutHTTPTreeRenameEditor(th, gtx, st)
						}
						label := material.Body2(th, row.label)
						label.Font.Typeface = ui.mainTypeface()
						label.TextSize = scaleThemeFontSize(th, 11)
						label.Color = txtColor
						label.MaxLines = 1
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Body2(th, compactHTTPMethod(request.Method))
						label.Font.Typeface = ui.mainTypeface()
						label.Font.Weight = font.Medium
						label.TextSize = scaleThemeFontSize(th, 10)
						label.Color = methodColor
						return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, label.Layout)
					}),
				)
			})
		})
	}
	if st.treeRenameMatches(row) {
		return layoutRow(gtx)
	}
	return click.Layout(gtx, layoutRow)
}

func (ui *UI) layoutHTTPTreeRenameEditor(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	if st.treeRenameFocus {
		st.treeRenameFocus = false
		gtx.Execute(key.FocusCmd{Tag: &st.treeRenameEd})
	}
	style := material.Editor(th, &st.treeRenameEd, "Name")
	style.Font.Typeface = ui.mainTypeface()
	style.Font.Weight = font.Medium
	style.TextSize = scaleThemeFontSize(th, 11)
	style.Color = txtColor
	style.HintColor = color.NRGBA{R: 105, G: 135, B: 142, A: 255}
	style.SelectionColor = color.NRGBA{R: analyzerAccent.R, G: analyzerAccent.G, B: analyzerAccent.B, A: 80}
	return layout.Inset{Right: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layoutHTTPURLField(gtx, true, style.Layout)
	})
}

const (
	httpTreeStepDp     = 22
	httpTreeArmDp      = 32
	httpTreeLabelGapDp = 6
)

func layoutHTTPTreeRow(gtx layout.Context, row httpCollectionRow, bottom unit.Dp, content layout.Widget) layout.Dimensions {
	textLeft := unit.Dp(10 + row.depth*httpTreeStepDp + httpTreeArmDp + httpTreeLabelGapDp)
	recording := op.Record(gtx.Ops)
	dimensions := layout.Inset{Left: textLeft, Right: unit.Dp(6), Top: unit.Dp(3), Bottom: bottom}.Layout(gtx, content)
	call := recording.Stop()
	size := constrainHTTPRecordedSize(gtx, dimensions.Size)
	if size.X < 1 || size.Y < 1 {
		call.Add(gtx.Ops)
		return dimensions
	}
	drawHTTPTreeBranches(gtx, row, size)
	stack := clip.Rect{Max: size}.Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
	return layout.Dimensions{Size: size}
}

func drawHTTPTreeBranches(gtx layout.Context, row httpCollectionRow, size image.Point) {
	baseX := httpCollectionSpineX(gtx)
	step := max(1, gtx.Dp(unit.Dp(httpTreeStepDp)))
	arm := max(1, gtx.Dp(unit.Dp(httpTreeArmDp)))
	middle := max(0, (size.Y-1)/2)
	rule := analyzerRule

	for depth, continues := range row.ancestorContinues {
		if !continues || depth >= row.depth {
			continue
		}
		x := baseX + depth*step
		if x >= 0 && x < size.X {
			paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(x, 0, x+1, size.Y)).Op())
		}
	}

	currentX := baseX + row.depth*step
	if currentX < 0 || currentX >= size.X {
		return
	}
	verticalBottom := size.Y
	if row.last {
		verticalBottom = middle + 1
	}
	paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(currentX, 0, currentX+1, verticalBottom)).Op())
	armEnd := min(size.X, currentX+arm+1)
	paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(currentX, middle, armEnd, middle+1)).Op())
	if row.hasChildren && row.expanded {
		childX := min(size.X-1, currentX+step)
		paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(childX, middle, childX+1, size.Y)).Op())
	}
}

func compactHTTPMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "DELETE" {
		return "DEL"
	}
	if method == "OPTIONS" {
		return "OPT"
	}
	return method
}

func httpMethodColor(method string) color.NRGBA {
	switch strings.ToUpper(method) {
	case "GET", "HEAD":
		return analyzerOK
	case "POST", "PUT", "PATCH":
		return color.NRGBA{R: 240, G: 180, B: 93, A: 255}
	case "DELETE":
		return analyzerError
	default:
		return analyzerAccent
	}
}

func (ui *UI) layoutHTTPWorkbench(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	requestRatio := st.requestRatio
	if requestRatio <= 0 {
		requestRatio = 0.42
	}
	requestRatio = max(float32(0.18), min(float32(0.75), requestRatio))
	st.requestRatio = requestRatio
	requestHeight := 0
	responseHeight := 0
	splitY := 0
	requestHeaderHeight := 0
	responseLineOffset := 0
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			dimensions := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := ui.layoutHTTPRequestTabs(th, gtx, st)
					requestHeaderHeight += dimensions.Size.Y
					return dimensions
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := ui.layoutHTTPRequestTabConnector(gtx, st)
					requestHeaderHeight += dimensions.Size.Y
					return dimensions
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := ui.layoutHTTPRequestLine(th, gtx, st)
					requestHeaderHeight += dimensions.Size.Y
					return dimensions
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := ui.layoutHTTPDetailTabs(th, gtx, st)
					requestHeaderHeight += dimensions.Size.Y
					return dimensions
				}),
				layout.Flexed(requestRatio, func(gtx layout.Context) layout.Dimensions {
					dimensions := ui.layoutHTTPDetailEditor(th, gtx, st)
					requestHeight = dimensions.Size.Y
					return dimensions
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := ui.layoutHTTPResponseHeader(th, gtx, st, &responseLineOffset)
					return dimensions
				}),
				layout.Flexed(1-requestRatio, func(gtx layout.Context) layout.Dimensions {
					dimensions := ui.layoutHTTPResponseEditor(th, gtx, st)
					responseHeight = dimensions.Size.Y
					return dimensions
				}),
			)
			if extent := requestHeight + responseHeight; extent > 0 {
				st.requestExtent = float32(extent)
			}
			splitY = requestHeaderHeight + requestHeight + responseLineOffset
			st.requestSplitY = splitY
			return dimensions
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			startY := ui.tabStripHeight(gtx)
			if size.X > 0 && size.Y > startY {
				paint.FillShape(gtx.Ops, analyzerRule, clip.Rect(image.Rect(size.X-1, startY, size.X, size.Y)).Op())
			}
			return layout.Dimensions{Size: size}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if splitY > 0 {
				halfHit := max(4, gtx.Dp(unit.Dp(6)))
				hit := image.Rect(0, splitY-halfHit, gtx.Constraints.Max.X, splitY+halfHit+1)
				extent := st.requestExtent
				if extent <= 0 {
					extent = float32(gtx.Constraints.Max.Y)
				}
				layoutHTTPSplitOverlay(gtx, &st.requestSplit, layout.Vertical, hit, &st.requestRatio, extent, 0.18, 0.75)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if !st.methodMenuOpen {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			menuWidth := min(gtx.Dp(unit.Dp(106)), gtx.Constraints.Max.X)
			menuY := ui.tabStripHeight(gtx) + max(3, gtx.Dp(unit.Dp(filePaneTabConnectorHeightDp))) + httpRequestLineHeight(th, gtx)
			menuGTX := gtx
			menuGTX.Constraints.Min = image.Pt(menuWidth, 0)
			menuGTX.Constraints.Max = image.Pt(menuWidth, max(0, gtx.Constraints.Max.Y-menuY))
			offset := op.Offset(image.Pt(0, menuY)).Push(gtx.Ops)
			dimensions := ui.layoutHTTPChoiceMenu(th, menuGTX, httpMethods, httpChoiceIndex(httpMethods, st.method), st.methodMenuClicks)
			registerHTTPMenuSurface(menuGTX, st, dimensions.Size)
			offset.Pop()
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
	)
}

func (ui *UI) layoutHTTPRequestTabs(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	items := make([]appTabItem, len(st.openTabs))
	for index, ref := range st.openTabs {
		title := "Missing request"
		if request := st.requestAt(ref); request != nil {
			title = request.Name
		}
		items[index] = appTabItem{
			title:  title,
			active: index == st.activeTab,
		}
	}
	style := appTabStripStyle{
		open:             true,
		activeBackground: httpRequestLineBackground(),
	}
	actions, dimensions, geometry := ui.layoutAppTabStrip(
		th,
		gtx,
		items,
		&st.tabScroll,
		&st.tabClicks,
		&st.tabCloses,
		&st.tabPrev,
		&st.tabNext,
		&st.tabAdd,
		style,
	)
	st.tabGeometry = geometry
	if actions.selectIdx >= 0 && st.activateRequestTab(actions.selectIdx) {
		gtx.Execute(op.InvalidateCmd{})
	}
	if actions.closeIdx >= 0 && st.closeRequestTab(actions.closeIdx) {
		gtx.Execute(op.InvalidateCmd{})
	}
	if actions.add && st.addScratchRequest() {
		gtx.Execute(op.InvalidateCmd{})
	}
	return dimensions
}

func (ui *UI) layoutHTTPRequestTabConnector(gtx layout.Context, st *httpClientState) layout.Dimensions {
	h := gtx.Dp(unit.Dp(filePaneTabConnectorHeightDp))
	if h < 3 {
		h = 3
	}
	w := gtx.Constraints.Max.X
	if w < 1 {
		return layout.Dimensions{Size: image.Pt(0, h)}
	}
	paint.FillShape(gtx.Ops, httpRequestLineBackground(), clip.Rect(image.Rect(0, 0, w, h)).Op())

	geometry := st.tabGeometry
	activeMin := max(0, min(w, geometry.activeMinX))
	activeMax := max(activeMin, min(w, geometry.activeMaxX))
	stroke := float32(max(1, gtx.Dp(unit.Dp(1))))
	inset := stroke / 2
	railY := inset
	stemTop := -float32(gtx.Dp(unit.Dp(4)))
	leftX := inset
	rightX := float32(w-1) - inset

	var rail clip.Path
	rail.Begin(gtx.Ops)
	if geometry.activeVisible && activeMax > activeMin {
		leftNotch := float32(activeMin)
		rightNotch := float32(activeMax)
		if activeMin > 0 {
			rail.MoveTo(f32.Pt(leftX, railY))
			rail.LineTo(f32.Pt(leftNotch, railY))
			rail.LineTo(f32.Pt(leftNotch, stemTop))
		}
		rail.MoveTo(f32.Pt(rightNotch, stemTop))
		rail.LineTo(f32.Pt(rightNotch, railY))
		rail.LineTo(f32.Pt(rightX, railY))
	} else {
		rail.MoveTo(f32.Pt(leftX, railY))
		rail.LineTo(f32.Pt(rightX, railY))
	}
	paint.FillShape(gtx.Ops, analyzerRule, clip.Stroke{Path: rail.End(), Width: stroke}.Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func httpRequestLineBackground() color.NRGBA {
	return color.NRGBA{R: 14, G: 21, B: 27, A: 255}
}

func httpRequestLineHeight(th *material.Theme, gtx layout.Context) int {
	height := gtx.Sp(scaleThemeFontSize(th, 11)) + gtx.Dp(unit.Dp(17))
	return max(gtx.Dp(unit.Dp(28)), height)
}

func httpRequestHeaderBlockHeight(th *material.Theme, gtx layout.Context) int {
	return gtx.Dp(unit.Dp(filePaneTabConnectorHeightDp)) + httpRequestLineHeight(th, gtx)
}

func (ui *UI) layoutHTTPRequestLine(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	return fixedHeight(gtx, httpRequestLineHeight(th, gtx), func(gtx layout.Context) layout.Dimensions {
		return layoutHTTPRequestWirePanel(gtx, httpRequestLineBackground(), true, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutHTTPFlatCommandButton(th, gtx, st, "method", &st.methodClick, st.method+" ▾", unit.Dp(62), httpMethodColor(st.method), false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPCommandSeparator(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPRequestURLSegment(th, gtx, ui, &st.urlEd)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPCommandSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "SEND"
					if st.sending {
						label = "SENDING…"
					}
					return ui.layoutHTTPFlatCommandButton(th, gtx, st, "send", &st.sendClick, label, unit.Dp(68), analyzerAccent, true)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPCommandSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutHTTPFlatSaveButton(gtx, st)
				}),
			)
		})
	})
}

func layoutHTTPRequestWirePanel(gtx layout.Context, bg color.NRGBA, bottomBorder bool, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	size := constrainHTTPRecordedSize(gtx, dimensions.Size)
	if size.X < 1 || size.Y < 1 {
		call.Add(gtx.Ops)
		return dimensions
	}

	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
	if bottomBorder {
	}
	stack := clip.Rect{Max: size}.Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
	if bottomBorder {
		paint.FillShape(gtx.Ops, httpCommandUnderlineColor(), clip.Rect(image.Rect(0, size.Y-1, size.X, size.Y)).Op())
	}
	return layout.Dimensions{Size: size}
}

func httpCommandUnderlineColor() color.NRGBA {
	return color.NRGBA{R: 66, G: 111, B: 121, A: 255}
}

func httpChoiceIndex(values []string, selected string) int {
	for index := range values {
		if values[index] == selected {
			return index
		}
	}
	return -1
}

func constrainHTTPRecordedSize(gtx layout.Context, size image.Point) image.Point {
	if size.X < gtx.Constraints.Min.X {
		size.X = gtx.Constraints.Min.X
	}
	if size.Y < gtx.Constraints.Min.Y {
		size.Y = gtx.Constraints.Min.Y
	}
	if size.X > gtx.Constraints.Max.X {
		size.X = gtx.Constraints.Max.X
	}
	if size.Y > gtx.Constraints.Max.Y {
		size.Y = gtx.Constraints.Max.Y
	}
	return size
}

func layoutHTTPWireGap(gtx layout.Context, width unit.Dp) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(gtx.Dp(width), 1)}
}

func (ui *UI) layoutHTTPFlatCommandButton(th *material.Theme, gtx layout.Context, st *httpClientState, id string, click *widget.Clickable, text string, width unit.Dp, textColor color.NRGBA, primary bool) layout.Dimensions {
	return fixedWidth(gtx, gtx.Dp(width), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			hover := httpHoverFill(gtx, st, id)
			bg := color.NRGBA{R: 17, G: 30, B: 37, A: 255}
			target := color.NRGBA{R: 32, G: 55, B: 64, A: 255}
			if primary {
				bg = color.NRGBA{R: 16, G: 38, B: 44, A: 255}
				target = color.NRGBA{R: 25, G: 68, B: 78, A: 255}
			}
			bg = mixNRGBA(bg, target, hover)
			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(th, text)
					label.Font.Typeface = ui.mainTypeface()
					label.Font.Weight = font.Medium
					label.TextSize = scaleThemeFontSize(th, 10)
					label.Color = textColor
					label.MaxLines = 1
					return layout.Center.Layout(gtx, label.Layout)
				})
			})
		})
	})
}

func layoutHTTPCommandSeparator(gtx layout.Context) layout.Dimensions {
	return fixedWidth(gtx, 1, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		if size.X < 1 || size.Y < 1 {
			return layout.Dimensions{Size: size}
		}
		paint.FillShape(gtx.Ops, analyzerRule, clip.Rect(image.Rect(0, 0, 1, size.Y)).Op())
		return layout.Dimensions{Size: size}
	})
}

func (ui *UI) layoutHTTPFlatSaveButton(gtx layout.Context, st *httpClientState) layout.Dimensions {
	width := gtx.Dp(unit.Dp(30))
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return st.saveClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			hover := httpHoverFill(gtx, st, "save")
			bg := mixNRGBA(httpRequestLineBackground(), color.NRGBA{R: 32, G: 55, B: 64, A: 255}, hover)
			paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
			if st.dirty {
				dot := max(2, gtx.Dp(unit.Dp(2)))
				paint.FillShape(gtx.Ops, analyzerAccent, clip.Rect(image.Rect(gtx.Constraints.Max.X-dot-2, 2, gtx.Constraints.Max.X-2, 2+dot)).Op())
			}
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := max(11, gtx.Dp(unit.Dp(14)))
				iconGTX := gtx
				iconGTX.Constraints = layout.Exact(image.Pt(size, size))
				uitheme.SaveIcon().Layout(iconGTX, txtColor)
				return layout.Dimensions{Size: image.Pt(size, size)}
			})
		})
	})
}

func layoutHTTPRequestWireButton(th *material.Theme, gtx layout.Context, typeface font.Typeface, click *widget.Clickable, text string, method, primary bool) layout.Dimensions {
	bg := httpRequestLineBackground()
	textColor := txtColor
	if method {
		methodName := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "[ "), " ▾ ]"))
		textColor = httpMethodColor(methodName)
	}
	if primary {
		textColor = analyzerAccent
	}
	if click.Hovered() {
		bg = color.NRGBA{R: 28, G: 49, B: 59, A: 255}
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layoutHTTPWireLabelTight(th, gtx, typeface, text, textColor, bg)
	})
}

func layoutHTTPRequestURLSegment(th *material.Theme, gtx layout.Context, ui *UI, editor *widget.Editor) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	return layoutHTTPURLField(gtx, gtx.Focused(editor), func(gtx layout.Context) layout.Dimensions {
		return layoutHTTPSingleLineEditor(th, gtx, ui, "http-url", editor, "https://api.example.test/v1/resource")
	})
}

func layoutHTTPURLField(gtx layout.Context, focused bool, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	size := constrainHTTPRecordedSize(gtx, dimensions.Size)
	bg := color.NRGBA{R: 8, G: 18, B: 24, A: 255}
	underline := httpCommandUnderlineColor()
	underlineH := 1
	if focused {
		bg = color.NRGBA{R: 12, G: 27, B: 35, A: 255}
		underline = analyzerAccent
		underlineH = max(1, gtx.Dp(unit.Dp(2)))
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
	paint.FillShape(gtx.Ops, color.NRGBA{R: 90, G: 130, B: 139, A: 70}, clip.Rect(image.Rect(0, 0, size.X, 1)).Op())
	paint.FillShape(gtx.Ops, underline, clip.Rect(image.Rect(0, size.Y-underlineH, size.X, size.Y)).Op())
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

func (ui *UI) layoutHTTPChoiceMenu(th *material.Theme, gtx layout.Context, labels []string, active int, clicks []widget.Clickable) layout.Dimensions {
	return layoutHTTPCommandFrame(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(labels))
		for index := range labels {
			index := index
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if index >= len(clicks) {
					return layout.Dimensions{}
				}
				return ui.layoutHTTPChoiceMenuItem(th, gtx, &clicks[index], labels[index], index == active)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (ui *UI) layoutHTTPChoiceMenuItem(th *material.Theme, gtx layout.Context, click *widget.Clickable, label string, active bool) layout.Dimensions {
	return fixedHeight(gtx, max(gtx.Dp(unit.Dp(24)), gtx.Sp(scaleThemeFontSize(th, 10))+gtx.Dp(unit.Dp(8))), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			bg := httpChoiceMenuBackground()
			textColor := txtColor
			if active {
				bg = color.NRGBA{R: 18, G: 47, B: 56, A: 255}
				textColor = analyzerAccent
			} else if click.Hovered() {
				bg = color.NRGBA{R: 25, G: 47, B: 56, A: 255}
			}
			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							marker := " "
							if active {
								marker = "*"
							}
							markerLabel := material.Body2(th, marker)
							markerLabel.Font.Typeface = ui.mainTypeface()
							markerLabel.TextSize = scaleThemeFontSize(th, 10)
							markerLabel.Color = analyzerAccent
							return fixedWidth(gtx, gtx.Dp(unit.Dp(13)), func(gtx layout.Context) layout.Dimensions {
								return layoutVCenteredLabel(gtx, markerLabel)
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							textLabel := material.Body2(th, label)
							textLabel.Font.Typeface = ui.mainTypeface()
							textLabel.Font.Weight = font.Medium
							textLabel.TextSize = scaleThemeFontSize(th, 10)
							textLabel.Color = textColor
							textLabel.MaxLines = 1
							textLabel.Truncator = "…"
							return layoutVCenteredLabel(gtx, textLabel)
						}),
					)
				})
			})
		})
	})
}

func layoutHTTPCommandFrame(gtx layout.Context, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	size := dimensions.Size
	if size.X < gtx.Constraints.Min.X {
		size.X = gtx.Constraints.Min.X
	}
	if size.Y < gtx.Constraints.Min.Y {
		size.Y = gtx.Constraints.Min.Y
	}
	if size.X > gtx.Constraints.Max.X {
		size.X = gtx.Constraints.Max.X
	}
	if size.Y > gtx.Constraints.Max.Y {
		size.Y = gtx.Constraints.Max.Y
	}
	if size.X < 1 || size.Y < 1 {
		call.Add(gtx.Ops)
		return dimensions
	}
	paint.FillShape(gtx.Ops, httpChoiceMenuBackground(), clip.Rect{Max: size}.Op())
	stack := clip.Rect{Max: size}.Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
	popupBorder := color.NRGBA{R: 87, G: 143, B: 154, A: 255}
	paint.FillShape(gtx.Ops, popupBorder, clip.Stroke{Path: rectangularPath(gtx.Ops, size), Width: 1}.Op())
	if size.X > 2 && size.Y > 2 {
		inner := color.NRGBA{R: 36, G: 64, B: 72, A: 210}
		paint.FillShape(gtx.Ops, inner, clip.Rect(image.Rect(1, 1, size.X-1, 2)).Op())
		paint.FillShape(gtx.Ops, inner, clip.Rect(image.Rect(1, 1, 2, size.Y-1)).Op())
	}
	return layout.Dimensions{Size: size}
}

func httpChoiceMenuBackground() color.NRGBA {
	return color.NRGBA{R: 13, G: 27, B: 34, A: 255}
}

func rectangularPath(ops *op.Ops, size image.Point) clip.PathSpec {
	var path clip.Path
	path.Begin(ops)
	path.MoveTo(f32.Pt(0.5, 0.5))
	path.LineTo(f32.Pt(float32(size.X)-0.5, 0.5))
	path.LineTo(f32.Pt(float32(size.X)-0.5, float32(size.Y)-0.5))
	path.LineTo(f32.Pt(0.5, float32(size.Y)-0.5))
	path.Close()
	return path.End()
}

func layoutHTTPConnectedRow(gtx layout.Context, bg, border color.NRGBA, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	size := dimensions.Size
	if size.X < gtx.Constraints.Min.X {
		size.X = gtx.Constraints.Min.X
	}
	if size.X > gtx.Constraints.Max.X {
		size.X = gtx.Constraints.Max.X
	}
	if size.X < 1 || size.Y < 1 {
		call.Add(gtx.Ops)
		return dimensions
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
	paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(0, size.Y-1, size.X, size.Y)).Op())
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

func (ui *UI) layoutHTTPDetailTabs(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	labels := []string{
		"Params (" + strconv.Itoa(len(parseKeyValueLines(st.queryEd.Text(), "="))) + ")",
		"Headers (" + strconv.Itoa(len(parseKeyValueLines(st.headersEd.Text(), ":"))) + ")",
		"Auth",
		"Body",
	}
	modes := []string{httpDetailParams, httpDetailHeaders, httpDetailAuth, httpDetailBody}
	height := max(gtx.Dp(unit.Dp(24)), gtx.Sp(scaleThemeFontSize(th, 10))+gtx.Dp(unit.Dp(10)))
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		dimensions, _ := layoutHTTPWireLinePanel(gtx, analyzerHeaderBg, analyzerRule, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutHTTPWireLabelTight(th, gtx, ui.mainTypeface(), "[ REQUEST ]", analyzerAccent, analyzerHeaderBg)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutHTTPWireGap(gtx, unit.Dp(6))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.layoutHTTPDetailTabsInline(th, gtx, st, labels, modes)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
					}),
				)
			})
		})
		return dimensions
	})
}

func (ui *UI) layoutHTTPDetailTabsInline(th *material.Theme, gtx layout.Context, st *httpClientState, labels, modes []string) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(labels)*2+2)
	for index := range labels {
		index := index
		if index > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutHTTPWireLabelTight(th, gtx, ui.mainTypeface(), "│", analyzerRule, analyzerHeaderBg)
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if index >= len(st.detailClicks) {
				return layout.Dimensions{}
			}
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return st.detailClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				textColor := txtColor
				if index < len(modes) && modes[index] == st.detailMode {
					textColor = analyzerAccent
				}
				hover := httpHoverFill(gtx, st, "detail-"+strconv.Itoa(index))
				bg := mixNRGBA(analyzerHeaderBg, color.NRGBA{R: 31, G: 55, B: 64, A: 255}, hover)
				return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Body2(th, labels[index])
						label.Font.Typeface = ui.mainTypeface()
						label.TextSize = scaleThemeFontSize(th, 10)
						label.Color = textColor
						return layoutVCenteredLabel(gtx, label)
					})
				})
			})
		}))
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutHTTPWireLabelTight(th, gtx, ui.mainTypeface(), "│", analyzerRule, analyzerHeaderBg)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return st.detailAddClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				hover := httpHoverFill(gtx, st, "detail-add")
				bg := mixNRGBA(analyzerHeaderBg, color.NRGBA{R: 31, G: 55, B: 64, A: 255}, hover)
				return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Body2(th, "+")
						label.Font.Typeface = ui.mainTypeface()
						label.Font.Weight = font.Medium
						label.TextSize = scaleThemeFontSize(th, 10)
						label.Color = analyzerAccent
						return layoutVCenteredLabel(gtx, label)
					})
				})
			})
		}),
	)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (ui *UI) layoutHTTPDetailEditor(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	editor := &st.bodyEd
	menuID := "http-body"
	hint := "{\n  \"field\": \"value\"\n}"
	switch st.detailMode {
	case httpDetailParams:
		editor = &st.queryEd
		menuID = "http-query"
		hint = "page=1\nlimit=20\n# disabled=value"
	case httpDetailHeaders:
		editor = &st.headersEd
		menuID = "http-headers"
		hint = "Accept: application/json\nAuthorization: Bearer {{token}}"
	case httpDetailAuth:
		editor = &st.authEd
		menuID = "http-auth"
		hint = "Bearer {{token}}\n\nThis value is sent as the Authorization header."
	}
	return layoutHTTPMultilineEditor(th, gtx, ui, menuID, editor, &st.detailScrollbar, hint, false)
}

func (ui *UI) layoutHTTPResponseHeader(th *material.Theme, gtx layout.Context, st *httpClientState, lineOffset *int) layout.Dimensions {
	right := "not sent"
	rightColor := hintColor
	if st.sending {
		right = "sending…"
		rightColor = analyzerAccent
	} else if st.response.Err != nil {
		right = "ERROR"
		rightColor = analyzerError
	} else if st.response.StatusCode > 0 {
		right = st.response.Status + " │ " + formatHTTPDuration(st.response.Duration) + " │ " + formatHTTPBytes(st.response.Size)
		if st.response.StatusCode >= 400 {
			rightColor = analyzerError
		} else {
			rightColor = analyzerOK
		}
	}
	rule := analyzerRule
	if st.requestSplit.dragging || st.requestSplit.hovering {
		rule = analyzerAccent
	}
	dimensions, lineY := layoutHTTPWireLinePanel(gtx, analyzerHeaderBg, rule, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPWireLabelTight(th, gtx, ui.mainTypeface(), "[ RESPONSE ]", analyzerAccent, analyzerHeaderBg)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPWireGap(gtx, unit.Dp(6))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutHTTPResponseTabsInline(th, gtx, st)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPWireLabelTight(th, gtx, ui.mainTypeface(), "[ "+right+" ]", rightColor, analyzerHeaderBg)
				}),
			)
		})
	})
	if lineOffset != nil {
		*lineOffset = lineY
	}
	return dimensions
}

func (ui *UI) layoutHTTPResponseTabsInline(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	labels := []string{"Pretty", "Raw", "Headers (" + strconv.Itoa(len(st.response.Headers)) + ")"}
	modes := []string{httpResponsePretty, httpResponseRaw, httpResponseHeaders}
	children := make([]layout.FlexChild, 0, len(labels)*2)
	for index := range labels {
		index := index
		if index > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutHTTPWireLabelTight(th, gtx, ui.mainTypeface(), "│", analyzerRule, analyzerHeaderBg)
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return st.responseClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				active := modes[index] == st.responseMode
				textColor := txtColor
				if active {
					textColor = analyzerAccent
				}
				hover := httpHoverFill(gtx, st, "response-"+strconv.Itoa(index))
				bg := mixNRGBA(analyzerHeaderBg, color.NRGBA{R: 31, G: 55, B: 64, A: 255}, hover)
				return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(th, labels[index])
					label.Font.Typeface = ui.mainTypeface()
					label.TextSize = scaleThemeFontSize(th, 10)
					label.Color = textColor
					return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, label.Layout)
				})
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func layoutHTTPWireLinePanel(gtx layout.Context, bg, rule color.NRGBA, content layout.Widget) (layout.Dimensions, int) {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	size := dimensions.Size
	if size.X < gtx.Constraints.Min.X {
		size.X = gtx.Constraints.Min.X
	}
	if size.Y < gtx.Constraints.Min.Y {
		size.Y = gtx.Constraints.Min.Y
	}
	if size.X > gtx.Constraints.Max.X {
		size.X = gtx.Constraints.Max.X
	}
	if size.Y > gtx.Constraints.Max.Y {
		size.Y = gtx.Constraints.Max.Y
	}
	lineY := httpWireLineY(gtx, size.Y)
	if size.X > 0 && size.Y > 0 {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
		paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(0, lineY, size.X, lineY+1)).Op())
		stack := clip.Rect{Max: size}.Push(gtx.Ops)
		call.Add(gtx.Ops)
		stack.Pop()
	} else {
		call.Add(gtx.Ops)
	}
	return layout.Dimensions{Size: size}, lineY
}

func httpWireLineY(_ layout.Context, height int) int {
	return max(0, (height-1)/2)
}

func (ui *UI) layoutHTTPResponseEditor(th *material.Theme, gtx layout.Context, st *httpClientState) layout.Dimensions {
	hint := "Send a request to inspect the response."
	return layoutHTTPMultilineEditor(th, gtx, ui, "http-response", &st.responseEd, &st.responseScrollbar, hint, true)
}

func layoutHTTPSectionHeader(th *material.Theme, gtx layout.Context, typeface font.Typeface, title, right string) layout.Dimensions {
	dimensions, _ := layoutHTTPWireLinePanel(gtx, analyzerHeaderBg, analyzerRule, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPWireLabelTight(th, gtx, typeface, "[ "+title+" ]", analyzerAccent, analyzerHeaderBg)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutHTTPWireLabelTight(th, gtx, typeface, "[ "+right+" ]", hintColor, analyzerHeaderBg)
				}),
			)
		})
	})
	return dimensions
}

func layoutHTTPWireLabelTight(th *material.Theme, gtx layout.Context, typeface font.Typeface, text string, textColor, bg color.NRGBA) layout.Dimensions {
	trimmed := strings.TrimSpace(text)
	bracketed := strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
	if bracketed {
		trimmed = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
	}
	content := func(gtx layout.Context) layout.Dimensions {
		label := material.Body2(th, trimmed)
		label.Font.Typeface = typeface
		label.Font.Weight = font.Medium
		label.TextSize = scaleThemeFontSize(th, 10)
		label.Color = textColor
		label.MaxLines = 1
		label.Truncator = "…"
		if bracketed {
			return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7)}.Layout(gtx, label.Layout)
		}
		return label.Layout(gtx)
	}
	if bracketed {
		return layoutHTTPWireBrackets(gtx, bg, textColor, content)
	}
	return fillBgExact(gtx, bg, content)
}

func layoutHTTPWireBrackets(gtx layout.Context, bg, fg color.NRGBA, content layout.Widget) layout.Dimensions {
	contentGTX := gtx
	contentGTX.Constraints.Min.Y = 0
	recording := op.Record(gtx.Ops)
	dimensions := content(contentGTX)
	call := recording.Stop()
	size := dimensions.Size
	if size.X < contentGTX.Constraints.Min.X {
		size.X = contentGTX.Constraints.Min.X
	}
	if size.X > contentGTX.Constraints.Max.X {
		size.X = contentGTX.Constraints.Max.X
	}
	if size.Y < contentGTX.Constraints.Min.Y {
		size.Y = contentGTX.Constraints.Min.Y
	}
	if size.Y > contentGTX.Constraints.Max.Y {
		size.Y = contentGTX.Constraints.Max.Y
	}
	if size.X < 2 || size.Y < 3 {
		call.Add(gtx.Ops)
		return dimensions
	}

	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
	top := 0
	bottom := size.Y - 1
	arm := min(size.X, max(2, gtx.Dp(unit.Dp(3))))
	paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(0, top, 1, bottom+1)).Op())
	paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(0, top, arm, top+1)).Op())
	paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(0, bottom, arm, bottom+1)).Op())
	paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(size.X-1, top, size.X, bottom+1)).Op())
	paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(size.X-arm, top, size.X, top+1)).Op())
	paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(size.X-arm, bottom, size.X, bottom+1)).Op())
	stack := clip.Rect{Max: size}.Push(gtx.Ops)
	contentOffset := op.Offset(image.Pt(0, max(0, (size.Y-dimensions.Size.Y)/2))).Push(gtx.Ops)
	call.Add(gtx.Ops)
	contentOffset.Pop()
	stack.Pop()
	return layout.Dimensions{Size: size, Baseline: dimensions.Baseline}
}

func layoutHTTPWireAction(th *material.Theme, gtx layout.Context, typeface font.Typeface, click *widget.Clickable, text string, active bool) layout.Dimensions {
	bg := color.NRGBA{R: 15, G: 23, B: 30, A: 255}
	textColor := txtColor
	if active {
		bg = color.NRGBA{R: 22, G: 50, B: 60, A: 255}
		textColor = analyzerAccent
	} else if click.Hovered() {
		bg = color.NRGBA{R: 20, G: 40, B: 48, A: 255}
		textColor = analyzerAccent
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layoutHTTPWireLabelTight(th, gtx, typeface, text, textColor, bg)
	})
}

func (ui *UI) layoutHTTPWireActionTooltip(th *material.Theme, gtx layout.Context, typeface font.Typeface, click *widget.Clickable, text, tip string) layout.Dimensions {
	dimensions := layoutHTTPWireAction(th, gtx, typeface, click, text, false)
	if click.Hovered() && strings.TrimSpace(tip) != "" {
		ui.deferHTTPActionTooltip(th, gtx, dimensions.Size, tip)
	}
	return dimensions
}

func (ui *UI) deferHTTPActionTooltip(th *material.Theme, gtx layout.Context, actionSize image.Point, tip string) {
	tipGTX := gtx
	tipGTX.Constraints.Min = image.Point{}
	tipGTX.Constraints.Max = image.Pt(gtx.Dp(unit.Dp(140)), gtx.Dp(unit.Dp(30)))
	tipRecord := op.Record(gtx.Ops)
	tipDimensions := ui.layoutHTTPActionTooltip(th, tipGTX, tip)
	tipCall := tipRecord.Stop()

	deferred := op.Record(gtx.Ops)
	x := actionSize.X - tipDimensions.Size.X
	y := actionSize.Y + gtx.Dp(unit.Dp(3))
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	tipCall.Add(gtx.Ops)
	offset.Pop()
	op.Defer(gtx.Ops, deferred.Stop())
}

func (ui *UI) layoutHTTPActionTooltip(th *material.Theme, gtx layout.Context, tip string) layout.Dimensions {
	theme := ui.fileViewerTheme()
	return fillRoundedBox(gtx, gtx.Dp(unit.Dp(2)), theme.TooltipBg, theme.TooltipBorder, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Caption(th, tip)
			label.Font.Typeface = ui.interfaceTypeface()
			label.TextSize = scaleThemeFontSize(th, 9)
			label.Color = theme.TooltipText
			label.MaxLines = 1
			return label.Layout(gtx)
		})
	})
}

func layoutHTTPCompactButton(th *material.Theme, gtx layout.Context, typeface font.Typeface, click *widget.Clickable, label string, method, primary bool) layout.Dimensions {
	bg := color.NRGBA{}
	textColor := txtColor
	if method {
		textColor = httpMethodColor(strings.TrimSuffix(strings.TrimSpace(label), " ▾"))
	}
	if primary {
		textColor = analyzerAccent
		bg = color.NRGBA{R: 20, G: 44, B: 53, A: 255}
	}
	if click.Hovered() {
		bg = color.NRGBA{R: 28, G: 49, B: 59, A: 255}
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6), Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				textLabel := material.Body2(th, label)
				textLabel.Font.Typeface = typeface
				textLabel.Font.Weight = font.Medium
				textLabel.TextSize = scaleThemeFontSize(th, 10)
				textLabel.Color = textColor
				textLabel.MaxLines = 1
				return textLabel.Layout(gtx)
			})
		})
	})
}

func layoutHTTPSingleLineEditor(th *material.Theme, gtx layout.Context, ui *UI, menuID string, editor *widget.Editor, hint string) layout.Dimensions {
	style := material.Editor(th, editor, hint)
	style.Font.Typeface = ui.mainTypeface()
	style.TextSize = scaleThemeFontSize(th, 11)
	style.Color = txtColor
	style.HintColor = hintColor
	return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.layoutEditorWithContextMenu(th, gtx, menuID, editor, true, style.Layout)
	})
}

func layoutHTTPMultilineEditor(th *material.Theme, gtx layout.Context, ui *UI, menuID string, editor *widget.Editor, scrollbar *widget.Scrollbar, hint string, readOnly bool) layout.Dimensions {
	editor.ReadOnly = readOnly
	style := material.Editor(th, editor, hint)
	style.Font.Typeface = ui.mainTypeface()
	style.TextSize = scaleThemeFontSize(th, 11)
	style.Color = txtColor
	style.HintColor = hintColor
	return fillBgExact(gtx, color.NRGBA{R: 9, G: 15, B: 20, A: 255}, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(7), Right: unit.Dp(7), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			if size.X <= 0 || size.Y <= 0 {
				return layout.Dimensions{Size: size}
			}
			barStyle := httpEditorScrollbarStyle(th, scrollbar)
			barWidth := gtx.Dp(barStyle.Width())
			if barWidth < 1 {
				barWidth = 1
			}
			editorWidth := max(1, size.X-barWidth)
			editorGTX := gtx
			editorGTX.Constraints = layout.Exact(image.Pt(editorWidth, size.Y))
			ui.layoutEditorWithContextMenu(th, editorGTX, menuID, editor, true, style.Layout)

			metrics, scrollable := editorVerticalScrollMetrics(editor)
			if scrollable && scrollbar != nil {
				start := clamp01(float32(metrics.Offset) / float32(metrics.Content))
				end := clamp01(float32(metrics.Offset+metrics.Viewport) / float32(metrics.Content))
				barGTX := gtx
				barGTX.Constraints = layout.Exact(image.Pt(barWidth, size.Y))
				offset := op.Offset(image.Pt(size.X-barWidth, 0)).Push(gtx.Ops)
				barStyle.Layout(barGTX, layout.Vertical, start, end)
				offset.Pop()
				if delta := scrollbar.ScrollDistance(); delta != 0 {
					editorScrollToVerticalOffset(editor, metrics.Offset+int(delta*float32(metrics.Content)))
					gtx.Execute(op.InvalidateCmd{})
				}
			}
			return layout.Dimensions{Size: size}
		})
	})
}

func httpEditorScrollbarStyle(th *material.Theme, scrollbar *widget.Scrollbar) material.ScrollbarStyle {
	style := material.Scrollbar(th, scrollbar)
	style.Track.Color = color.NRGBA{R: 19, G: 34, B: 41, A: 255}
	style.Track.MajorPadding = unit.Dp(1)
	style.Track.MinorPadding = unit.Dp(1)
	style.Indicator.Color = color.NRGBA{R: 73, G: 119, B: 128, A: 255}
	style.Indicator.HoverColor = analyzerAccent
	style.Indicator.MajorMinLen = unit.Dp(18)
	style.Indicator.MinorWidth = unit.Dp(4)
	style.Indicator.CornerRadius = 0
	return style
}

func layoutHTTPPanel(gtx layout.Context, bg, rule color.NRGBA, top, bottom bool, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	size := dimensions.Size
	if size.X < gtx.Constraints.Min.X {
		size.X = gtx.Constraints.Min.X
	}
	if size.X > gtx.Constraints.Max.X {
		size.X = gtx.Constraints.Max.X
	}
	if size.X < 1 || size.Y < 1 {
		call.Add(gtx.Ops)
		return dimensions
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
	if top {
		paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(0, 0, size.X, 1)).Op())
	}
	if bottom {
		paint.FillShape(gtx.Ops, rule, clip.Rect(image.Rect(0, size.Y-1, size.X, size.Y)).Op())
	}
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

func layoutHTTPOuterSurface(gtx layout.Context, bg, border color.NRGBA, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	size := dimensions.Size
	if size.X < gtx.Constraints.Min.X {
		size.X = gtx.Constraints.Min.X
	}
	if size.Y < gtx.Constraints.Min.Y {
		size.Y = gtx.Constraints.Min.Y
	}
	if size.X > gtx.Constraints.Max.X {
		size.X = gtx.Constraints.Max.X
	}
	if size.Y > gtx.Constraints.Max.Y {
		size.Y = gtx.Constraints.Max.Y
	}
	if size.X < 1 {
		size.X = 1
	}
	if size.Y < 1 {
		size.Y = 1
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
	paint.FillShape(gtx.Ops, border, clip.Rect(image.Rect(0, size.Y-1, size.X, size.Y)).Op())
	stack := clip.Rect{Max: size}.Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
	return layout.Dimensions{Size: size}
}
