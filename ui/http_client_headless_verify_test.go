// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build uiverify

package ui

import (
	"fmt"
	"hexone/fm"
	"hexone/httpclient"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestHeadlessHTTPClient(t *testing.T) {
	outDir := os.Getenv("HTTP_UI_VERIFY_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	scale := 1
	if value := os.Getenv("HTTP_UI_VERIFY_SCALE"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			scale = parsed
		}
	}
	width, height := 1200*scale, 700*scale
	win, err := headless.NewWindow(width, height)
	if err != nil {
		t.Fatalf("headless window: %v", err)
	}
	defer win.Release()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(pathHeaderVerifyFontCollection(t)))
	ui := NewUI(fm.DefaultConfig())
	ui.httpCollectionsPath = filepath.Join(t.TempDir(), "hexone-http.yaml")
	ui.Tabs.Value = "tab3"
	router := new(input.Router)
	st := ui.ensureHTTPClientState()
	st.selectRequest(httpRequestRef{collection: 0, folder: 0, request: 1})
	var focusBeforeRender event.Tag

	render := func(name string) {
		var screenshot *image.RGBA
		base := time.Now()
		for frame := range 4 {
			var ops op.Ops
			gtx := layout.Context{
				Ops:         &ops,
				Metric:      unit.Metric{PxPerDp: float32(scale), PxPerSp: float32(scale)},
				Constraints: layout.Exact(image.Pt(width, height)),
				Now:         base.Add(time.Duration(frame) * 50 * time.Millisecond),
				Source:      router.Source(),
			}
			if frame == 0 && focusBeforeRender != nil {
				event.Op(gtx.Ops, focusBeforeRender)
				gtx.Execute(key.FocusCmd{Tag: focusBeforeRender})
			}
			ui.Layout(th, gtx)
			router.Frame(&ops)
			if err := win.Frame(&ops); err != nil {
				t.Fatalf("render frame: %v", err)
			}
			screenshot = image.NewRGBA(image.Rect(0, 0, width, height))
			if err := win.Screenshot(screenshot); err != nil {
				t.Fatalf("screenshot: %v", err)
			}
		}
		focusBeforeRender = nil
		path := filepath.Join(outDir, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, screenshot); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}

	render("http-client-workbench.png")
	if st.requestSplitY < height/3 {
		t.Fatalf("request/response split y=%d is above the response rail", st.requestSplitY)
	}
	selectedRequest := st.currentRequest()
	originalMethod := selectedRequest.Method
	selectedRequest.Method = "PATCH"
	st.method = "PATCH"
	render("http-client-patch-method.png")
	selectedRequest.Method = "DELETE"
	st.method = "DELETE"
	render("http-client-delete-method.png")
	selectedRequest.Method = "OPTIONS"
	st.method = "OPTIONS"
	render("http-client-options-method.png")
	selectedRequest.Method = originalMethod
	st.method = originalMethod
	var selectedRequestRow httpCollectionRow
	for _, row := range st.collectionRows() {
		if row.kind == "request" && row.ref == st.selected {
			selectedRequestRow = row
			break
		}
	}
	if !st.beginTreeRename(selectedRequestRow) {
		t.Fatal("could not start request rename verification")
	}
	render("http-client-request-rename.png")
	originalRequestName, _ := st.treeRenameName(selectedRequestRow.kind, selectedRequestRow.ref)
	st.treeRenameEd.SetText("discarded by selection")
	healthRowY := 104
	healthRowX := 150
	if scale > 1 {
		healthRowX = 300
		healthRowY = 212
	}
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(healthRowX), float32(healthRowY))},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(float32(healthRowX), float32(healthRowY))},
	)
	render("http-client-rename-cancel-selection.png")
	if st.treeRenameActive || st.selected == selectedRequestRow.ref {
		t.Fatalf("different request click left rename active=%t selected=%#v", st.treeRenameActive, st.selected)
	}
	if got, _ := st.treeRenameName(selectedRequestRow.kind, selectedRequestRow.ref); got != originalRequestName {
		t.Fatalf("different request click committed name %q want %q", got, originalRequestName)
	}
	st.selectRequest(selectedRequestRow.ref)
	collectionRow := st.collectionRows()[0]
	if !st.beginTreeRename(collectionRow) {
		t.Fatal("could not start collection rename verification")
	}
	render("http-client-collection-rename.png")
	st.finishTreeRename(false)
	var folderRow httpCollectionRow
	for _, row := range st.collectionRows() {
		if row.kind == "folder" {
			folderRow = row
			break
		}
	}
	if !st.beginTreeRename(folderRow) {
		t.Fatal("could not start folder rename verification")
	}
	render("http-client-folder-rename.png")
	st.finishTreeRename(false)
	for index, row := range st.collectionRows() {
		if row.kind == "request" && row.ref == selectedRequestRow.ref {
			st.treeMenuRow = row
			st.treeMenuRowIndex = index
			break
		}
	}
	st.treeMenuOpen = true
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyTreeMenu, index: 1}
	focusBeforeRender = &st.treeKeyTag
	render("http-client-tree-context-menu.png")
	st.treeMenuOpen = false
	if !st.activateRequestTab(0) {
		t.Fatal("could not activate first request tab for seam verification")
	}
	render("http-client-first-tab-seam.png")
	if !st.activateRequestTab(1) {
		t.Fatal("could not restore second request tab after seam verification")
	}
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(240*scale), float32(41*scale))})
	render("http-client-action-tooltip.png")
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(12*scale), float32(180*scale))})
	st.methodMenuOpen = true
	render("http-client-method-menu.png")
	st.methodMenuOpen = false
	st.envMenuOpen = true
	render("http-client-environment-menu.png")
	st.envMenuOpen = false
	if !st.openEnvironmentEditor(0, true) {
		t.Fatal("could not open environment editor verification")
	}
	st.envEditorVarsEd.SetText("base_url=http://localhost:8080\ntoken=local-secret\nregion=eu-central")
	st.environmentAuth.set(httpclient.Auth{Type: httpclient.AuthAPIKey, Key: "X-API-Key", Value: "{{token}}", In: httpclient.AuthInHeader}, false)
	render("http-client-environment-editor.png")
	st.envEditorFocus = ""
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyEnvAuthTab, index: 3}
	focusBeforeRender = &st.treeKeyTag
	render("http-client-environment-editor-auth-focus.png")
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyEnvActions, index: 1}
	focusBeforeRender = &st.treeKeyTag
	render("http-client-environment-editor-actions-focus.png")
	st.closeEnvironmentEditor()
	st.response = httpclient.Response{
		StatusCode: 201,
		Status:     "201 Created",
		Headers: []httpclient.KeyValue{
			{Name: "Content-Type", Value: "application/json"},
			{Name: "X-Request-ID", Value: "req-1042"},
		},
		Body:     []byte(`{"id":1042,"name":"Ada Lovelace","status":"active"}`),
		Duration: 42 * time.Millisecond,
		Size:     364,
	}
	st.status = "201 Created · 42 ms · 364 B"
	st.updateResponseEditor()
	render("http-client-response.png")
	st.detailMode = httpDetailAuth
	st.requestAuth.set(httpclient.Auth{Type: httpclient.AuthBearer, Token: "{{token}}"}, true)
	render("http-client-auth-bearer.png")
	st.requestAuth.set(httpclient.Auth{Type: httpclient.AuthBasic, Username: "{{user}}", Password: "{{password}}"}, true)
	render("http-client-auth-basic.png")
	st.requestAuth.set(httpclient.Auth{Type: httpclient.AuthAPIKey, Key: "X-API-Key", Value: "{{api_key}}", In: httpclient.AuthInHeader}, true)
	render("http-client-auth-api-key.png")
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyRequestAuthTab, index: httpChoiceIndex(httpAuthTypes, httpclient.AuthAPIKey)}
	focusBeforeRender = &st.treeKeyTag
	render("http-client-auth-api-key-keyboard-focus.png")
	st.file.Environments[st.environment].Auth = httpclient.Auth{Type: httpclient.AuthBearer, Token: "{{token}}"}
	st.requestAuth.set(httpclient.Auth{Type: httpclient.AuthInherit}, true)
	render("http-client-auth-inherited.png")
	st.detailMode = httpDetailHeaders
	if !st.addCollection() || !st.addFolderToSelection() || !st.addRequestToSelection() {
		t.Fatal("could not build collection-action verification hierarchy")
	}
	render("http-client-collection-actions.png")
	st.finishTreeRename(false)
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyEnvironmentGroup, index: 0}
	focusBeforeRender = &st.treeKeyTag
	render("http-client-keyboard-environment-focus.png")
	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	render("http-client-keyboard-environment-add-focus.png")
	if st.keyboardTarget != (httpKeyboardTarget{kind: httpKeyEnvironmentGroup, index: 1}) {
		t.Fatalf("environment group Right target=%#v, want add environment", st.keyboardTarget)
	}
	st.selectKeyboardTreeRow(st.collectionRows()[0])
	focusBeforeRender = &st.urlEd
	render("http-client-tree-keyboard-url-focus.png")
	router.Queue(key.Event{Name: key.NameTab, State: key.Press})
	render("http-client-keyboard-send-focus.png")
	if st.keyboardTarget != (httpKeyboardTarget{kind: httpKeyCommandGroup, index: 0}) {
		t.Fatalf("keyboard Tab target=%#v, want Send", st.keyboardTarget)
	}
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyMethod}
	focusBeforeRender = &st.treeKeyTag
	render("http-client-keyboard-method-focus.png")
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyRequestTab, index: 0}
	st.activateRequestTab(0)
	focusBeforeRender = &st.treeKeyTag
	render("http-client-keyboard-request-tab-focus.png")
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyResponseTab, index: 1}
	st.responseMode = httpResponseRaw
	st.updateResponseEditor()
	focusBeforeRender = &st.treeKeyTag
	render("http-client-keyboard-response-tab-focus.png")
	st.selectKeyboardTreeRow(st.collectionRows()[0])
	st.keyboardTarget = httpKeyboardTarget{kind: httpKeyTree}
	focusBeforeRender = &st.treeKeyTag
	render("http-client-tree-keyboard-root.png")
	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	render("http-client-tree-keyboard-next-row.png")
	if st.treeSelected.kind != "folder" || st.treeSelected.ref.collection != 0 {
		t.Fatalf("keyboard Down selected %#v, want first folder", st.treeSelected)
	}
	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	render("http-client-tree-keyboard-inside.png")
	if st.treeSelected.kind != "request" || st.treeSelected.ref.collection != 0 {
		t.Fatalf("second keyboard Down selected %#v, want first request", st.treeSelected)
	}

	for index := 0; index < 36; index++ {
		st.file.Collections[0].Folders[0].Requests = append(st.file.Collections[0].Folders[0].Requests, httpclient.Request{
			Name:   fmt.Sprintf("Generated request %02d", index+1),
			Method: "GET",
			URL:    "{{base_url}}/generated",
		})
	}
	var body strings.Builder
	body.WriteString("{\n")
	for index := 0; index < 48; index++ {
		fmt.Fprintf(&body, "  \"field_%02d\": \"value_%02d\",\n", index, index)
	}
	body.WriteString("  \"done\": true\n}")
	st.bodyEd.SetText(body.String())
	st.detailMode = httpDetailBody
	st.response.Body = []byte(body.String())
	st.updateResponseEditor()
	st.collectionWidth = 360
	st.requestRatio = 0.30
	render("http-client-resized-scrollbars.png")

	ui.functionBarToolsOpen = true
	ui.functionBarToolsOpenedAt = time.Now().Add(-time.Second)
	ui.functionBarToolsSelected = 4
	render("http-client-tools-menu.png")
}
