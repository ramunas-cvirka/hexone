// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package resources

import _ "embed"

const (
	EmbeddedHelpSource          = "bundled help"
	EmbeddedLicenseSource       = "bundled LICENSE"
	EmbeddedNoticeSource        = "bundled NOTICE"
	EmbeddedRegularFontPath     = "embedded:FiraCode-Regular.ttf"
	EmbeddedMediumFontPath      = "embedded:FiraCode-Medium.ttf"
	EmbeddedBoldFontPath        = "embedded:FiraCode-Bold.ttf"
	EmbeddedConsolasRegularPath = "embedded:consola.ttf"
	EmbeddedConsolasBoldPath    = "embedded:consolab.ttf"

	BundledFontFamilyFiraCode = "Fira Code"
	BundledFontFamilyConsolas = "Consolas"
)

type BundledFontFamily struct {
	Name        string
	RegularPath string
	MediumPath  string
	BoldPath    string
}

//go:embed HELP.md
var embeddedHelpMarkdown string

//go:embed LICENSE
var embeddedLicenseText string

//go:embed NOTICE
var embeddedNoticeText string

//go:embed protocols.yaml
var embeddedProtocolsYAML []byte

//go:embed assets/FiraCode-Regular.ttf
var embeddedRegularFont []byte

//go:embed assets/FiraCode-Medium.ttf
var embeddedMediumFont []byte

//go:embed assets/FiraCode-Bold.ttf
var embeddedBoldFont []byte

//go:embed assets/consola.ttf
var embeddedConsolasRegular []byte

//go:embed assets/consolab.ttf
var embeddedConsolasBold []byte

var bundledFontFamilies = []BundledFontFamily{
	{
		Name:        BundledFontFamilyFiraCode,
		RegularPath: EmbeddedRegularFontPath,
		MediumPath:  EmbeddedMediumFontPath,
		BoldPath:    EmbeddedBoldFontPath,
	},
	{
		Name:        BundledFontFamilyConsolas,
		RegularPath: EmbeddedConsolasRegularPath,
		MediumPath:  EmbeddedConsolasRegularPath,
		BoldPath:    EmbeddedConsolasBoldPath,
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

func BundledFont(path string) ([]byte, bool) {
	switch path {
	case EmbeddedRegularFontPath:
		return append([]byte(nil), embeddedRegularFont...), true
	case EmbeddedMediumFontPath:
		return append([]byte(nil), embeddedMediumFont...), true
	case EmbeddedBoldFontPath:
		return append([]byte(nil), embeddedBoldFont...), true
	case EmbeddedConsolasRegularPath:
		return append([]byte(nil), embeddedConsolasRegular...), true
	case EmbeddedConsolasBoldPath:
		return append([]byte(nil), embeddedConsolasBold...), true
	default:
		return nil, false
	}
}
