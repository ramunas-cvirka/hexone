package theme

import (
	resources "hexone"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

func OpticalTextYOffsetPx(gtx layout.Context, face font.Typeface, size unit.Sp) int {
	switch string(face) {
	case resources.BundledFontFamilyConsolas:
		px := gtx.Sp(size)
		if px >= 17 {
			return 2
		}
		if px >= 1 {
			return 1
		}
	}
	return 0
}
