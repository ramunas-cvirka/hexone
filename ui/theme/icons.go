// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package theme

import (
	"sync"

	"gioui.org/widget"
	mdicons "golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	iconsOnce          sync.Once
	closeIconRef       *widget.Icon
	refreshIcon        *widget.Icon
	copyIcon           *widget.Icon
	chevronLeftIcon    *widget.Icon
	chevronRightIcon   *widget.Icon
	addIcon            *widget.Icon
	arrowUpIcon        *widget.Icon
	arrowDownIcon      *widget.Icon
	favoriteIcon       *widget.Icon
	favoriteBorderIcon *widget.Icon
	disconnectIcon     *widget.Icon
)

func CloseIcon() *widget.Icon {
	initIcons()
	return closeIconRef
}

func RefreshIcon() *widget.Icon {
	initIcons()
	return refreshIcon
}

func CopyIcon() *widget.Icon {
	initIcons()
	return copyIcon
}

func ChevronLeftIcon() *widget.Icon {
	initIcons()
	return chevronLeftIcon
}

func ChevronRightIcon() *widget.Icon {
	initIcons()
	return chevronRightIcon
}

func AddIcon() *widget.Icon {
	initIcons()
	return addIcon
}

func ArrowUpIcon() *widget.Icon {
	initIcons()
	return arrowUpIcon
}

func ArrowDownIcon() *widget.Icon {
	initIcons()
	return arrowDownIcon
}

func FavoriteIcon(active bool) *widget.Icon {
	initIcons()
	if active {
		return favoriteIcon
	}
	return favoriteBorderIcon
}

func DisconnectIcon() *widget.Icon {
	initIcons()
	return disconnectIcon
}

func initIcons() {
	iconsOnce.Do(func() {
		closeIconRef = mustIcon(widget.NewIcon(mdicons.NavigationClose))
		refreshIcon = mustIcon(widget.NewIcon(mdicons.NavigationRefresh))
		copyIcon = mustIcon(widget.NewIcon(mdicons.ContentContentCopy))
		chevronLeftIcon = mustIcon(widget.NewIcon(mdicons.NavigationChevronLeft))
		chevronRightIcon = mustIcon(widget.NewIcon(mdicons.NavigationChevronRight))
		addIcon = mustIcon(widget.NewIcon(mdicons.ContentAdd))
		arrowUpIcon = mustIcon(widget.NewIcon(mdicons.NavigationExpandLess))
		arrowDownIcon = mustIcon(widget.NewIcon(mdicons.NavigationExpandMore))
		favoriteIcon = mustIcon(widget.NewIcon(mdicons.ToggleStar))
		favoriteBorderIcon = mustIcon(widget.NewIcon(mdicons.ToggleStarBorder))
		disconnectIcon = mustIcon(widget.NewIcon(mdicons.ActionPowerSettingsNew))
	})
}

func mustIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}
