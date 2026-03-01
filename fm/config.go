package fm

import (
	"os"
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
	NameWidthDp    int `yaml:"name_width_dp"`
	NameMinWidthDp int `yaml:"name_min_width_dp"`
	SizeWidthDp    int `yaml:"size_width_dp"`
	SizeMinWidthDp int `yaml:"size_min_width_dp"`
	DateWidthDp    int `yaml:"date_width_dp"`
	DateMinWidthDp int `yaml:"date_min_width_dp"`
	BriefWidthDp   int `yaml:"brief_width_dp"`
	BriefGapDp     int `yaml:"brief_gap_dp"`
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
}

type SortConfig struct {
	DefaultKey       string `yaml:"default_key"`
	Descending       bool   `yaml:"descending"`
	DirectoriesFirst bool   `yaml:"directories_first"`
}

type Config struct {
	DateFormats []string     `yaml:"date_formats"`
	NameCompact NameCompact  `yaml:"name_compact"`
	Columns     ColumnWidths `yaml:"columns"`
	KeyBindings KeyBindings  `yaml:"key_bindings"`
	Sort        SortConfig   `yaml:"sort"`
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
		NameCompact: NameCompact{
			ApproxCharPx: 8,
			MinHead:      6,
			MinTail:      3,
			Marker:       "..",
		},
		Columns: ColumnWidths{
			NameWidthDp:    180,
			NameMinWidthDp: 52,
			SizeWidthDp:    92,
			SizeMinWidthDp: 48,
			DateWidthDp:    128,
			DateMinWidthDp: 56,
			BriefWidthDp:   180,
			BriefGapDp:     4,
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
		},
		Sort: SortConfig{
			DefaultKey:       "name",
			Descending:       false,
			DirectoriesFirst: true,
		},
	}
	cfg.normalize()
	return cfg
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

	if c.Sort.DefaultKey == "" {
		c.Sort.DefaultKey = "name"
	}
}
