// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"image/color"
	"strings"
	"sync"
	"time"

	"gioui.org/widget"
	mdicons "golang.org/x/exp/shiny/materialdesign/icons"
)

type filenameIconOption struct {
	key   string
	label string
}

var filenameIconOptions = []filenameIconOption{
	{key: "", label: "Default"},
	{key: fm.FilenameIconRecent, label: "Recent"},
	{key: fm.FilenameIconDocument, label: "Document"},
	{key: fm.FilenameIconBook, label: "Book"},
	{key: fm.FilenameIconCode, label: "Code"},
	{key: fm.FilenameIconTable, label: "Table"},
	{key: fm.FilenameIconImage, label: "Image"},
	{key: fm.FilenameIconVideo, label: "Video"},
	{key: fm.FilenameIconAudio, label: "Audio"},
	{key: fm.FilenameIconArchive, label: "Archive"},
	{key: fm.FilenameIconApp, label: "App"},
	{key: fm.FilenameIconLink, label: "Link"},
	{key: fm.FilenameIconLocked, label: "Locked"},
}

type filePaneFilenameVisual struct {
	color    color.NRGBA
	hasColor bool
	iconKey  string
}

type filePaneFilenameAgeRule struct {
	maxAge time.Duration
	visual filePaneFilenameVisual
}

type filePaneFilenamePermissionRule struct {
	rule   fm.FilenamePermissionRule
	visual filePaneFilenameVisual
}

type filePaneFilenameExtensionRule struct {
	suffix string
	visual filePaneFilenameVisual
}

type filePaneFilenameSizeRule struct {
	rule      fm.FilenameSizeRule
	threshold int64
	visual    filePaneFilenameVisual
}

type filePaneFilenameTheme struct {
	defaultVisual   filePaneFilenameVisual
	ageRules        []filePaneFilenameAgeRule
	permissionRules []filePaneFilenamePermissionRule
	extensionRules  []filePaneFilenameExtensionRule
	sizeRules       []filePaneFilenameSizeRule
}

var filenameRuleIcons struct {
	once     sync.Once
	file     *widget.Icon
	recent   *widget.Icon
	document *widget.Icon
	book     *widget.Icon
	code     *widget.Icon
	table    *widget.Icon
	image    *widget.Icon
	video    *widget.Icon
	audio    *widget.Icon
	archive  *widget.Icon
	app      *widget.Icon
	link     *widget.Icon
	locked   *widget.Icon
}

func mustFilenameIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}

func loadFilenameRuleIcons() {
	filenameRuleIcons.file = mustFilenameIcon(widget.NewIcon(mdicons.EditorInsertDriveFile))
	filenameRuleIcons.recent = mustFilenameIcon(widget.NewIcon(mdicons.ActionSchedule))
	filenameRuleIcons.document = mustFilenameIcon(widget.NewIcon(mdicons.ActionDescription))
	filenameRuleIcons.book = mustFilenameIcon(widget.NewIcon(mdicons.ActionBook))
	filenameRuleIcons.code = mustFilenameIcon(widget.NewIcon(mdicons.EditorModeEdit))
	filenameRuleIcons.table = mustFilenameIcon(widget.NewIcon(mdicons.ImageGridOn))
	filenameRuleIcons.image = mustFilenameIcon(widget.NewIcon(mdicons.EditorInsertPhoto))
	filenameRuleIcons.video = mustFilenameIcon(widget.NewIcon(mdicons.AVMovie))
	filenameRuleIcons.audio = mustFilenameIcon(widget.NewIcon(mdicons.AVLibraryMusic))
	filenameRuleIcons.archive = mustFilenameIcon(widget.NewIcon(mdicons.ContentArchive))
	filenameRuleIcons.app = mustFilenameIcon(widget.NewIcon(mdicons.ActionSettingsApplications))
	filenameRuleIcons.link = mustFilenameIcon(widget.NewIcon(mdicons.ContentLink))
	filenameRuleIcons.locked = mustFilenameIcon(widget.NewIcon(mdicons.ActionLock))
}

func filenameIconLabel(key string) string {
	for _, opt := range filenameIconOptions {
		if opt.key == fm.NormalizeFilenameIcon(key) {
			return opt.label
		}
	}
	return "Default"
}

func filenamePreviewIcon(key string) *widget.Icon {
	if icon := filenameRuleIcon(key); icon != nil {
		return icon
	}
	filenameRuleIcons.once.Do(loadFilenameRuleIcons)
	return filenameRuleIcons.file
}

func filenameRuleIcon(key string) *widget.Icon {
	filenameRuleIcons.once.Do(loadFilenameRuleIcons)
	switch fm.NormalizeFilenameIcon(key) {
	case fm.FilenameIconRecent:
		return filenameRuleIcons.recent
	case fm.FilenameIconDocument:
		return filenameRuleIcons.document
	case fm.FilenameIconBook:
		return filenameRuleIcons.book
	case fm.FilenameIconCode:
		return filenameRuleIcons.code
	case fm.FilenameIconTable:
		return filenameRuleIcons.table
	case fm.FilenameIconImage:
		return filenameRuleIcons.image
	case fm.FilenameIconVideo:
		return filenameRuleIcons.video
	case fm.FilenameIconAudio:
		return filenameRuleIcons.audio
	case fm.FilenameIconArchive:
		return filenameRuleIcons.archive
	case fm.FilenameIconApp:
		return filenameRuleIcons.app
	case fm.FilenameIconLink:
		return filenameRuleIcons.link
	case fm.FilenameIconLocked:
		return filenameRuleIcons.locked
	default:
		return nil
	}
}

func newFilePaneFilenameTheme(cfg *fm.Config) filePaneFilenameTheme {
	if cfg == nil {
		cfg = fm.DefaultConfig()
	}
	theme := filePaneFilenameTheme{
		defaultVisual: parseFilePaneFilenameVisual(cfg.Colors.Filenames.Text, cfg.Colors.Filenames.Icon),
	}
	if len(cfg.Colors.Filenames.AgeRules) > 0 {
		theme.ageRules = make([]filePaneFilenameAgeRule, 0, len(cfg.Colors.Filenames.AgeRules))
		for _, rule := range cfg.Colors.Filenames.AgeRules {
			maxAge, ok := fm.ParseFilenameAge(rule.MaxAge)
			if !ok || maxAge <= 0 {
				continue
			}
			visual := parseFilePaneFilenameVisual(rule.Text, rule.Icon)
			if !visual.hasColor && visual.iconKey == "" {
				continue
			}
			theme.ageRules = append(theme.ageRules, filePaneFilenameAgeRule{
				maxAge: maxAge,
				visual: visual,
			})
		}
	}
	if len(cfg.Colors.Filenames.ExtensionRules) > 0 {
		theme.extensionRules = make([]filePaneFilenameExtensionRule, 0, len(cfg.Colors.Filenames.ExtensionRules))
		for _, rule := range cfg.Colors.Filenames.ExtensionRules {
			suffix := fm.NormalizeFilenameExtension(rule.Extension)
			if suffix == "" {
				continue
			}
			visual := parseFilePaneFilenameVisual(rule.Text, rule.Icon)
			if !visual.hasColor && visual.iconKey == "" {
				continue
			}
			theme.extensionRules = append(theme.extensionRules, filePaneFilenameExtensionRule{
				suffix: suffix,
				visual: visual,
			})
		}
	}
	if len(cfg.Colors.Filenames.SizeRules) > 0 {
		theme.sizeRules = make([]filePaneFilenameSizeRule, 0, len(cfg.Colors.Filenames.SizeRules))
		for _, rule := range cfg.Colors.Filenames.SizeRules {
			size, ok := fm.ParseFilenameSize(rule.Size)
			if !ok {
				continue
			}
			visual := parseFilePaneFilenameVisual(rule.Text, rule.Icon)
			if !visual.hasColor && visual.iconKey == "" {
				continue
			}
			theme.sizeRules = append(theme.sizeRules, filePaneFilenameSizeRule{
				rule:      rule,
				threshold: size,
				visual:    visual,
			})
		}
	}
	if len(cfg.Colors.Filenames.PermissionRules) > 0 {
		theme.permissionRules = make([]filePaneFilenamePermissionRule, 0, len(cfg.Colors.Filenames.PermissionRules))
		for _, rule := range cfg.Colors.Filenames.PermissionRules {
			if fm.NormalizeFilenamePermissions(rule.Permissions) == "" {
				continue
			}
			visual := parseFilePaneFilenameVisual(rule.Text, rule.Icon)
			if !visual.hasColor && visual.iconKey == "" {
				continue
			}
			theme.permissionRules = append(theme.permissionRules, filePaneFilenamePermissionRule{
				rule:   rule,
				visual: visual,
			})
		}
	}
	return theme
}

func parseFilePaneFilenameVisual(textHex, iconKey string) filePaneFilenameVisual {
	visual := filePaneFilenameVisual{
		iconKey: fm.NormalizeFilenameIcon(iconKey),
	}
	if c, ok := fm.ParseHexColor(textHex); ok {
		visual.color = c
		visual.hasColor = true
	}
	return visual
}

func (v filePaneFilenameVisual) merge(next filePaneFilenameVisual) filePaneFilenameVisual {
	if next.hasColor {
		v.color = next.color
		v.hasColor = true
	}
	if next.iconKey != "" {
		v.iconKey = next.iconKey
	}
	return v
}

func (t filePaneFilenameTheme) visualForEntry(entry filesys.Entry, now time.Time) filePaneFilenameVisual {
	if entry.Kind != filesys.EntryFile {
		return filePaneFilenameVisual{}
	}
	visual := t.defaultVisual
	if !entry.ModTime.IsZero() && len(t.ageRules) > 0 {
		age := now.Sub(entry.ModTime)
		if age < 0 {
			age = 0
		}
		for _, rule := range t.ageRules {
			if age <= rule.maxAge {
				visual = visual.merge(rule.visual)
				break
			}
		}
	}
	if len(t.extensionRules) > 0 {
		name := strings.ToLower(entry.Name)
		for _, rule := range t.extensionRules {
			if strings.HasSuffix(name, rule.suffix) {
				visual = visual.merge(rule.visual)
			}
		}
	}
	if len(t.sizeRules) > 0 {
		for _, rule := range t.sizeRules {
			if fm.FilenameSizeMatches(entry.SizeBytes, rule.rule) {
				visual = visual.merge(rule.visual)
			}
		}
	}
	if len(t.permissionRules) > 0 {
		for _, rule := range t.permissionRules {
			if fm.FilenamePermissionMatches(entry.PermOctal, rule.rule) {
				visual = visual.merge(rule.visual)
			}
		}
	}
	return visual
}

func (m *filePaneModel) rebuildFilenameVisuals(now time.Time) {
	if m == nil {
		return
	}
	if len(m.entries) == 0 {
		m.filenameVisuals = nil
		return
	}
	if cap(m.filenameVisuals) < len(m.entries) {
		m.filenameVisuals = make([]filePaneFilenameVisual, len(m.entries))
	} else {
		m.filenameVisuals = m.filenameVisuals[:len(m.entries)]
	}
	for i, entry := range m.entries {
		m.filenameVisuals[i] = m.filenameTheme.visualForEntry(entry, now)
	}
}

func (m *filePaneModel) filenameVisual(row int) filePaneFilenameVisual {
	if m == nil || row < 0 || row >= len(m.filenameVisuals) {
		return filePaneFilenameVisual{}
	}
	return m.filenameVisuals[row]
}
