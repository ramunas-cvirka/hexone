package ui

import (
	"sync"

	"gioui.org/widget"
	mdicons "golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	uiIconsOnce    sync.Once
	uiCloseIconRef *widget.Icon
	uiRefreshIcon  *widget.Icon
	uiCopyIcon     *widget.Icon
)

func uiCloseIcon() *widget.Icon {
	initUIIcons()
	return uiCloseIconRef
}

func uiRefreshGlyphIcon() *widget.Icon {
	initUIIcons()
	return uiRefreshIcon
}

func uiCopyGlyphIcon() *widget.Icon {
	initUIIcons()
	return uiCopyIcon
}

func initUIIcons() {
	uiIconsOnce.Do(func() {
		uiCloseIconRef = mustUIIcon(widget.NewIcon(mdicons.NavigationClose))
		uiRefreshIcon = mustUIIcon(widget.NewIcon(mdicons.NavigationRefresh))
		uiCopyIcon = mustUIIcon(widget.NewIcon(mdicons.ContentContentCopy))
	})
}

func mustUIIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}
