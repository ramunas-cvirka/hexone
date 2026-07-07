// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package resources

import _ "embed"

const (
	EmbeddedHelpSource    = "bundled help"
	EmbeddedLicenseSource = "bundled LICENSE"
	EmbeddedNoticeSource  = "bundled NOTICE"

	EmbeddedFiraCodeNerdFontMonoRegularPath      = "embedded:FiraCodeNerdFontMono-Regular.ttf"
	EmbeddedFiraCodeNerdFontMonoBoldPath         = "embedded:FiraCodeNerdFontMono-Bold.ttf"
	EmbeddedHackNerdFontMonoRegularPath          = "embedded:HackNerdFontMono-Regular.ttf"
	EmbeddedHackNerdFontMonoBoldPath             = "embedded:HackNerdFontMono-Bold.ttf"
	EmbeddedJetBrainsMonoNerdFontMonoRegularPath = "embedded:JetBrainsMonoNerdFontMono-Regular.ttf"
	EmbeddedJetBrainsMonoNerdFontMonoBoldPath    = "embedded:JetBrainsMonoNerdFontMono-Bold.ttf"
	EmbeddedIosevkaNerdFontMonoRegularPath       = "embedded:IosevkaNerdFontMono-Regular.ttf"
	EmbeddedIosevkaNerdFontMonoBoldPath          = "embedded:IosevkaNerdFontMono-Bold.ttf"

	BundledFontFamilyFiraCodeNerdFontMono      = "FiraCode Nerd Font Mono"
	BundledFontFamilyHackNerdFontMono          = "Hack Nerd Font Mono"
	BundledFontFamilyJetBrainsMonoNerdFontMono = "JetBrainsMono Nerd Font Mono"
	BundledFontFamilyIosevkaNerdFontMono       = "Iosevka Nerd Font Mono"
)

type BundledFontFamily struct {
	Name        string
	Label       string
	RegularPath string
	BoldPath    string
	Monospace   bool
}

//go:embed HELP.md
var embeddedHelpMarkdown string

//go:embed LICENSE
var embeddedLicenseText string

//go:embed NOTICE
var embeddedNoticeText string

//go:embed protocols.yaml
var embeddedProtocolsYAML []byte

//go:embed assets/FiraCodeNerdFontMono-Regular.ttf
var embeddedFiraCodeNerdFontMonoRegular []byte

//go:embed assets/FiraCodeNerdFontMono-Bold.ttf
var embeddedFiraCodeNerdFontMonoBold []byte

//go:embed assets/HackNerdFontMono-Regular.ttf
var embeddedHackNerdFontMonoRegular []byte

//go:embed assets/HackNerdFontMono-Bold.ttf
var embeddedHackNerdFontMonoBold []byte

//go:embed assets/JetBrainsMonoNerdFontMono-Regular.ttf
var embeddedJetBrainsMonoNerdFontMonoRegular []byte

//go:embed assets/JetBrainsMonoNerdFontMono-Bold.ttf
var embeddedJetBrainsMonoNerdFontMonoBold []byte

//go:embed assets/IosevkaNerdFontMono-Regular.ttf
var embeddedIosevkaNerdFontMonoRegular []byte

//go:embed assets/IosevkaNerdFontMono-Bold.ttf
var embeddedIosevkaNerdFontMonoBold []byte

var bundledFontFamilies = []BundledFontFamily{
	{
		Name:        BundledFontFamilyFiraCodeNerdFontMono,
		Label:       "FiraCode",
		RegularPath: EmbeddedFiraCodeNerdFontMonoRegularPath,
		BoldPath:    EmbeddedFiraCodeNerdFontMonoBoldPath,
		Monospace:   true,
	},
	{
		Name:        BundledFontFamilyJetBrainsMonoNerdFontMono,
		Label:       "JetBrains",
		RegularPath: EmbeddedJetBrainsMonoNerdFontMonoRegularPath,
		BoldPath:    EmbeddedJetBrainsMonoNerdFontMonoBoldPath,
		Monospace:   true,
	},
	{
		Name:        BundledFontFamilyHackNerdFontMono,
		Label:       "Hack",
		RegularPath: EmbeddedHackNerdFontMonoRegularPath,
		BoldPath:    EmbeddedHackNerdFontMonoBoldPath,
		Monospace:   true,
	},
	{
		Name:        BundledFontFamilyIosevkaNerdFontMono,
		Label:       "Iosevka",
		RegularPath: EmbeddedIosevkaNerdFontMonoRegularPath,
		BoldPath:    EmbeddedIosevkaNerdFontMonoBoldPath,
		Monospace:   true,
	},
}

func HelpMarkdown() string {
	return embeddedHelpMarkdown
}

func LicenseText() string {
	return embeddedLicenseText
}

func NoticeText() string {
	return embeddedNoticeText
}

func BundledFontFamilies() []BundledFontFamily {
	out := make([]BundledFontFamily, len(bundledFontFamilies))
	copy(out, bundledFontFamilies)
	return out
}

func BundledFontFamilyByName(name string) (BundledFontFamily, bool) {
	for _, family := range bundledFontFamilies {
		if family.Name == name {
			return family, true
		}
	}
	return BundledFontFamily{}, false
}

func IsBundledFontFamily(name string) bool {
	_, ok := BundledFontFamilyByName(name)
	return ok
}

func IsBundledMonospaceFontFamily(name string) bool {
	family, ok := BundledFontFamilyByName(name)
	return ok && family.Monospace
}

func BundledFont(path string) ([]byte, bool) {
	switch path {
	case EmbeddedFiraCodeNerdFontMonoRegularPath:
		return append([]byte(nil), embeddedFiraCodeNerdFontMonoRegular...), true
	case EmbeddedFiraCodeNerdFontMonoBoldPath:
		return append([]byte(nil), embeddedFiraCodeNerdFontMonoBold...), true
	case EmbeddedHackNerdFontMonoRegularPath:
		return append([]byte(nil), embeddedHackNerdFontMonoRegular...), true
	case EmbeddedHackNerdFontMonoBoldPath:
		return append([]byte(nil), embeddedHackNerdFontMonoBold...), true
	case EmbeddedJetBrainsMonoNerdFontMonoRegularPath:
		return append([]byte(nil), embeddedJetBrainsMonoNerdFontMonoRegular...), true
	case EmbeddedJetBrainsMonoNerdFontMonoBoldPath:
		return append([]byte(nil), embeddedJetBrainsMonoNerdFontMonoBold...), true
	case EmbeddedIosevkaNerdFontMonoRegularPath:
		return append([]byte(nil), embeddedIosevkaNerdFontMonoRegular...), true
	case EmbeddedIosevkaNerdFontMonoBoldPath:
		return append([]byte(nil), embeddedIosevkaNerdFontMonoBold...), true
	default:
		return nil, false
	}
}
