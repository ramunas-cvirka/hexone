// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package fm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestNameCompactBackwardCompatibility(t *testing.T) {
	raw := `
name_compact:
  keep_start_chars: 7
  min_head: 9
  min_tail: 5
  approx_char_px: 11
  marker: __
columns:
  permission_format: symbolic
  perm_min_width_dp: 10
  size_min_width_dp: 11
  date_min_width_dp: 12
date_formats:
  - Jan 02
  - 01-02
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if cfg.NameCompact.KeepStartChars != 7 {
		t.Fatalf("KeepStartChars=%d, want 7", cfg.NameCompact.KeepStartChars)
	}
	if cfg.NameCompact.Marker != "__" {
		t.Fatalf("Marker=%q, want __", cfg.NameCompact.Marker)
	}

	if got, want := SizeMinWidthDp(cfg), defaultApproxCharPx*5+8+8; got != want {
		t.Fatalf("SizeMinWidthDp=%d, want %d", got, want)
	}
	if got, want := PermMinWidthDp(cfg), defaultApproxCharPx*9+12+8; got != want {
		t.Fatalf("PermMinWidthDp=%d, want %d", got, want)
	}
	if got, want := DateMinWidthDp(cfg), defaultApproxCharPx*5+16+8; got != want {
		t.Fatalf("DateMinWidthDp=%d, want %d", got, want)
	}
}

func TestMarshalConfigOmitsInternalFields(t *testing.T) {
	cfg := DefaultConfig()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	out := string(data)

	for _, disallowed := range []string{
		"approx_char_px:",
		"min_head:",
		"min_tail:",
		"perm_min_width_dp:",
		"size_min_width_dp:",
		"date_min_width_dp:",
		"name_width_dp:",
		"name_min_width_dp:",
		"perm_width_dp:",
		"full_pad_dp:",
		"size_width_dp:",
		"date_width_dp:",
		"brief_width_dp:",
		"brief_gap_dp:",
	} {
		if strings.Contains(out, disallowed) {
			t.Fatalf("serialized config still contains %q:\n%s", disallowed, out)
		}
	}
	if !strings.Contains(out, "keep_start_chars:") {
		t.Fatalf("serialized config missing keep_start_chars:\n%s", out)
	}
	if !strings.Contains(out, "full_chars:") || !strings.Contains(out, "brief_chars:") {
		t.Fatalf("serialized config missing character-based column widths:\n%s", out)
	}
	if strings.Contains(out, "name_chars:") {
		t.Fatalf("serialized config should use full_chars instead of name_chars:\n%s", out)
	}
	if strings.Contains(out, "key_bindings:") {
		t.Fatalf("serialized config should not contain legacy key_bindings block:\n%s", out)
	}
}

func TestLegacyColumnWidthsMigrateToChars(t *testing.T) {
	raw := `
columns:
  name_width_dp: 180
  name_min_width_dp: 52
  perm_width_dp: 92
  full_pad_dp: 4
  size_width_dp: 92
  date_width_dp: 128
  brief_width_dp: 180
  brief_gap_dp: 4
  show_permissions: false
  permission_format: octal
  full_drop_priority:
    - size
    - date
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.Columns.NameChars, float32(20); got != want {
		t.Fatalf("Columns.NameChars=%v, want %v", got, want)
	}
	if got, want := cfg.Columns.BriefChars, float32(20); got != want {
		t.Fatalf("Columns.BriefChars=%v, want %v", got, want)
	}
	if cfg.Columns.ShowPermissions {
		t.Fatal("Columns.ShowPermissions should preserve explicit false")
	}
	if cfg.Columns.PermissionFormat != "octal" {
		t.Fatalf("Columns.PermissionFormat=%q, want octal", cfg.Columns.PermissionFormat)
	}

	out := string(mustMarshalConfig(t, cfg))
	if strings.Contains(out, "name_width_dp:") || strings.Contains(out, "brief_width_dp:") {
		t.Fatalf("serialized config should not contain legacy dp widths:\n%s", out)
	}
	if !strings.Contains(out, "full_chars: 20") || !strings.Contains(out, "brief_chars: 20") {
		t.Fatalf("serialized config missing migrated character widths:\n%s", out)
	}
}

func TestLegacyNameCharsMigratesToFullChars(t *testing.T) {
	raw := `
columns:
  name_chars: 24
  brief_chars: 16
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.Columns.NameChars, float32(24); got != want {
		t.Fatalf("Columns.NameChars=%v, want %v", got, want)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "full_chars: 24") {
		t.Fatalf("serialized config missing full_chars:\n%s", out)
	}
	if strings.Contains(out, "name_chars:") {
		t.Fatalf("serialized config should not contain legacy name_chars:\n%s", out)
	}
}

func TestLegacyFontBlockMigratesToGeneral(t *testing.T) {
	raw := `
font:
  typeface: Consolas
  size_sp: 16
general:
  dim_inactive_panes: true
viewer:
  mode: file
  shell: auto
  command: cat {path}
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if cfg.General.Typeface != "Consolas" {
		t.Fatalf("General.Typeface=%q, want Consolas", cfg.General.Typeface)
	}
	if cfg.General.FontSizeSp != 16 {
		t.Fatalf("General.FontSizeSp=%v, want 16", cfg.General.FontSizeSp)
	}
	if !cfg.General.DimInactivePanes {
		t.Fatal("General.DimInactivePanes should preserve general block values")
	}

	out := string(mustMarshalConfig(t, cfg))
	if strings.Contains(out, "\nfont:\n") {
		t.Fatalf("serialized config should not contain legacy font block:\n%s", out)
	}
	if !strings.Contains(out, "general:") || !strings.Contains(out, "typeface: Consolas") || !strings.Contains(out, "font_size_sp: 16") {
		t.Fatalf("serialized config missing migrated general font settings:\n%s", out)
	}
}

func TestNormalizeViewerAssociations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.Associations = []ViewerAssociation{
		{Extension: " PDF ", AppPath: ` "C:\Apps\SumatraPDF\SumatraPDF.exe" `},
		{Extension: ".png", AppPath: `C:\Apps\ImageView\viewer.exe`},
		{Extension: "*.tar.gz", AppPath: `C:\Apps\Archive\archive.exe`},
		{Extension: ".PNG", AppPath: `C:\Apps\ImageView\new-viewer.exe`},
		{Extension: "", AppPath: `C:\bad.exe`},
		{Extension: ".", AppPath: `C:\bad.exe`},
		{Extension: `dir\bad`, AppPath: `C:\bad.exe`},
		{Extension: ".txt", AppPath: ""},
	}

	cfg.normalize()

	if strings.Contains(string(mustMarshalConfig(t, cfg)), "associated_extensions:") {
		t.Fatal("deprecated associated_extensions should not be serialized")
	}

	if len(cfg.Viewer.Associations) != 0 {
		t.Fatalf("viewer.associations should be cleared after normalize, got %#v", cfg.Viewer.Associations)
	}

	got := make([]string, 0, len(FlattenAssociationPrograms(cfg.Associations)))
	for _, assoc := range FlattenAssociationPrograms(cfg.Associations) {
		got = append(got, assoc.Extension+"="+assoc.AppPath)
	}
	want := ".pdf=C:\\Apps\\SumatraPDF\\SumatraPDF.exe,.png=C:\\Apps\\ImageView\\new-viewer.exe,.tar.gz=C:\\Apps\\Archive\\archive.exe"
	if strings.Join(got, ",") != want {
		t.Fatalf("Associations=%q, want %q", strings.Join(got, ","), want)
	}

	out := string(mustMarshalConfig(t, cfg))
	if strings.Contains(out, "viewer:\n    associations:") {
		t.Fatalf("serialized config should not keep viewer.associations:\n%s", out)
	}
	if !strings.Contains(out, "associations:") {
		t.Fatalf("serialized config missing top-level associations:\n%s", out)
	}
}

func TestNormalizeTopLevelAssociationPrograms(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Associations = []AssociationProgram{
		{AppPath: ` "C:\Apps\Player\player.exe" `, Extensions: []string{"MP3", ".mkv"}},
		{AppPath: `C:\Apps\Player\player.exe`, Extensions: []string{"*.mp4", ".mp3"}},
		{AppPath: `C:\Apps\Viewer\view.exe`, Extensions: []string{".pdf"}},
	}

	cfg.normalize()

	got := make([]string, 0, len(FlattenAssociationPrograms(cfg.Associations)))
	for _, assoc := range FlattenAssociationPrograms(cfg.Associations) {
		got = append(got, assoc.Extension+"="+assoc.AppPath)
	}
	want := ".mkv=C:\\Apps\\Player\\player.exe,.mp3=C:\\Apps\\Player\\player.exe,.mp4=C:\\Apps\\Player\\player.exe,.pdf=C:\\Apps\\Viewer\\view.exe"
	if strings.Join(got, ",") != want {
		t.Fatalf("FlattenAssociationPrograms=%q, want %q", strings.Join(got, ","), want)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "extensions: mkv, mp3, mp4") {
		t.Fatalf("serialized grouped associations should use compact csv extensions:\n%s", out)
	}
	if strings.Contains(out, "- .mkv") || strings.Contains(out, "extensions:\n") {
		t.Fatalf("serialized grouped associations should not use a multiline extension list:\n%s", out)
	}
}

func TestLoadTopLevelAssociationProgramsFromCompactAndLegacyForms(t *testing.T) {
	raw := `
associations:
  - app_path: C:\Apps\Player\player.exe
    extensions: mkv, mp3, mp4
  - app_path: C:\Apps\Viewer\view.exe
    extensions:
      - .pdf
      - txt
viewer:
  mode: file
  shell: auto
  command: cat {path}
`
	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	got := make([]string, 0, len(FlattenAssociationPrograms(cfg.Associations)))
	for _, assoc := range FlattenAssociationPrograms(cfg.Associations) {
		got = append(got, assoc.Extension+"="+assoc.AppPath)
	}
	want := ".mkv=C:\\Apps\\Player\\player.exe,.mp3=C:\\Apps\\Player\\player.exe,.mp4=C:\\Apps\\Player\\player.exe,.pdf=C:\\Apps\\Viewer\\view.exe,.txt=C:\\Apps\\Viewer\\view.exe"
	if strings.Join(got, ",") != want {
		t.Fatalf("loaded grouped associations=%q, want %q", strings.Join(got, ","), want)
	}
}

func TestNormalizeViewerCommandRules(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.CommandRules = []ViewerCommandRule{
		{Pattern: ` (?i)\.log$ `, Command: ` tail -f {path} `},
		{Pattern: `[`, Command: `broken`},
		{Pattern: `^access\.log$`, Command: `grep ERROR {path}`},
		{Pattern: `(?i)\.log$`, Command: `tail -n 200 {path}`},
		{Pattern: `^debug\.log$`, Command: ``},
	}

	cfg.normalize()

	got := make([]string, 0, len(cfg.Viewer.CommandRules))
	for _, rule := range cfg.Viewer.CommandRules {
		got = append(got, rule.Pattern+"="+rule.Command)
	}
	want := `^access\.log$=grep ERROR {path},(?i)\.log$=tail -n 200 {path}`
	if strings.Join(got, ",") != want {
		t.Fatalf("Viewer.CommandRules=%q, want %q", strings.Join(got, ","), want)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "command_rules:") {
		t.Fatalf("serialized config missing viewer command_rules:\n%s", out)
	}
	if strings.Contains(out, "pattern: [") {
		t.Fatalf("serialized config should drop invalid command rule patterns:\n%s", out)
	}
}

func TestNormalizeViewerRemoteSearchCommandDefaultsAndAllowsOff(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Viewer.RemoteSearchMode != ViewerRemoteSearchModeRemote {
		t.Fatalf("default remote search mode=%q want %q", cfg.Viewer.RemoteSearchMode, ViewerRemoteSearchModeRemote)
	}
	if cfg.Viewer.RemoteSearchCommand != DefaultViewerRemoteSearchCommand {
		t.Fatalf("default remote search command=%q want %q", cfg.Viewer.RemoteSearchCommand, DefaultViewerRemoteSearchCommand)
	}

	cfg.Viewer.RemoteSearchCommand = ""
	cfg.normalize()
	if cfg.Viewer.RemoteSearchCommand != DefaultViewerRemoteSearchCommand {
		t.Fatalf("normalized remote search command=%q want default", cfg.Viewer.RemoteSearchCommand)
	}

	cfg.Viewer.RemoteSearchCommand = "off"
	cfg.normalize()
	if cfg.Viewer.RemoteSearchCommand != "off" {
		t.Fatalf("remote search disable value=%q want %q", cfg.Viewer.RemoteSearchCommand, "off")
	}
	if got := EffectiveViewerRemoteSearchCommand(cfg.Viewer.RemoteSearchCommand); got != "" {
		t.Fatalf("effective remote search command=%q want empty", got)
	}
}

func TestNormalizeViewerRemoteSearchModeDefaultsAndAcceptsAliases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.RemoteSearchMode = ""
	cfg.normalize()
	if cfg.Viewer.RemoteSearchMode != ViewerRemoteSearchModeRemote {
		t.Fatalf("normalized remote search mode=%q want %q", cfg.Viewer.RemoteSearchMode, ViewerRemoteSearchModeRemote)
	}

	cfg.Viewer.RemoteSearchMode = "utility"
	cfg.normalize()
	if cfg.Viewer.RemoteSearchMode != ViewerRemoteSearchModeRemote {
		t.Fatalf("utility remote search mode=%q want %q", cfg.Viewer.RemoteSearchMode, ViewerRemoteSearchModeRemote)
	}

	cfg.Viewer.RemoteSearchMode = "internal"
	cfg.normalize()
	if cfg.Viewer.RemoteSearchMode != ViewerRemoteSearchModeLocal {
		t.Fatalf("internal remote search mode=%q want %q", cfg.Viewer.RemoteSearchMode, ViewerRemoteSearchModeLocal)
	}
}

func TestNormalizeViewerRemoteSearchCommandMigratesLegacyDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.RemoteSearchCommand = legacyViewerRemoteSearchCommand
	cfg.normalize()
	if cfg.Viewer.RemoteSearchCommand != DefaultViewerRemoteSearchCommand {
		t.Fatalf("legacy remote search command=%q want %q", cfg.Viewer.RemoteSearchCommand, DefaultViewerRemoteSearchCommand)
	}
}

func TestMatchViewerCommandRulesUsesLastMatch(t *testing.T) {
	rules := []ViewerCommandRule{
		{Pattern: `\.log$`, Command: `tail -f {path}`},
		{Pattern: `^error\.log$`, Command: `grep ERROR {path}`},
	}

	got, ok := MatchViewerCommandRules(rules, "error.log")
	if !ok {
		t.Fatal("MatchViewerCommandRules should match error.log")
	}
	if got != `grep ERROR {path}` {
		t.Fatalf("MatchViewerCommandRules=%q, want %q", got, `grep ERROR {path}`)
	}

	if got, ok := MatchViewerCommandRules(rules, "notes.txt"); ok || got != "" {
		t.Fatalf("MatchViewerCommandRules(notes.txt) = (%q, %v), want no match", got, ok)
	}
}

func TestNormalizeViewerModeAcceptsHex(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.Mode = "hex"

	cfg.normalize()

	if cfg.Viewer.Mode != "hex" {
		t.Fatalf("Viewer.Mode=%q, want hex", cfg.Viewer.Mode)
	}
}

func TestNormalizeViewerFileEncodingAcceptsUTF16Variants(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.FileEncoding = "utf16-be"

	cfg.normalize()

	if cfg.Viewer.FileEncoding != ViewerFileEncodingUTF16BE {
		t.Fatalf("Viewer.FileEncoding=%q, want %q", cfg.Viewer.FileEncoding, ViewerFileEncodingUTF16BE)
	}
}

func TestNormalizeViewerFileEncodingAcceptsCP437(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.FileEncoding = "ibm437"

	cfg.normalize()

	if cfg.Viewer.FileEncoding != ViewerFileEncodingCP437 {
		t.Fatalf("Viewer.FileEncoding=%q, want %q", cfg.Viewer.FileEncoding, ViewerFileEncodingCP437)
	}
}

func TestDefaultConfigUsesAutoViewerFileEncoding(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Viewer.FileEncoding != ViewerFileEncodingAuto {
		t.Fatalf("Viewer.FileEncoding=%q, want %q", cfg.Viewer.FileEncoding, ViewerFileEncodingAuto)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "file_encoding: auto") {
		t.Fatalf("serialized config missing viewer file_encoding:\n%s", out)
	}
}

func TestDefaultConfigHidesFunctionBarWhenViewerOpen(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Viewer.HideFunctionBarWhenOpen {
		t.Fatal("viewer hide_function_bar_when_open should default to true")
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "hide_function_bar_when_open: true") {
		t.Fatalf("serialized config missing viewer hide_function_bar_when_open:\n%s", out)
	}
}

func TestDefaultConfigDimsInactivePanes(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.General.DimInactivePanes {
		t.Fatal("general dim_inactive_panes should default to false")
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "dim_inactive_panes: false") {
		t.Fatalf("serialized config missing general dim_inactive_panes:\n%s", out)
	}
}

func TestLoadConfigEnsuringFileCreatesDefaultWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")

	cfg, err := LoadConfigEnsuringFile(path)
	if err != nil {
		t.Fatalf("LoadConfigEnsuringFile: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfigEnsuringFile returned nil config")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}

	saved := LoadConfig(path)
	if saved.Viewer.HideFunctionBarWhenOpen != cfg.Viewer.HideFunctionBarWhenOpen {
		t.Fatal("saved config should round-trip defaults")
	}
}

func TestLegacyKeyBindingsAreIgnoredOnLoad(t *testing.T) {
	raw := `
key_bindings:
  focus_next_pane: f3
  move_up: home
  view: f11
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	out := string(mustMarshalConfig(t, cfg))
	if strings.Contains(out, "key_bindings:") {
		t.Fatalf("serialized config should drop legacy key_bindings block:\n%s", out)
	}
}

func TestNormalizeColors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Colors.FilePaneBackground = "161e2"
	cfg.Colors.FilePaneText = "badtext"
	cfg.Colors.Hover = "hover"
	cfg.Colors.HoverText = "hovertext"
	cfg.Colors.Selection = "bad"
	cfg.Colors.SelectionText = "oops"
	cfg.Colors.SelectedFiles = "selected"
	cfg.Colors.SelectedFilesText = "oops"
	cfg.Colors.FocusedSelected = "focused"
	cfg.Colors.FocusedSelectedText = "focusedtext"
	cfg.Colors.CurrentDirBg = "currdir"
	cfg.Colors.CurrentDirText = "currtext"

	cfg.normalize()

	if cfg.Colors.FilePaneBackground != DefaultFilePaneBackgroundHex {
		t.Fatalf("FilePaneBackground=%q, want %q", cfg.Colors.FilePaneBackground, DefaultFilePaneBackgroundHex)
	}
	if cfg.Colors.FilePaneText != DefaultFilePaneTextHex {
		t.Fatalf("FilePaneText=%q, want %q", cfg.Colors.FilePaneText, DefaultFilePaneTextHex)
	}
	if cfg.Colors.Hover != DefaultFilePaneHoverHex {
		t.Fatalf("Hover=%q, want %q", cfg.Colors.Hover, DefaultFilePaneHoverHex)
	}
	if cfg.Colors.HoverText != DefaultFilePaneHoverTextHex {
		t.Fatalf("HoverText=%q, want %q", cfg.Colors.HoverText, DefaultFilePaneHoverTextHex)
	}
	if cfg.Colors.Selection != DefaultFilePaneSelectionHex {
		t.Fatalf("Selection=%q, want %q", cfg.Colors.Selection, DefaultFilePaneSelectionHex)
	}
	if cfg.Colors.SelectionText != DefaultFilePaneSelectionTextHex {
		t.Fatalf("SelectionText=%q, want %q", cfg.Colors.SelectionText, DefaultFilePaneSelectionTextHex)
	}
	if cfg.Colors.SelectedFiles != DefaultFilePaneSelectedFilesHex {
		t.Fatalf("SelectedFiles=%q, want %q", cfg.Colors.SelectedFiles, DefaultFilePaneSelectedFilesHex)
	}
	if cfg.Colors.SelectedFilesText != DefaultFilePaneSelectedTextHex {
		t.Fatalf("SelectedFilesText=%q, want %q", cfg.Colors.SelectedFilesText, DefaultFilePaneSelectedTextHex)
	}
	if cfg.Colors.FocusedSelected != DefaultFilePaneFocusedSelectedHex {
		t.Fatalf("FocusedSelected=%q, want %q", cfg.Colors.FocusedSelected, DefaultFilePaneFocusedSelectedHex)
	}
	if cfg.Colors.FocusedSelectedText != DefaultFilePaneFocusedSelectedTextHex {
		t.Fatalf("FocusedSelectedText=%q, want %q", cfg.Colors.FocusedSelectedText, DefaultFilePaneFocusedSelectedTextHex)
	}
	if cfg.Colors.CurrentDirBg != DefaultCurrentDirBackgroundHex {
		t.Fatalf("CurrentDirBg=%q, want %q", cfg.Colors.CurrentDirBg, DefaultCurrentDirBackgroundHex)
	}
	if cfg.Colors.CurrentDirText != DefaultCurrentDirTextHex {
		t.Fatalf("CurrentDirText=%q, want %q", cfg.Colors.CurrentDirText, DefaultCurrentDirTextHex)
	}
}

func TestNormalizeViewerThemeOverrides(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.Background = "viewerbg"
	cfg.Viewer.Text = "viewtext"
	cfg.Viewer.Selection = "viewsel"

	cfg.normalize()

	if cfg.Viewer.Background != DefaultFilePaneBackgroundHex {
		t.Fatalf("Viewer.Background=%q, want %q", cfg.Viewer.Background, DefaultFilePaneBackgroundHex)
	}
	if cfg.Viewer.Text != DefaultFilePaneTextHex {
		t.Fatalf("Viewer.Text=%q, want %q", cfg.Viewer.Text, DefaultFilePaneTextHex)
	}
	if cfg.Viewer.Selection != DefaultFilePaneSelectionHex {
		t.Fatalf("Viewer.Selection=%q, want %q", cfg.Viewer.Selection, DefaultFilePaneSelectionHex)
	}

	cfg.Viewer.Background = "#112233"
	cfg.Viewer.Text = "aabbcc"
	cfg.Viewer.Selection = "3355dd"
	cfg.normalize()

	if cfg.Viewer.Background != "#112233" {
		t.Fatalf("Viewer.Background=%q, want %q", cfg.Viewer.Background, "#112233")
	}
	if cfg.Viewer.Text != "#AABBCC" {
		t.Fatalf("Viewer.Text=%q, want %q", cfg.Viewer.Text, "#AABBCC")
	}
	if cfg.Viewer.Selection != "#3355DD" {
		t.Fatalf("Viewer.Selection=%q, want %q", cfg.Viewer.Selection, "#3355DD")
	}
}

func TestNormalizeFilenameColors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Colors.Filenames.Text = "oops"
	cfg.Colors.Filenames.Icon = "mystery"
	cfg.Colors.Filenames.AgeRules = []FilenameAgeRule{
		{MaxAge: "24h", Text: "aabbcc", Icon: "schedule"},
		{MaxAge: "bad", Text: "#112233", Icon: "lock"},
		{MaxAge: "1w", Text: "", Icon: ""},
	}
	cfg.Colors.Filenames.PermissionRules = []FilenamePermissionRule{
		{Permissions: "755", Match: "partial", Text: "#556677", Icon: "lock"},
		{Permissions: "oops", Text: "#123456", Icon: "document"},
		{Permissions: "0644", Text: "bad", Icon: "description"},
		{Permissions: "0000", Match: "any", Text: "#654321", Icon: "document"},
	}
	cfg.Colors.Filenames.ExtensionRules = []FilenameExtensionRule{
		{Extension: "GO", Text: "abcdef", Icon: "edit"},
		{Extension: "*.tar.gz", Text: "", Icon: "archive"},
		{Extension: "bad/name", Text: "#123456", Icon: "document"},
	}
	cfg.Colors.Filenames.SizeRules = []FilenameSizeRule{
		{Size: "10mb", Match: "max", Text: "102030", Icon: "movie"},
		{Size: "1g", Text: "", Icon: ""},
		{Size: "oops", Text: "#123456", Icon: "lock"},
	}

	cfg.normalize()

	if cfg.Colors.Filenames.Text != "" {
		t.Fatalf("Filenames.Text=%q want empty", cfg.Colors.Filenames.Text)
	}
	if cfg.Colors.Filenames.Icon != "" {
		t.Fatalf("Filenames.Icon=%q want empty", cfg.Colors.Filenames.Icon)
	}
	if len(cfg.Colors.Filenames.AgeRules) != 1 {
		t.Fatalf("len(Filenames.AgeRules)=%d want 1", len(cfg.Colors.Filenames.AgeRules))
	}
	if got := cfg.Colors.Filenames.AgeRules[0].MaxAge; got != "1d" {
		t.Fatalf("AgeRules[0].MaxAge=%q want %q", got, "1d")
	}
	if got := cfg.Colors.Filenames.AgeRules[0].Text; got != "#AABBCC" {
		t.Fatalf("AgeRules[0].Text=%q want %q", got, "#AABBCC")
	}
	if got := cfg.Colors.Filenames.AgeRules[0].Icon; got != FilenameIconRecent {
		t.Fatalf("AgeRules[0].Icon=%q want %q", got, FilenameIconRecent)
	}
	if len(cfg.Colors.Filenames.PermissionRules) != 2 {
		t.Fatalf("len(Filenames.PermissionRules)=%d want 2", len(cfg.Colors.Filenames.PermissionRules))
	}
	if got := cfg.Colors.Filenames.PermissionRules[0].Permissions; got != "0755" {
		t.Fatalf("PermissionRules[0].Permissions=%q want %q", got, "0755")
	}
	if got := cfg.Colors.Filenames.PermissionRules[0].Match; got != FilenamePermissionMatchAny {
		t.Fatalf("PermissionRules[0].Match=%q want %q", got, FilenamePermissionMatchAny)
	}
	if got := cfg.Colors.Filenames.PermissionRules[0].Icon; got != FilenameIconLocked {
		t.Fatalf("PermissionRules[0].Icon=%q want %q", got, FilenameIconLocked)
	}
	if got := cfg.Colors.Filenames.PermissionRules[1].Permissions; got != "0644" {
		t.Fatalf("PermissionRules[1].Permissions=%q want %q", got, "0644")
	}
	if got := cfg.Colors.Filenames.PermissionRules[1].Match; got != FilenamePermissionMatchExact {
		t.Fatalf("PermissionRules[1].Match=%q want exact", got)
	}
	if got := cfg.Colors.Filenames.PermissionRules[1].Text; got != "" {
		t.Fatalf("PermissionRules[1].Text=%q want empty", got)
	}
	if got := cfg.Colors.Filenames.PermissionRules[1].Icon; got != FilenameIconDocument {
		t.Fatalf("PermissionRules[1].Icon=%q want %q", got, FilenameIconDocument)
	}
	if len(cfg.Colors.Filenames.ExtensionRules) != 2 {
		t.Fatalf("len(Filenames.ExtensionRules)=%d want 2", len(cfg.Colors.Filenames.ExtensionRules))
	}
	if got := cfg.Colors.Filenames.ExtensionRules[0].Extension; got != ".go" {
		t.Fatalf("ExtensionRules[0].Extension=%q want %q", got, ".go")
	}
	if got := cfg.Colors.Filenames.ExtensionRules[0].Text; got != "#ABCDEF" {
		t.Fatalf("ExtensionRules[0].Text=%q want %q", got, "#ABCDEF")
	}
	if got := cfg.Colors.Filenames.ExtensionRules[0].Icon; got != FilenameIconCode {
		t.Fatalf("ExtensionRules[0].Icon=%q want %q", got, FilenameIconCode)
	}
	if got := cfg.Colors.Filenames.ExtensionRules[1].Extension; got != ".tar.gz" {
		t.Fatalf("ExtensionRules[1].Extension=%q want %q", got, ".tar.gz")
	}
	if got := cfg.Colors.Filenames.ExtensionRules[1].Icon; got != FilenameIconArchive {
		t.Fatalf("ExtensionRules[1].Icon=%q want %q", got, FilenameIconArchive)
	}
	if len(cfg.Colors.Filenames.SizeRules) != 1 {
		t.Fatalf("len(Filenames.SizeRules)=%d want 1", len(cfg.Colors.Filenames.SizeRules))
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Size; got != "10m" {
		t.Fatalf("SizeRules[0].Size=%q want %q", got, "10m")
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Match; got != FilenameSizeMatchAtMost {
		t.Fatalf("SizeRules[0].Match=%q want %q", got, FilenameSizeMatchAtMost)
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Text; got != "#102030" {
		t.Fatalf("SizeRules[0].Text=%q want %q", got, "#102030")
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Icon; got != FilenameIconVideo {
		t.Fatalf("SizeRules[0].Icon=%q want %q", got, FilenameIconVideo)
	}
}

func TestNormalizeFilenameAgeRulesSortsAndDedupes(t *testing.T) {
	got := NormalizeFilenameAgeRules([]FilenameAgeRule{
		{MaxAge: "1w", Text: "#334455"},
		{MaxAge: "24h", Text: "#112233"},
		{MaxAge: "1d", Text: "#556677", Icon: "schedule"},
		{MaxAge: "45m", Icon: "lock"},
	})

	if len(got) != 3 {
		t.Fatalf("len(NormalizeFilenameAgeRules)=%d want 3", len(got))
	}
	if got[0].MaxAge != "45m" || got[1].MaxAge != "1d" || got[2].MaxAge != "1w" {
		t.Fatalf("NormalizeFilenameAgeRules order=%#v want 45m, 1d, 1w", got)
	}
	if got[1].Text != "#556677" || got[1].Icon != FilenameIconRecent {
		t.Fatalf("NormalizeFilenameAgeRules duplicate merge=%#v want last 1d rule", got[1])
	}
}

func TestFilenamePermissionAndSizeMatchesSupportPartialRules(t *testing.T) {
	if !FilenamePermissionMatches("0755", FilenamePermissionRule{Permissions: "0111", Match: FilenamePermissionMatchAny}) {
		t.Fatal("any-match permissions should detect executables")
	}
	if !FilenamePermissionMatches("0444", FilenamePermissionRule{Permissions: "0222", Match: FilenamePermissionMatchNone}) {
		t.Fatal("none-match permissions should detect readonly files")
	}
	if FilenamePermissionMatches("0644", FilenamePermissionRule{Permissions: "0111", Match: FilenamePermissionMatchAny}) {
		t.Fatal("any-match permissions should not match non-executables")
	}
	if !FilenamePermissionMatches("0644", FilenamePermissionRule{Permissions: "0644"}) {
		t.Fatal("exact permissions should still work")
	}
	if !FilenameSizeMatches(10<<20, FilenameSizeRule{Size: "10m"}) {
		t.Fatal("at-least size match should include equal values")
	}
	if !FilenameSizeMatches(512, FilenameSizeRule{Size: "1k", Match: FilenameSizeMatchAtMost}) {
		t.Fatal("at-most size match should include smaller values")
	}
	if FilenameSizeMatches(2048, FilenameSizeRule{Size: "1k", Match: FilenameSizeMatchAtMost}) {
		t.Fatal("at-most size match should reject larger values")
	}
}

func mustMarshalConfig(t *testing.T, cfg *Config) []byte {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return data
}
