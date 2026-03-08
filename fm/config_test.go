package fm

import (
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
  full_pad_dp: 6
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

	if got, want := SizeMinWidthDp(cfg), defaultApproxCharPx*5+8+12; got != want {
		t.Fatalf("SizeMinWidthDp=%d, want %d", got, want)
	}
	if got, want := PermMinWidthDp(cfg), defaultApproxCharPx*9+12+12; got != want {
		t.Fatalf("PermMinWidthDp=%d, want %d", got, want)
	}
	if got, want := DateMinWidthDp(cfg), defaultApproxCharPx*5+16+12; got != want {
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
	} {
		if strings.Contains(out, disallowed) {
			t.Fatalf("serialized config still contains %q:\n%s", disallowed, out)
		}
	}
	if !strings.Contains(out, "keep_start_chars:") {
		t.Fatalf("serialized config missing keep_start_chars:\n%s", out)
	}
	if strings.Contains(out, "key_bindings:") {
		t.Fatalf("serialized config should not contain legacy key_bindings block:\n%s", out)
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

func TestNormalizeViewerModeAcceptsHex(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.Mode = "hex"

	cfg.normalize()

	if cfg.Viewer.Mode != "hex" {
		t.Fatalf("Viewer.Mode=%q, want hex", cfg.Viewer.Mode)
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

func mustMarshalConfig(t *testing.T, cfg *Config) []byte {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return data
}
