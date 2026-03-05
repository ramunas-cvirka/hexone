package fm

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
)

type NameCompact struct {
	ApproxCharPx int    `yaml:"approx_char_px"`
	MinHead      int    `yaml:"min_head"`
	MinTail      int    `yaml:"min_tail"`
	Marker       string `yaml:"marker"`
}

type ColumnWidths struct {
	NameWidthDp      int    `yaml:"name_width_dp"`
	NameMinWidthDp   int    `yaml:"name_min_width_dp"`
	PermWidthDp      int    `yaml:"perm_width_dp"`
	PermMinWidthDp   int    `yaml:"perm_min_width_dp"`
	ShowPermissions  bool   `yaml:"show_permissions"`
	PermissionFormat string `yaml:"permission_format"`
	SizeWidthDp      int    `yaml:"size_width_dp"`
	SizeMinWidthDp   int    `yaml:"size_min_width_dp"`
	DateWidthDp      int    `yaml:"date_width_dp"`
	DateMinWidthDp   int    `yaml:"date_min_width_dp"`
	BriefWidthDp     int    `yaml:"brief_width_dp"`
	BriefGapDp       int    `yaml:"brief_gap_dp"`
}

type KeyBindings struct {
	FocusNextPane string `yaml:"focus_next_pane"`
	FocusPrevPane string `yaml:"focus_prev_pane"`
	MoveUp        string `yaml:"move_up"`
	MoveDown      string `yaml:"move_down"`
	MoveLeft      string `yaml:"move_left"`
	MoveRight     string `yaml:"move_right"`
	PageUp        string `yaml:"page_up"`
	PageDown      string `yaml:"page_down"`
	Home          string `yaml:"home"`
	End           string `yaml:"end"`
	Activate      string `yaml:"activate"`
	View          string `yaml:"view"`
	Copy          string `yaml:"copy"`
	Delete        string `yaml:"delete"`
}

type SortConfig struct {
	DefaultKey       string `yaml:"default_key"`
	Descending       bool   `yaml:"descending"`
	DirectoriesFirst bool   `yaml:"directories_first"`
}

type FontConfig struct {
	Typeface    string  `yaml:"typeface"`
	SizeSp      float32 `yaml:"size_sp"`
	RegularPath string  `yaml:"regular_path"`
	MediumPath  string  `yaml:"medium_path"`
	BoldPath    string  `yaml:"bold_path"`
}

type ViewerConfig struct {
	Mode       string  `yaml:"mode"`
	Shell      string  `yaml:"shell"`
	Command    string  `yaml:"command"`
	FontSizeSp float32 `yaml:"font_size_sp"`
}

type SSHSetup struct {
	Name          string `yaml:"name"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	User          string `yaml:"user"`
	Password      string `yaml:"password"`
	KeyPath       string `yaml:"key_path"`
	KeyPassphrase string `yaml:"key_passphrase"`
}

type SSHConfig struct {
	Setups []SSHSetup `yaml:"setups"`
}

type Config struct {
	DateFormats       []string     `yaml:"date_formats"`
	FavoriteLocations []string     `yaml:"favorite_locations"`
	NameCompact       NameCompact  `yaml:"name_compact"`
	Columns           ColumnWidths `yaml:"columns"`
	KeyBindings       KeyBindings  `yaml:"key_bindings"`
	Sort              SortConfig   `yaml:"sort"`
	Font              FontConfig   `yaml:"font"`
	Viewer            ViewerConfig `yaml:"viewer"`
	SSH               SSHConfig    `yaml:"ssh"`
}

func DefaultConfig() *Config {
	cfg := &Config{
		DateFormats: []string{
			"Jan 02 2006 15:04",
			"Jan 02 15:04",
			"Jan 02 2006",
			"Jan 02",
			"01-02",
		},
		FavoriteLocations: []string{},
		NameCompact: NameCompact{
			ApproxCharPx: 8,
			MinHead:      6,
			MinTail:      3,
			Marker:       "..",
		},
		Columns: ColumnWidths{
			NameWidthDp:      180,
			NameMinWidthDp:   52,
			PermWidthDp:      92,
			PermMinWidthDp:   44,
			ShowPermissions:  true,
			PermissionFormat: "auto",
			SizeWidthDp:      92,
			SizeMinWidthDp:   48,
			DateWidthDp:      128,
			DateMinWidthDp:   56,
			BriefWidthDp:     180,
			BriefGapDp:       4,
		},
		KeyBindings: KeyBindings{
			FocusNextPane: "tab",
			FocusPrevPane: "shift+tab",
			MoveUp:        "up",
			MoveDown:      "down",
			MoveLeft:      "left",
			MoveRight:     "right",
			PageUp:        "pgup",
			PageDown:      "pgdown",
			Home:          "home",
			End:           "end",
			Activate:      "enter",
			View:          "f3",
			Copy:          "f5",
			Delete:        "f8",
		},
		Sort: SortConfig{
			DefaultKey:       "name",
			Descending:       false,
			DirectoriesFirst: true,
		},
		Font: FontConfig{
			Typeface:    "Fira Code",
			SizeSp:      14,
			RegularPath: "assets/FiraCode-Regular.ttf",
			MediumPath:  "assets/FiraCode-Medium.ttf",
			BoldPath:    "assets/FiraCode-Bold.ttf",
		},
		Viewer: ViewerConfig{
			Mode:       "file",
			Shell:      "auto",
			Command:    "cat {path}",
			FontSizeSp: 13,
		},
		SSH: SSHConfig{
			Setups: []SSHSetup{},
		},
	}
	cfg.normalize()
	return cfg
}

func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.normalize()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadConfig(path string) *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg
	}

	cfg.normalize()
	return cfg
}

func (c *Config) normalize() {
	if c == nil {
		return
	}
	if len(c.DateFormats) == 0 {
		c.DateFormats = DefaultConfig().DateFormats
	}
	c.normalizeFavoriteLocations()
	if c.NameCompact.ApproxCharPx < 4 {
		c.NameCompact.ApproxCharPx = 8
	}
	if c.NameCompact.MinHead < 1 {
		c.NameCompact.MinHead = 6
	}
	if c.NameCompact.MinTail < 1 {
		c.NameCompact.MinTail = 3
	}
	if c.NameCompact.Marker == "" {
		c.NameCompact.Marker = ".."
	}

	if c.Columns.NameWidthDp < 1 {
		c.Columns.NameWidthDp = 180
	}
	if c.Columns.NameMinWidthDp < 1 {
		c.Columns.NameMinWidthDp = 52
	}
	if c.Columns.PermWidthDp < 1 {
		c.Columns.PermWidthDp = 92
	}
	if c.Columns.PermMinWidthDp < 1 {
		c.Columns.PermMinWidthDp = 44
	}
	switch c.Columns.PermissionFormat {
	case "symbolic", "octal", "auto":
	default:
		c.Columns.PermissionFormat = "auto"
	}
	if c.Columns.SizeWidthDp < 1 {
		c.Columns.SizeWidthDp = 92
	}
	if c.Columns.SizeMinWidthDp < 1 {
		c.Columns.SizeMinWidthDp = 44
	}
	if c.Columns.DateWidthDp < 1 {
		c.Columns.DateWidthDp = 128
	}
	if c.Columns.DateMinWidthDp < 1 {
		c.Columns.DateMinWidthDp = 44
	}
	if c.Columns.BriefWidthDp < 1 {
		c.Columns.BriefWidthDp = 180
	}
	if c.Columns.BriefGapDp < 0 {
		c.Columns.BriefGapDp = 4
	}

	sizeMin := c.NameCompact.ApproxCharPx*5 + 8
	if c.Columns.SizeMinWidthDp < sizeMin {
		c.Columns.SizeMinWidthDp = sizeMin
	}

	permChars := 4
	if c.Columns.PermissionFormat == "symbolic" {
		permChars = 9
	}
	permMin := c.NameCompact.ApproxCharPx*permChars + 12
	if c.Columns.PermMinWidthDp < permMin {
		c.Columns.PermMinWidthDp = permMin
	}

	shortestDate := 5
	if len(c.DateFormats) > 0 {
		shortestDate = utf8.RuneCountInString(c.DateFormats[0])
		for _, format := range c.DateFormats[1:] {
			n := utf8.RuneCountInString(format)
			if n < shortestDate {
				shortestDate = n
			}
		}
	}
	dateMin := c.NameCompact.ApproxCharPx*shortestDate + 16
	if c.Columns.DateMinWidthDp < dateMin {
		c.Columns.DateMinWidthDp = dateMin
	}

	if c.KeyBindings.FocusNextPane == "" {
		c.KeyBindings.FocusNextPane = "tab"
	}
	if c.KeyBindings.FocusPrevPane == "" {
		c.KeyBindings.FocusPrevPane = "shift+tab"
	}
	if c.KeyBindings.MoveUp == "" {
		c.KeyBindings.MoveUp = "up"
	}
	if c.KeyBindings.MoveDown == "" {
		c.KeyBindings.MoveDown = "down"
	}
	if c.KeyBindings.MoveLeft == "" {
		c.KeyBindings.MoveLeft = "left"
	}
	if c.KeyBindings.MoveRight == "" {
		c.KeyBindings.MoveRight = "right"
	}
	if c.KeyBindings.PageUp == "" {
		c.KeyBindings.PageUp = "pgup"
	}
	if c.KeyBindings.PageDown == "" {
		c.KeyBindings.PageDown = "pgdown"
	}
	if c.KeyBindings.Home == "" {
		c.KeyBindings.Home = "home"
	}
	if c.KeyBindings.End == "" {
		c.KeyBindings.End = "end"
	}
	if c.KeyBindings.Activate == "" {
		c.KeyBindings.Activate = "enter"
	}
	if c.KeyBindings.View == "" {
		c.KeyBindings.View = "f3"
	}
	if c.KeyBindings.Copy == "" {
		c.KeyBindings.Copy = "f5"
	}
	if c.KeyBindings.Delete == "" {
		c.KeyBindings.Delete = "f8"
	}

	if c.Sort.DefaultKey == "" {
		c.Sort.DefaultKey = "name"
	}

	if c.Font.Typeface == "" {
		c.Font.Typeface = "Fira Code"
	}
	if c.Font.SizeSp <= 0 {
		c.Font.SizeSp = 14
	}
	if c.Font.RegularPath == "" {
		c.Font.RegularPath = "assets/FiraCode-Regular.ttf"
	}
	if c.Font.MediumPath == "" {
		c.Font.MediumPath = "assets/FiraCode-Medium.ttf"
	}
	if c.Font.BoldPath == "" {
		c.Font.BoldPath = "assets/FiraCode-Bold.ttf"
	}

	switch c.Viewer.Mode {
	case "file", "command":
	default:
		c.Viewer.Mode = "file"
	}
	switch strings.ToLower(strings.TrimSpace(c.Viewer.Shell)) {
	case "", "auto":
		c.Viewer.Shell = "auto"
	case "sh":
		c.Viewer.Shell = "sh"
	case "pwsh", "powershell":
		c.Viewer.Shell = "powershell"
	default:
		c.Viewer.Shell = "auto"
	}
	if c.Viewer.Command == "" {
		c.Viewer.Command = "cat {path}"
	}
	if c.Viewer.FontSizeSp < 6 {
		c.Viewer.FontSizeSp = c.Font.SizeSp * (13.0 / 14.0)
		if c.Viewer.FontSizeSp < 6 {
			c.Viewer.FontSizeSp = 13
		}
	}

	c.normalizeSSHSetups()
}

func (c *Config) normalizeFavoriteLocations() {
	if c == nil {
		return
	}
	if len(c.FavoriteLocations) == 0 {
		c.FavoriteLocations = nil
		return
	}
	out := make([]string, 0, len(c.FavoriteLocations))
	for _, raw := range c.FavoriteLocations {
		loc := strings.TrimSpace(raw)
		if loc == "" {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if sameFavoriteLocation(existing, loc) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		out = append(out, loc)
	}
	c.FavoriteLocations = out
}

func sameFavoriteLocation(a, b string) bool {
	if filepath.IsAbs(a) && filepath.IsAbs(b) {
		a = filepath.Clean(a)
		b = filepath.Clean(b)
		if os.PathSeparator == '\\' {
			return strings.EqualFold(a, b)
		}
		return a == b
	}
	return a == b
}

func (c *Config) normalizeSSHSetups() {
	if c == nil {
		return
	}
	if len(c.SSH.Setups) == 0 {
		c.SSH.Setups = nil
		return
	}
	out := make([]SSHSetup, 0, len(c.SSH.Setups))
	for _, raw := range c.SSH.Setups {
		setup := SSHSetup{
			Name:          strings.TrimSpace(raw.Name),
			Host:          strings.TrimSpace(raw.Host),
			Port:          raw.Port,
			User:          strings.TrimSpace(raw.User),
			Password:      raw.Password,
			KeyPath:       strings.TrimSpace(raw.KeyPath),
			KeyPassphrase: raw.KeyPassphrase,
		}
		if setup.Port <= 0 {
			setup.Port = 22
		}
		out = append(out, setup)
	}
	c.SSH.Setups = out
}
