package ui

import (
	"fmt"
	"hexone/fm"
	uitheme "hexone/ui/theme"
	"hexone/ui/widget/table"
	"image"
	"image/color"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"go.yaml.in/yaml/v4"
)

type settingsModalState struct {
	backdropClick widget.Clickable
	closeClick    widget.Clickable
	saveClick     widget.Clickable
	cancelClick   widget.Clickable

	tabGeneralClick widget.Clickable
	tabViewerClick  widget.Clickable
	tabAssocClick   widget.Clickable
	tabConfigClick  widget.Clickable
	activeTab       string
	navPrevTab      string
	navAnimAt       time.Time
	navHoverKey     string
	navHoverPrev    string
	navHoverAt      time.Time
	navPulseKey     string
	navPulseAt      time.Time

	viewModeFileClick     widget.Clickable
	viewModeHexClick      widget.Clickable
	viewModeCommandClick  widget.Clickable
	viewMode              string
	viewModePrev          string
	viewModeAnimAt        time.Time
	viewModeHoverKey      string
	viewModeHoverPrev     string
	viewModeHoverAt       time.Time
	viewModePulseKey      string
	viewModePulseAt       time.Time
	viewCommandEdit       widget.Editor
	viewShellEdit         widget.Editor
	viewFontSizeEdit      widget.Editor
	viewAssocExtEdit      widget.Editor
	viewAssocAppEdit      widget.Editor
	viewAssocPickClick    widget.Clickable
	viewAssocRemoveClick  widget.Clickable
	viewAssocPickOpen     bool
	viewAssocPickList     layout.List
	viewAssocRowClicks    map[string]*widget.Clickable
	viewAssocEntries      []fm.ViewerAssociation
	viewAssocSavedEntries []fm.ViewerAssociation
	viewAssocLookupExt    string

	footerHoverKey  string
	footerHoverPrev string
	footerHoverAt   time.Time
	footerPulseKey  string
	footerPulseAt   time.Time

	configEdit widget.Editor

	errText       string
	assocInfoText string
}

type viewerAssociationProgram struct {
	AppPath    string
	Extensions []string
	MatchRank  int
}

func (ui *UI) openSettingsModal() {
	if ui == nil {
		return
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}
	st := ui.settingsModal
	if st == nil {
		st = &settingsModalState{activeTab: "viewer"}
		st.viewCommandEdit.SingleLine = true
		st.viewCommandEdit.Submit = false
		st.viewShellEdit.SingleLine = true
		st.viewShellEdit.Submit = false
		st.viewFontSizeEdit.SingleLine = true
		st.viewFontSizeEdit.Submit = false
		st.viewAssocExtEdit.SingleLine = true
		st.viewAssocExtEdit.Submit = false
		st.viewAssocAppEdit.SingleLine = true
		st.viewAssocAppEdit.Submit = false
		st.viewAssocPickList.Axis = layout.Vertical
		st.configEdit.SingleLine = false
		st.configEdit.Submit = false
	}
	st.loadFromConfig(ui.fmCfg)
	ui.settingsModal = st
}

func (ui *UI) closeSettingsModal() {
	ui.settingsModal = nil
}

func (st *settingsModalState) loadFromConfig(cfg *fm.Config) {
	if st == nil || cfg == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Viewer.Mode))
	switch mode {
	case "file", "hex", "command":
	default:
		mode = "file"
	}
	st.viewMode = mode
	st.viewCommandEdit.SetText(cfg.Viewer.Command)
	st.viewShellEdit.SetText(normalizeViewerShellInput(cfg.Viewer.Shell))
	st.viewFontSizeEdit.SetText(formatConfigFloat(cfg.Viewer.FontSizeSp))
	st.viewAssocEntries = append([]fm.ViewerAssociation(nil), fm.FlattenAssociationPrograms(cfg.Associations)...)
	st.viewAssocSavedEntries = append([]fm.ViewerAssociation(nil), st.viewAssocEntries...)
	st.viewAssocPickOpen = false
	st.viewAssocPickList.Position.First = 0
	st.viewAssocPickList.Position.Offset = 0
	st.loadViewerAssociationFields("", "")
	if raw, err := yaml.Marshal(cfg); err == nil {
		st.configEdit.SetText(string(raw))
	}
	st.errText = ""
	st.assocInfoText = ""
}

func (st *settingsModalState) viewerAssocRowClick(key string) *widget.Clickable {
	if st == nil || key == "" {
		return nil
	}
	if st.viewAssocRowClicks == nil {
		st.viewAssocRowClicks = make(map[string]*widget.Clickable)
	}
	if click := st.viewAssocRowClicks[key]; click != nil {
		return click
	}
	click := new(widget.Clickable)
	st.viewAssocRowClicks[key] = click
	return click
}

func (st *settingsModalState) viewerAssociationIndex(ext string) int {
	if st == nil || ext == "" {
		return -1
	}
	for i, assoc := range st.viewAssocEntries {
		if assoc.Extension == ext {
			return i
		}
	}
	return -1
}

func (st *settingsModalState) viewerAssociation(ext string) (fm.ViewerAssociation, bool) {
	if idx := st.viewerAssociationIndex(ext); idx >= 0 {
		return st.viewAssocEntries[idx], true
	}
	return fm.ViewerAssociation{}, false
}

func (st *settingsModalState) viewerSavedAssociation(ext string) (fm.ViewerAssociation, bool) {
	if st == nil || ext == "" {
		return fm.ViewerAssociation{}, false
	}
	for _, assoc := range st.viewAssocSavedEntries {
		if assoc.Extension == ext {
			return assoc, true
		}
	}
	return fm.ViewerAssociation{}, false
}

func (st *settingsModalState) loadViewerAssociationFields(ext, app string) {
	if st == nil {
		return
	}
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(ext))
	st.viewAssocAppEdit.SetText(app)
	st.viewAssocLookupExt = fm.NormalizeViewerAssociationExtension(ext)
}

func (st *settingsModalState) applyPickedViewerAssociation(appPath string) {
	if st == nil {
		return
	}
	targetExt := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	if targetExt == "" {
		st.viewAssocAppEdit.SetText(appPath)
		st.viewAssocLookupExt = ""
		st.viewAssocPickOpen = false
		st.errText = ""
		st.assocInfoText = ""
		return
	}
	prevAssoc, hadAssoc := st.viewerAssociation(targetExt)
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(targetExt))
	st.viewAssocAppEdit.SetText(appPath)
	st.viewAssocLookupExt = targetExt
	if _, err := st.upsertCurrentViewerAssociation(); err != nil {
		st.errText = err.Error()
		st.assocInfoText = ""
		st.viewAssocPickOpen = false
		return
	}
	st.viewAssocPickOpen = false
	st.errText = ""
	if hadAssoc && prevAssoc.AppPath == appPath {
		st.assocInfoText = ""
		return
	}
	action := "added"
	if hadAssoc {
		action = "changed"
	}
	st.assocInfoText = fmt.Sprintf("%s association %s; Save to persist", viewerAssociationDisplayExtension(targetExt), action)
}

func (st *settingsModalState) refreshViewerAssociationDraftInfo(autoApplyExisting bool) {
	if st == nil {
		return
	}
	ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	app := fm.NormalizeViewerAssociationAppPath(st.viewAssocAppEdit.Text())
	st.assocInfoText = ""
	if ext == "" || app == "" {
		return
	}
	existing, ok := st.viewerAssociation(ext)
	if !ok {
		st.assocInfoText = fmt.Sprintf("%s will be added on Save", viewerAssociationDisplayExtension(ext))
		return
	}
	if existing.AppPath == app {
		return
	}
	if autoApplyExisting {
		if _, err := st.upsertCurrentViewerAssociation(); err != nil {
			st.errText = err.Error()
			return
		}
		st.errText = ""
	}
	st.assocInfoText = fmt.Sprintf("%s association changed; Save to persist", viewerAssociationDisplayExtension(ext))
}

func (st *settingsModalState) viewerAssociationNoticeText() string {
	if st == nil {
		return ""
	}
	ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	if ext == "" {
		return ""
	}
	app := fm.NormalizeViewerAssociationAppPath(st.viewAssocAppEdit.Text())
	savedAssoc, savedExists := st.viewerSavedAssociation(ext)
	_, currentExists := st.viewerAssociation(ext)
	switch {
	case savedExists && app == "":
		if !currentExists {
			return fmt.Sprintf("%s association removed; Save to persist", viewerAssociationDisplayExtension(ext))
		}
	case savedExists && app != "" && app != savedAssoc.AppPath:
		return fmt.Sprintf("%s association changed; Save to persist", viewerAssociationDisplayExtension(ext))
	case !savedExists && app != "":
		return fmt.Sprintf("%s will be added on Save", viewerAssociationDisplayExtension(ext))
	case savedExists && !currentExists:
		return fmt.Sprintf("%s association removed; Save to persist", viewerAssociationDisplayExtension(ext))
	}
	return ""
}

func (st *settingsModalState) syncViewerAssociationEditors() {
	if st == nil {
		return
	}
	ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	if ext == st.viewAssocLookupExt {
		return
	}
	st.viewAssocLookupExt = ext
	if assoc, ok := st.viewerAssociation(ext); ok {
		st.viewAssocAppEdit.SetText(assoc.AppPath)
		return
	}
	if strings.TrimSpace(st.viewAssocAppEdit.Text()) == "" {
		st.viewAssocAppEdit.SetText("")
	}
}

func (st *settingsModalState) upsertCurrentViewerAssociation() (string, error) {
	if st == nil {
		return "Add", nil
	}
	assoc, err := parseViewerAssociationFields(st.viewAssocExtEdit.Text(), st.viewAssocAppEdit.Text())
	if err != nil {
		return "Add", err
	}
	action := "Add"
	if idx := st.viewerAssociationIndex(assoc.Extension); idx >= 0 {
		st.viewAssocEntries[idx] = assoc
		action = "Update"
	} else {
		st.viewAssocEntries = append(st.viewAssocEntries, assoc)
	}
	st.viewAssocEntries = fm.NormalizeViewerAssociations(st.viewAssocEntries)
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(assoc.Extension))
	st.viewAssocAppEdit.SetText(assoc.AppPath)
	st.viewAssocLookupExt = assoc.Extension
	return action, nil
}

func (st *settingsModalState) removeCurrentViewerAssociation() bool {
	if st == nil {
		return false
	}
	ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	idx := st.viewerAssociationIndex(ext)
	if idx < 0 {
		return false
	}
	st.viewAssocEntries = append(st.viewAssocEntries[:idx], st.viewAssocEntries[idx+1:]...)
	st.viewAssocExtEdit.SetText(viewerAssociationDisplayExtension(ext))
	st.viewAssocAppEdit.SetText("")
	st.viewAssocLookupExt = ext
	return true
}

func (st *settingsModalState) viewerAssociationPickerPrograms() ([]viewerAssociationProgram, int) {
	if st == nil || len(st.viewAssocEntries) == 0 {
		return nil, 0
	}
	filter := strings.ToLower(strings.TrimSpace(st.viewAssocExtEdit.Text()))
	filter = strings.TrimLeft(filter, ".")
	byApp := make(map[string]*viewerAssociationProgram, len(st.viewAssocEntries))
	for _, assoc := range st.viewAssocEntries {
		app := fm.NormalizeViewerAssociationAppPath(assoc.AppPath)
		if app == "" {
			continue
		}
		group := byApp[app]
		if group == nil {
			group = &viewerAssociationProgram{AppPath: app}
			byApp[app] = group
		}
		dispExt := viewerAssociationDisplayExtension(assoc.Extension)
		group.Extensions = append(group.Extensions, dispExt)
		if filter == "" {
			continue
		}
		lowerExt := strings.ToLower(dispExt)
		rank := 0
		switch {
		case lowerExt == filter:
			rank = 3
		case strings.HasPrefix(lowerExt, filter):
			rank = 2
		case strings.Contains(lowerExt, filter):
			rank = 1
		}
		if rank > group.MatchRank {
			group.MatchRank = rank
		}
	}
	if len(byApp) == 0 {
		return nil, 0
	}
	out := make([]viewerAssociationProgram, 0, len(byApp))
	matchCount := 0
	for _, group := range byApp {
		sort.Strings(group.Extensions)
		if group.MatchRank > 0 {
			matchCount++
		}
		out = append(out, *group)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MatchRank != out[j].MatchRank {
			return out[i].MatchRank > out[j].MatchRank
		}
		iBase := strings.ToLower(filepath.Base(out[i].AppPath))
		jBase := strings.ToLower(filepath.Base(out[j].AppPath))
		if iBase != jBase {
			return iBase < jBase
		}
		return strings.ToLower(out[i].AppPath) < strings.ToLower(out[j].AppPath)
	})
	return out, matchCount
}

func (st *settingsModalState) openViewerAssociationPicker() {
	if st == nil {
		return
	}
	st.viewAssocPickOpen = true
	st.viewAssocPickList.Position.First = 0
	st.viewAssocPickList.Position.Offset = 0
	// Recreate row clickables on every open so stale click state from a prior
	// picker session cannot fire against a different visible row later.
	st.viewAssocRowClicks = nil
}

func (st *settingsModalState) setActiveTab(next string, now time.Time) {
	if st == nil || next == "" || st.activeTab == next {
		return
	}
	st.navPrevTab = st.activeTab
	st.navAnimAt = now
	st.activeTab = next
}

func settingsTabIndex(key string) int {
	switch key {
	case "viewer":
		return 0
	case "associations":
		return 1
	case "general":
		return 2
	case "config":
		return 3
	default:
		return 0
	}
}

func (st *settingsModalState) tabPosition(now time.Time) (float32, bool) {
	if st == nil {
		return 0, false
	}
	current := float32(settingsTabIndex(st.activeTab))
	if st.navPrevTab == "" || st.navAnimAt.IsZero() || st.navPrevTab == st.activeTab {
		return current, false
	}
	elapsed := now.Sub(st.navAnimAt)
	if elapsed >= toolbarAnimDur {
		st.navPrevTab = ""
		st.navAnimAt = time.Time{}
		return current, false
	}
	prev := float32(settingsTabIndex(st.navPrevTab))
	t := smoothstep01(clamp01(float32(elapsed) / float32(toolbarAnimDur)))
	return prev + (current-prev)*t, true
}

func settingsViewerRowLabel(ui *UI, th *material.Theme, txt string, enabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Caption(th, txt)
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
		lbl.Color = hintColor
		if !enabled {
			lbl.Color = color.NRGBA{R: 102, G: 102, B: 102, A: 255}
		}
		return lbl.Layout(gtx)
	}
}

func (st *settingsModalState) tabFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.navPrevTab == "" || st.navAnimAt.IsZero() || st.navPrevTab == st.activeTab {
		if key == st.activeTab {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.navAnimAt)
	if elapsed >= toolbarAnimDur {
		st.navPrevTab = ""
		st.navAnimAt = time.Time{}
		if key == st.activeTab {
			return 1, false
		}
		return 0, false
	}
	t := smoothstep01(float32(elapsed) / float32(toolbarAnimDur))
	if key == st.activeTab {
		return t, true
	}
	if key == st.navPrevTab {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsModalState) setHover(key string, now time.Time) {
	if st == nil || st.navHoverKey == key {
		return
	}
	st.navHoverPrev = st.navHoverKey
	st.navHoverKey = key
	st.navHoverAt = now
}

func (st *settingsModalState) hoverFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.navHoverAt.IsZero() || st.navHoverPrev == st.navHoverKey {
		if st.navHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.navHoverAt)
	if elapsed >= toolbarHoverDur {
		st.navHoverPrev = ""
		st.navHoverAt = time.Time{}
		if st.navHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarHoverDur))
	if key == st.navHoverKey {
		return t, true
	}
	if key == st.navHoverPrev {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsModalState) setPulse(key string, now time.Time) {
	if st == nil || key == "" {
		return
	}
	st.navPulseKey = key
	st.navPulseAt = now
}

func (st *settingsModalState) pulseFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" || st.navPulseKey != key || st.navPulseAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(st.navPulseAt)
	if elapsed >= toolbarClickDur {
		st.navPulseKey = ""
		st.navPulseAt = time.Time{}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarClickDur))
	return 1 - t, true
}

func (st *settingsModalState) setViewMode(next string, now time.Time) {
	if st == nil || next == "" || st.viewMode == next {
		return
	}
	st.viewModePrev = st.viewMode
	st.viewModeAnimAt = now
	st.viewMode = next
}

func (st *settingsModalState) viewModeFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.viewModePrev == "" || st.viewModeAnimAt.IsZero() || st.viewModePrev == st.viewMode {
		if key == st.viewMode {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.viewModeAnimAt)
	if elapsed >= toolbarAnimDur {
		st.viewModePrev = ""
		st.viewModeAnimAt = time.Time{}
		if key == st.viewMode {
			return 1, false
		}
		return 0, false
	}
	t := smoothstep01(float32(elapsed) / float32(toolbarAnimDur))
	if key == st.viewMode {
		return t, true
	}
	if key == st.viewModePrev {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsModalState) setViewModeHover(key string, now time.Time) {
	if st == nil || st.viewModeHoverKey == key {
		return
	}
	st.viewModeHoverPrev = st.viewModeHoverKey
	st.viewModeHoverKey = key
	st.viewModeHoverAt = now
}

func (st *settingsModalState) viewModeHoverFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.viewModeHoverAt.IsZero() || st.viewModeHoverPrev == st.viewModeHoverKey {
		if st.viewModeHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.viewModeHoverAt)
	if elapsed >= toolbarHoverDur {
		st.viewModeHoverPrev = ""
		st.viewModeHoverAt = time.Time{}
		if st.viewModeHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarHoverDur))
	if key == st.viewModeHoverKey {
		return t, true
	}
	if key == st.viewModeHoverPrev {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsModalState) setViewModePulse(key string, now time.Time) {
	if st == nil || key == "" {
		return
	}
	st.viewModePulseKey = key
	st.viewModePulseAt = now
}

func (st *settingsModalState) viewModePulseFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" || st.viewModePulseKey != key || st.viewModePulseAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(st.viewModePulseAt)
	if elapsed >= toolbarClickDur {
		st.viewModePulseKey = ""
		st.viewModePulseAt = time.Time{}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarClickDur))
	return 1 - t, true
}

func (st *settingsModalState) setFooterHover(key string, now time.Time) {
	if st == nil || st.footerHoverKey == key {
		return
	}
	st.footerHoverPrev = st.footerHoverKey
	st.footerHoverKey = key
	st.footerHoverAt = now
}

func (st *settingsModalState) footerHoverFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" {
		return 0, false
	}
	if st.footerHoverAt.IsZero() || st.footerHoverPrev == st.footerHoverKey {
		if st.footerHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	elapsed := now.Sub(st.footerHoverAt)
	if elapsed >= toolbarHoverDur {
		st.footerHoverPrev = ""
		st.footerHoverAt = time.Time{}
		if st.footerHoverKey == key {
			return 1, false
		}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarHoverDur))
	if key == st.footerHoverKey {
		return t, true
	}
	if key == st.footerHoverPrev {
		return 1 - t, true
	}
	return 0, true
}

func (st *settingsModalState) setFooterPulse(key string, now time.Time) {
	if st == nil || key == "" {
		return
	}
	st.footerPulseKey = key
	st.footerPulseAt = now
}

func (st *settingsModalState) footerPulseFill(now time.Time, key string) (float32, bool) {
	if st == nil || key == "" || st.footerPulseKey != key || st.footerPulseAt.IsZero() {
		return 0, false
	}
	elapsed := now.Sub(st.footerPulseAt)
	if elapsed >= toolbarClickDur {
		st.footerPulseKey = ""
		st.footerPulseAt = time.Time{}
		return 0, false
	}
	t := clamp01(float32(elapsed) / float32(toolbarClickDur))
	return 1 - t, true
}

func (ui *UI) saveSettingsModal(now time.Time) error {
	st := ui.settingsModal
	if st == nil {
		return nil
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}
	if st.activeTab == "config" {
		next := fm.DefaultConfig()
		raw := strings.TrimSpace(st.configEdit.Text())
		if raw == "" {
			return fmt.Errorf("config yaml is empty")
		}
		if err := yaml.Unmarshal([]byte(raw), next); err != nil {
			return fmt.Errorf("invalid config yaml: %w", err)
		}
		ui.fmCfg = next
		if err := ui.saveFMConfig(); err != nil {
			return err
		}
		ui.applyConfigRuntime(now)
		st.loadFromConfig(ui.fmCfg)
		return nil
	}

	mode := strings.ToLower(strings.TrimSpace(st.viewMode))
	switch mode {
	case "file", "hex", "command":
	default:
		return fmt.Errorf("viewer mode is invalid")
	}

	cmd := strings.TrimSpace(st.viewCommandEdit.Text())
	if mode == "command" && cmd == "" {
		return fmt.Errorf("viewer command is required in command mode")
	}
	if cmd == "" {
		cmd = "cat {path}"
	}
	shell := normalizeViewerShellInput(st.viewShellEdit.Text())
	switch shell {
	case "auto", "sh", "powershell":
	default:
		return fmt.Errorf("viewer shell must be auto, sh, or powershell")
	}

	viewerFontSize, err := strconv.ParseFloat(strings.TrimSpace(st.viewFontSizeEdit.Text()), 32)
	if err != nil || viewerFontSize < 6 {
		return fmt.Errorf("viewer font size must be at least 6")
	}
	if strings.TrimSpace(st.viewAssocExtEdit.Text()) != "" || strings.TrimSpace(st.viewAssocAppEdit.Text()) != "" {
		if _, err := st.upsertCurrentViewerAssociation(); err != nil {
			return err
		}
	}

	ui.fmCfg.Viewer.Mode = mode
	ui.fmCfg.Viewer.Command = cmd
	ui.fmCfg.Viewer.Shell = shell
	ui.fmCfg.Viewer.FontSizeSp = float32(viewerFontSize)
	ui.fmCfg.Associations = fm.GroupViewerAssociations(st.viewAssocEntries)
	ui.fmCfg.Viewer.Associations = nil
	if err := ui.saveFMConfig(); err != nil {
		return err
	}
	ui.refreshFileViewerNow(now)
	st.loadFromConfig(ui.fmCfg)
	return nil
}

func (ui *UI) layoutSettingsModal(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.settingsModal
	if st == nil {
		return layout.Dimensions{}
	}

	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if ok && ke.State == key.Press && ke.Name == key.NameEscape {
			if st.viewAssocPickOpen {
				st.viewAssocPickOpen = false
				gtx.Execute(op.InvalidateCmd{})
				break
			}
			ui.closeSettingsModal()
			return layout.Dimensions{}
		}
	}

	for st.backdropClick.Clicked(gtx) {
	}
	if st.closeClick.Clicked(gtx) {
		ui.closeSettingsModal()
		return layout.Dimensions{}
	}
	if st.cancelClick.Clicked(gtx) {
		st.setFooterPulse("cancel", gtx.Now)
		ui.closeSettingsModal()
		return layout.Dimensions{}
	}
	if st.saveClick.Clicked(gtx) {
		st.setFooterPulse("save", gtx.Now)
		if err := ui.saveSettingsModal(gtx.Now); err != nil {
			st.errText = err.Error()
		} else {
			ui.closeSettingsModal()
			return layout.Dimensions{}
		}
	}
	if st.tabGeneralClick.Clicked(gtx) {
		st.setActiveTab("general", gtx.Now)
		st.setPulse("general", gtx.Now)
	}
	if st.tabViewerClick.Clicked(gtx) {
		st.setActiveTab("viewer", gtx.Now)
		st.setPulse("viewer", gtx.Now)
	}
	if st.tabAssocClick.Clicked(gtx) {
		st.setActiveTab("associations", gtx.Now)
		st.setPulse("associations", gtx.Now)
	}
	if st.tabConfigClick.Clicked(gtx) {
		st.setActiveTab("config", gtx.Now)
		st.setPulse("config", gtx.Now)
	}
	if st.viewModeFileClick.Clicked(gtx) {
		st.setViewMode("file", gtx.Now)
		st.setViewModePulse("file", gtx.Now)
	}
	if st.viewModeHexClick.Clicked(gtx) {
		st.setViewMode("hex", gtx.Now)
		st.setViewModePulse("hex", gtx.Now)
	}
	if st.viewModeCommandClick.Clicked(gtx) {
		st.setViewMode("command", gtx.Now)
		st.setViewModePulse("command", gtx.Now)
	}

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 140}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		width := gtx.Dp(unit.Dp(760))
		height := gtx.Dp(unit.Dp(460))
		maxW := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(20))
		maxH := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(20))
		if width > maxW {
			width = maxW
		}
		if height > maxH {
			height = maxH
		}
		if width < 520 {
			width = 520
		}
		if height < 320 {
			height = 320
		}

		m := op.Record(gtx.Ops)
		card := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return minHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
				return fillRoundedBox(
					gtx,
					gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
					color.NRGBA{R: 20, G: 20, B: 20, A: 252},
					color.NRGBA{R: 255, G: 255, B: 255, A: 18},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsModalHeader(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsModalBody(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsModalFooter(th, gtx, st)
								}),
							)
						})
					},
				)
			})
		})
		call := m.Stop()

		x := (gtx.Constraints.Max.X - card.Size.X) / 2
		y := (gtx.Constraints.Max.Y - card.Size.Y) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

func (ui *UI) layoutSettingsModalHeader(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, "Global Settings")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.Font.Weight = font.Bold
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 12)
			lbl.Color = txtColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTinyIconModeButton(th, gtx, &st.closeClick, uitheme.CloseIcon(), false)
		}),
	)
}

func fillSettingsNavSegmentBg(gtx layout.Context, bg color.NRGBA, radius int, roundTop, roundBottom bool, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		call.Add(gtx.Ops)
		return dims
	}
	if bg.A != 0 {
		rr := clip.RRect{Rect: image.Rect(0, 0, dims.Size.X, dims.Size.Y)}
		if roundTop {
			rr.NW = radius
			rr.NE = radius
		}
		if roundBottom {
			rr.SW = radius
			rr.SE = radius
		}
		paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	}
	call.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutSettingsNavSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, activeFill, hoverFill, pulseFill float32, roundTop, roundBottom bool) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	dims := c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		activeFill = clamp01(activeFill)
		hoverFill = clamp01(hoverFill)
		pulseFill = clamp01(pulseFill)
		if c.Pressed() && pulseFill < 0.5 {
			pulseFill = 0.5
		}

		baseBlue := color.NRGBA{R: 40, G: 40, B: 40, A: 255}
		hoverDark := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
		hoverLight := color.NRGBA{R: 54, G: 54, B: 54, A: 255}
		pulseCol := color.NRGBA{R: 72, G: 72, B: 72, A: 255}

		bg := mixNRGBA(color.NRGBA{}, baseBlue, activeFill)
		darkMix := hoverFill * (1 - activeFill)
		lightMix := hoverFill * activeFill * 0.25
		bg = mixNRGBA(bg, hoverDark, darkMix)
		bg = mixNRGBA(bg, hoverLight, lightMix)
		bg = mixNRGBA(bg, pulseCol, pulseFill*0.35)

		fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, activeFill)
		fg = mixNRGBA(fg, color.NRGBA{R: 228, G: 228, B: 228, A: 255}, hoverFill*0.75)
		fg = mixNRGBA(fg, color.NRGBA{R: 246, G: 246, B: 246, A: 255}, pulseFill*0.25)
		radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
		return fillSettingsNavSegmentBg(gtx, bg, radius, roundTop, roundBottom, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, label)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Medium
					lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
					lbl.Color = fg
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				})
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func layoutSettingsNavSeparator(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(1))
	if h < 1 {
		h = 1
	}
	w := gtx.Constraints.Max.X
	if w < 1 {
		w = 1
	}
	paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 22}, clip.Rect(image.Rect(0, 0, w, h)).Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (ui *UI) layoutSettingsHSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, activeFill, hoverFill, pulseFill float32, stripH int, roundLeft, roundRight bool) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	dims := fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			activeFill = clamp01(activeFill)
			hoverFill = clamp01(hoverFill)
			pulseFill = clamp01(pulseFill)
			if c.Pressed() && pulseFill < 0.5 {
				pulseFill = 0.5
			}

			baseBlue := color.NRGBA{R: 40, G: 40, B: 40, A: 255}
			hoverDark := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
			hoverLight := color.NRGBA{R: 54, G: 54, B: 54, A: 255}
			pulseCol := color.NRGBA{R: 72, G: 72, B: 72, A: 255}

			bg := mixNRGBA(color.NRGBA{}, baseBlue, activeFill)
			darkMix := hoverFill * (1 - activeFill)
			lightMix := hoverFill * activeFill * 0.25
			bg = mixNRGBA(bg, hoverDark, darkMix)
			bg = mixNRGBA(bg, hoverLight, lightMix)
			bg = mixNRGBA(bg, pulseCol, pulseFill*0.35)

			fg := mixNRGBA(txtColor, color.NRGBA{R: 236, G: 236, B: 236, A: 255}, activeFill)
			fg = mixNRGBA(fg, color.NRGBA{R: 228, G: 228, B: 228, A: 255}, hoverFill*0.75)
			fg = mixNRGBA(fg, color.NRGBA{R: 246, G: 246, B: 246, A: 255}, pulseFill*0.25)

			radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
			return fillSegmentBg(gtx, bg, radius, roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, label)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
						lbl.Color = fg
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					})
				})
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutSettingsNavSliderSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, activeFill, hoverFill, pulseFill float32, stripH int) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	dims := fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			activeFill = clamp01(activeFill)
			hoverFill = clamp01(hoverFill)
			pulseFill = clamp01(pulseFill)
			if c.Pressed() && pulseFill < 0.5 {
				pulseFill = 0.5
			}

			bg := color.NRGBA{}
			bg = mixNRGBA(bg, color.NRGBA{R: 255, G: 255, B: 255, A: 10}, hoverFill*(1-activeFill))
			bg = mixNRGBA(bg, color.NRGBA{R: 255, G: 255, B: 255, A: 18}, pulseFill*0.25)

			fg := mixNRGBA(txtColor, color.NRGBA{R: 238, G: 238, B: 238, A: 255}, clamp01(activeFill*0.8+0.12))
			fg = mixNRGBA(fg, color.NRGBA{R: 232, G: 232, B: 232, A: 255}, hoverFill*0.75)
			fg = mixNRGBA(fg, color.NRGBA{R: 246, G: 246, B: 246, A: 255}, pulseFill*0.25)

			return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, label)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
						lbl.Color = fg
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					})
				})
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (ui *UI) layoutSettingsNavTabs(th *material.Theme, gtx layout.Context, st *settingsModalState, fillViewer, fillAssoc, fillGeneral, fillConfig, hoverViewer, hoverAssoc, hoverGeneral, hoverConfig, pulseViewer, pulseAssoc, pulseGeneral, pulseConfig float32) layout.Dimensions {
	if st == nil {
		return layout.Dimensions{}
	}
	stripH := gtx.Dp(unit.Dp(30))
	if stripH < 1 {
		stripH = 1
	}
	sepH := gtx.Dp(unit.Dp(1))
	if sepH < 1 {
		sepH = 1
	}
	totalH := stripH*4 + sepH*3
	pos, animPos := st.tabPosition(gtx.Now)
	if animPos {
		gtx.Execute(op.InvalidateCmd{})
	}

	return fillBgExact(gtx, color.NRGBA{R: 24, G: 24, B: 24, A: 255}, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, totalH, func(gtx layout.Context) layout.Dimensions {
			w := gtx.Constraints.Max.X
			if w < 1 {
				w = 1
			}
			step := stripH + sepH
			sliderY := int(float32(step) * pos)
			maxSliderY := totalH - stripH
			if maxSliderY < 0 {
				maxSliderY = 0
			}
			if sliderY < 0 {
				sliderY = 0
			}
			if sliderY > maxSliderY {
				sliderY = maxSliderY
			}
			sliderRect := image.Rect(0, sliderY, w, sliderY+stripH)

			innerClip := clip.Rect(image.Rect(0, 0, w, totalH)).Push(gtx.Ops)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 54, G: 54, B: 54, A: 255}, clip.Rect(sliderRect).Op())

			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabViewerClick, "Viewer", fillViewer, hoverViewer, pulseViewer, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabAssocClick, "Associations", fillAssoc, hoverAssoc, pulseAssoc, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabGeneralClick, "General", fillGeneral, hoverGeneral, pulseGeneral, stripH)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutSettingsNavSeparator(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSettingsNavSliderSegment(th, gtx, &st.tabConfigClick, "Config", fillConfig, hoverConfig, pulseConfig, stripH)
				}),
			)
			innerClip.Pop()
			return dims
		})
	})
}

func (ui *UI) layoutSettingsTabContent(th *material.Theme, gtx layout.Context, st *settingsModalState, tab string) layout.Dimensions {
	switch tab {
	case "associations":
		return ui.layoutSettingsAssociationsTab(th, gtx, st)
	case "general":
		lbl := material.Body2(th, "Favorites are managed from the '*' menu. Use the Config tab for full fm.yaml editing.")
		lbl.Font.Typeface = ui.mainTypeface()
		lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 11)
		lbl.Color = hintColor
		return lbl.Layout(gtx)
	case "config":
		return ui.layoutSettingsConfigTab(th, gtx, st)
	default:
		return ui.layoutSettingsViewerTab(th, gtx, st)
	}
}

func (ui *UI) layoutSettingsModalBody(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	fillViewer, animViewer := st.tabFill(gtx.Now, "viewer")
	fillAssoc, animAssoc := st.tabFill(gtx.Now, "associations")
	fillGeneral, animGeneral := st.tabFill(gtx.Now, "general")
	fillConfig, animConfig := st.tabFill(gtx.Now, "config")
	hoverKey := ""
	if st.tabViewerClick.Hovered() {
		hoverKey = "viewer"
	}
	if st.tabAssocClick.Hovered() {
		hoverKey = "associations"
	}
	if st.tabGeneralClick.Hovered() {
		hoverKey = "general"
	}
	if st.tabConfigClick.Hovered() {
		hoverKey = "config"
	}
	st.setHover(hoverKey, gtx.Now)
	hoverViewer, hoverAnimViewer := st.hoverFill(gtx.Now, "viewer")
	hoverAssoc, hoverAnimAssoc := st.hoverFill(gtx.Now, "associations")
	hoverGeneral, hoverAnimGeneral := st.hoverFill(gtx.Now, "general")
	hoverConfig, hoverAnimConfig := st.hoverFill(gtx.Now, "config")
	pulseViewer, pulseAnimViewer := st.pulseFill(gtx.Now, "viewer")
	pulseAssoc, pulseAnimAssoc := st.pulseFill(gtx.Now, "associations")
	pulseGeneral, pulseAnimGeneral := st.pulseFill(gtx.Now, "general")
	pulseConfig, pulseAnimConfig := st.pulseFill(gtx.Now, "config")
	if animViewer || animAssoc || animGeneral || animConfig ||
		hoverAnimViewer || hoverAnimAssoc || hoverAnimGeneral || hoverAnimConfig ||
		pulseAnimViewer || pulseAnimAssoc || pulseAnimGeneral || pulseAnimConfig {
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(146)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsNavTabs(
					th, gtx, st,
					fillViewer, fillAssoc, fillGeneral, fillConfig,
					hoverViewer, hoverAssoc, hoverGeneral, hoverConfig,
					pulseViewer, pulseAssoc, pulseGeneral, pulseConfig,
				)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSettingsTabContent(th, gtx, st, st.activeTab)
		}),
	)
}

func (ui *UI) layoutSettingsViewerTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	fillFile, animFile := st.viewModeFill(gtx.Now, "file")
	fillHex, animHex := st.viewModeFill(gtx.Now, "hex")
	fillCommand, animCommand := st.viewModeFill(gtx.Now, "command")
	hoverModeKey := ""
	if st.viewModeFileClick.Hovered() {
		hoverModeKey = "file"
	}
	if st.viewModeHexClick.Hovered() {
		hoverModeKey = "hex"
	}
	if st.viewModeCommandClick.Hovered() {
		hoverModeKey = "command"
	}
	st.setViewModeHover(hoverModeKey, gtx.Now)
	hoverFile, hoverAnimFile := st.viewModeHoverFill(gtx.Now, "file")
	hoverHex, hoverAnimHex := st.viewModeHoverFill(gtx.Now, "hex")
	hoverCommand, hoverAnimCommand := st.viewModeHoverFill(gtx.Now, "command")
	pulseFile, pulseAnimFile := st.viewModePulseFill(gtx.Now, "file")
	pulseHex, pulseAnimHex := st.viewModePulseFill(gtx.Now, "hex")
	pulseCommand, pulseAnimCommand := st.viewModePulseFill(gtx.Now, "command")
	if animFile || animHex || animCommand || hoverAnimFile || hoverAnimHex || hoverAnimCommand || pulseAnimFile || pulseAnimHex || pulseAnimCommand {
		gtx.Execute(op.InvalidateCmd{})
	}

	commandEnabled := st.viewMode == "command"
	rowLabel := func(txt string, enabled bool) layout.Widget {
		return settingsViewerRowLabel(ui, th, txt, enabled)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel("Mode", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			stripH := gtx.Dp(unit.Dp(22))
			if stripH < 1 {
				stripH = 1
			}
			m := op.Record(gtx.Ops)
			stripDims := fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneControlCornerDp)),
				color.NRGBA{R: 24, G: 24, B: 24, A: 255},
				color.NRGBA{R: 255, G: 255, B: 255, A: 22},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsHSegment(th, gtx, &st.viewModeFileClick, "File", fillFile, hoverFile, pulseFile, stripH, true, false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return toolbarSeparator(gtx, stripH)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsHSegment(th, gtx, &st.viewModeHexClick, "Hex", fillHex, hoverHex, pulseHex, stripH, false, false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return toolbarSeparator(gtx, stripH)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsHSegment(th, gtx, &st.viewModeCommandClick, "Command", fillCommand, hoverCommand, pulseCommand, stripH, false, true)
								}),
							)
						})
					})
				},
			)
			stripCall := m.Stop()
			fillH := stripDims.Size.Y
			if fillH < 1 {
				fillH = stripH + gtx.Dp(unit.Dp(2))
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					stripCall.Add(gtx.Ops)
					return stripDims
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, fillH)}
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("Command template ({filename} {fullpath} {path})", commandEnabled)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			st.viewCommandEdit.ReadOnly = !commandEnabled
			ed := material.Editor(th, &st.viewCommandEdit, "cat {path}")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			if !commandEnabled {
				ed.Color = color.NRGBA{R: 128, G: 136, B: 152, A: 255}
				ed.HintColor = color.NRGBA{R: 95, G: 101, B: 114, A: 255}
			}
			return ui.layoutEditorWithContextMenu(th, gtx, "settings-view-command", &st.viewCommandEdit, commandEnabled, func(gtx layout.Context) layout.Dimensions {
				return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewCommandEdit), commandEnabled, ed.Layout)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("Shell (auto | sh | powershell)", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.viewShellEdit, "auto")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			return ui.layoutEditorWithContextMenu(th, gtx, "settings-view-shell", &st.viewShellEdit, true, func(gtx layout.Context) layout.Dimensions {
				return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewShellEdit), true, ed.Layout)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("Viewer font size (sp)", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.viewFontSizeEdit, "13")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			return ui.layoutEditorWithContextMenu(th, gtx, "settings-view-font", &st.viewFontSizeEdit, true, func(gtx layout.Context) layout.Dimensions {
				return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewFontSizeEdit), true, ed.Layout)
			})
		}),
	)
}

func (ui *UI) layoutSettingsAssociationsTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	for {
		ev, ok := st.viewAssocExtEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
			st.refreshViewerAssociationDraftInfo(false)
		}
	}
	for {
		ev, ok := st.viewAssocAppEdit.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			st.errText = ""
			st.refreshViewerAssociationDraftInfo(true)
		}
	}
	st.syncViewerAssociationEditors()
	for st.viewAssocRemoveClick.Clicked(gtx) {
		ext := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
		if ext == "" {
			st.errText = "association extension is required"
			continue
		}
		if !st.removeCurrentViewerAssociation() {
			st.errText = "no association set for " + viewerAssociationDisplayExtension(ext)
			continue
		}
		st.errText = ""
		st.assocInfoText = ""
		st.viewAssocPickOpen = false
	}
	for st.viewAssocPickClick.Clicked(gtx) {
		st.viewAssocPickOpen = !st.viewAssocPickOpen
		if st.viewAssocPickOpen {
			st.openViewerAssociationPicker()
		}
	}

	currentAssocExt := fm.NormalizeViewerAssociationExtension(st.viewAssocExtEdit.Text())
	_, currentAssocExists := st.viewerAssociation(currentAssocExt)
	pickerPrograms, pickerMatchCount := st.viewerAssociationPickerPrograms()
	totalAssocCount := len(st.viewAssocEntries)
	statusText := "No association"
	statusColor := hintColor
	switch {
	case currentAssocExt == "":
		if totalAssocCount == 0 {
			statusText = "No associations"
		} else {
			statusText = fmt.Sprintf("%d associations saved", totalAssocCount)
		}
	case currentAssocExists:
		statusText = "Associated"
		statusColor = color.NRGBA{R: 152, G: 205, B: 152, A: 255}
	case totalAssocCount > 0:
		statusText = fmt.Sprintf("New association | %d saved", totalAssocCount)
	default:
		statusText = "New association"
	}
	rowLabel := func(txt string, enabled bool) layout.Widget {
		return settingsViewerRowLabel(ui, th, txt, enabled)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel("F4 app override (local files only)", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "F4 uses this app path when set, otherwise it falls back to the OS association. Browse shows one row per app and picking one reuses it for the current extension.")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
			lbl.Color = hintColor
			lbl.MaxLines = 2
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("Extension", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, gtx.Dp(unit.Dp(108)), func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(th, &st.viewAssocExtEdit, "mp3")
						ed.Font.Typeface = ui.mainTypeface()
						ed.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
						ed.Color = txtColor
						ed.HintColor = hintColor
						return ui.layoutEditorWithContextMenu(th, gtx, "settings-view-assoc-ext", &st.viewAssocExtEdit, true, func(gtx layout.Context) layout.Dimensions {
							return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewAssocExtEdit), true, ed.Layout)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTinyModeButton(th, gtx, ui.mainTypeface(), &st.viewAssocPickClick, "Browse", st.viewAssocPickOpen)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, statusText)
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
					lbl.Color = statusColor
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !st.viewAssocPickOpen {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSettingsViewerAssocPicker(th, gtx, st, pickerPrograms, pickerMatchCount)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(rowLabel("App path", true)),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &st.viewAssocAppEdit, `C:\Program Files\App\player.exe`)
					ed.Font.Typeface = ui.mainTypeface()
					ed.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
					ed.Color = txtColor
					ed.HintColor = hintColor
					return ui.layoutEditorWithContextMenu(th, gtx, "settings-view-assoc-app", &st.viewAssocAppEdit, true, func(gtx layout.Context) layout.Dimensions {
						return layoutNeutralEditorBox(gtx, gtx.Focused(&st.viewAssocAppEdit), true, ed.Layout)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !currentAssocExists {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layoutTinyIconModeButton(th, gtx, &st.viewAssocRemoveClick, uitheme.CloseIcon(), false)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			infoText := st.viewerAssociationNoticeText()
			if infoText == "" {
				infoText = st.assocInfoText
			}
			if infoText == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, infoText)
				lbl.Font.Typeface = ui.mainTypeface()
				lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
				lbl.Color = color.NRGBA{R: 152, G: 205, B: 152, A: 255}
				lbl.MaxLines = 2
				lbl.Truncator = "..."
				return lbl.Layout(gtx)
			})
		}),
	)
}

func (ui *UI) layoutSettingsViewerAssocPicker(th *material.Theme, gtx layout.Context, st *settingsModalState, programs []viewerAssociationProgram, matchCount int) layout.Dimensions {
	gtx2 := gtx
	maxH := gtx.Dp(unit.Dp(168))
	if gtx2.Constraints.Max.Y > maxH {
		gtx2.Constraints.Max.Y = maxH
	}
	return fillRoundedBox(
		gtx2,
		gtx2.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 18, G: 22, B: 30, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			if len(programs) == 0 {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, "No saved apps")
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
					lbl.Color = hintColor
					return lbl.Layout(gtx)
				})
			}
			var picked *viewerAssociationProgram
			currentAppPath := fm.NormalizeViewerAssociationAppPath(st.viewAssocAppEdit.Text())
			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if matchCount <= 0 || matchCount >= len(programs) {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(6), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, fmt.Sprintf("%d similar apps, then all saved apps", matchCount))
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
						lbl.Color = hintColor
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return st.viewAssocPickList.Layout(gtx, len(programs), func(gtx layout.Context, i int) layout.Dimensions {
						program := programs[i]
						click := st.viewerAssocRowClick(program.AppPath)
						// Clickable.Layout drains queued clicks before painting, so row
						// actions must be drained before Layout and then applied once
						// after the list finishes to avoid mid-layout state changes.
						for click.Clicked(gtx) {
							if picked == nil {
								programCopy := program
								picked = &programCopy
							}
						}
						selected := strings.EqualFold(program.AppPath, currentAppPath)
						dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							bg := color.NRGBA{A: 0}
							if selected {
								bg = color.NRGBA{R: 80, G: 120, B: 220, A: 45}
							} else if click.Hovered() {
								bg = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
							}
							return fillBgExact(gtx, bg, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											usedBy := strings.Join(program.Extensions, ", ")
											lbl := material.Body2(th, filepath.Base(program.AppPath)+" used by "+usedBy)
											lbl.Font.Typeface = ui.mainTypeface()
											lbl.Font.Weight = font.Medium
											lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
											lbl.Color = txtColor
											lbl.MaxLines = 1
											lbl.Truncator = "..."
											return lbl.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											lbl := material.Caption(th, program.AppPath)
											lbl.Font.Typeface = ui.mainTypeface()
											lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 8)
											lbl.Color = hintColor
											lbl.MaxLines = 1
											lbl.Truncator = "..."
											return lbl.Layout(gtx)
										}),
									)
								})
							})
						})
						return dims
					})
				}),
			)
			if picked != nil {
				st.applyPickedViewerAssociation(picked.AppPath)
			}
			return dims
		},
	)
}

func (ui *UI) layoutSettingsModalFooter(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	hoverFooterKey := ""
	if st.cancelClick.Hovered() {
		hoverFooterKey = "cancel"
	}
	if st.saveClick.Hovered() {
		hoverFooterKey = "save"
	}
	st.setFooterHover(hoverFooterKey, gtx.Now)
	hoverCancel, hoverAnimCancel := st.footerHoverFill(gtx.Now, "cancel")
	hoverSave, hoverAnimSave := st.footerHoverFill(gtx.Now, "save")
	pulseCancel, pulseAnimCancel := st.footerPulseFill(gtx.Now, "cancel")
	pulseSave, pulseAnimSave := st.footerPulseFill(gtx.Now, "save")
	if hoverAnimCancel || hoverAnimSave || pulseAnimCancel || pulseAnimSave {
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if st.errText == "" {
				return layout.Dimensions{}
			}
			lbl := material.Caption(th, st.errText)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
			lbl.Color = color.NRGBA{R: 255, G: 170, B: 170, A: 255}
			lbl.MaxLines = 2
			lbl.Truncator = "..."
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			stripH := gtx.Dp(unit.Dp(22))
			if stripH < 1 {
				stripH = 1
			}
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneControlCornerDp)),
				color.NRGBA{R: 24, G: 24, B: 24, A: 255},
				color.NRGBA{R: 255, G: 255, B: 255, A: 22},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsHSegment(th, gtx, &st.cancelClick, "Cancel", 0, hoverCancel, pulseCancel, stripH, true, false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return toolbarSeparator(gtx, stripH)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSettingsHSegment(th, gtx, &st.saveClick, "Save", 0, hoverSave, pulseSave, stripH, false, true)
								}),
							)
						})
					})
				},
			)
		}),
	)
}

func (ui *UI) layoutSettingsConfigTab(th *material.Theme, gtx layout.Context, st *settingsModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "Full fm.yaml (all config fields)")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 9)
			lbl.Color = hintColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &st.configEdit, "")
			ed.Font.Typeface = ui.mainTypeface()
			ed.TextSize = scaleModalThemeFontSize(th, ui.fmCfg, 10)
			ed.Color = txtColor
			ed.HintColor = hintColor
			return ui.layoutEditorWithContextMenu(th, gtx, "settings-config", &st.configEdit, true, func(gtx layout.Context) layout.Dimensions {
				return layoutNeutralEditorBox(gtx, gtx.Focused(&st.configEdit), true, ed.Layout)
			})
		}),
	)
}

func formatConfigFloat(v float32) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}

func viewerAssociationDisplayExtension(ext string) string {
	ext = strings.TrimSpace(ext)
	ext = strings.TrimPrefix(ext, ".")
	return ext
}

func parseViewerAssociationFields(extRaw, appRaw string) (fm.ViewerAssociation, error) {
	ext := fm.NormalizeViewerAssociationExtension(extRaw)
	if ext == "" {
		return fm.ViewerAssociation{}, fmt.Errorf("association extension is invalid")
	}
	app := fm.NormalizeViewerAssociationAppPath(appRaw)
	if app == "" {
		return fm.ViewerAssociation{}, fmt.Errorf("association app path is required")
	}
	return fm.ViewerAssociation{
		Extension: ext,
		AppPath:   app,
	}, nil
}

func normalizeViewerShellInput(raw string) string {
	shell := strings.ToLower(strings.TrimSpace(raw))
	switch shell {
	case "", "auto":
		return "auto"
	case "sh":
		return "sh"
	case "pwsh", "powershell":
		return "powershell"
	default:
		return shell
	}
}

func (ui *UI) applyConfigRuntime(now time.Time) {
	if ui == nil {
		return
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}
	ui.fileKeys = newFileKeyMap(ui.fmCfg)
	ui.typeface = font.Typeface(ui.fmCfg.Font.Typeface)
	ui.textSize = fontSizeFromConfig(ui.fmCfg)
	if ui.tab2State != nil {
		ui.tab2State.typeface = ui.mainTypeface()
	}
	ui.reloadFilePanesForConfig(now)
	ui.refreshFileViewerNow(now)
}

func (ui *UI) reloadFilePanesForConfig(now time.Time) {
	if ui == nil || ui.fmCfg == nil || len(ui.filePanes) == 0 {
		return
	}
	active := ui.activeFilePane
	next := make([]*filePaneState, len(ui.filePanes))
	for i, old := range ui.filePanes {
		if old == nil {
			continue
		}
		dir := old.dir
		reloadDir := dir
		baseDir := dir
		var reloadRemote *paneSSHSession
		localBeforeRemote := old.localDirBeforeRemote
		if old.remoteConnected() && old.remote != nil {
			reloadRemote = old.remote.clone()
			if strings.TrimSpace(baseDir) == "" || strings.HasPrefix(baseDir, "/") {
				baseDir = strings.TrimSpace(localBeforeRemote)
			}
			if strings.TrimSpace(baseDir) == "" {
				baseDir = "."
			}
			if strings.TrimSpace(reloadDir) == "" {
				reloadDir = reloadRemote.homeDir()
			}
			reloadDir = path.Clean(reloadDir)
			if reloadDir == "" || reloadDir == "." {
				reloadDir = "/"
			}
		}

		selectedPath := ""
		mode := table.ModeFull
		if old.table != nil {
			mode = old.table.Mode
		}
		if sel := old.selectedEntry(); sel != nil {
			selectedPath = sel.Path
		}

		pane := newFilePaneState(baseDir, ui.fmCfg)
		pane.table.SetMode(mode)
		idx := i
		pane.table.OnClick = func(row int) {
			_ = row
			ui.setActiveFilePane(idx)
		}
		pane.table.OnDoubleClick = func(row int) {
			ui.queueFilePaneOpen(idx, row)
		}
		pane.table.OnActivate = func(row int) {
			ui.queueFilePaneOpen(idx, row)
		}
		if reloadRemote != nil {
			pane.remote = reloadRemote
			pane.localDirBeforeRemote = localBeforeRemote
			if pane.localDirBeforeRemote == "" {
				pane.localDirBeforeRemote = baseDir
			}
			if err := pane.load(reloadDir); err != nil {
				pane.setNotice("remote reload failed: "+err.Error(), now)
			} else {
				pane.applySelection(selectedPath, "", pane.table.Selected)
			}
		} else {
			startLocalPaneLoad(pane, filepath.Clean(dir), selectedPath, "", pane.table.Selected)
		}
		if old.remote != nil {
			old.remote.close()
			old.remote = nil
		}
		next[i] = pane
	}
	ui.filePanes = next
	if active < 0 {
		active = 0
	}
	if active >= len(ui.filePanes) {
		active = len(ui.filePanes) - 1
	}
	ui.setActiveFilePane(active)
}
