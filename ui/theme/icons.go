package theme

import (
	"sync"

	"gioui.org/widget"
	mdicons "golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	iconsOnce      sync.Once
	closeIconRef   *widget.Icon
	refreshIcon    *widget.Icon
	copyIcon       *widget.Icon
	disconnectIcon *widget.Icon
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

func DisconnectIcon() *widget.Icon {
	initIcons()
	return disconnectIcon
}

func initIcons() {
	iconsOnce.Do(func() {
		closeIconRef = mustIcon(widget.NewIcon(mdicons.NavigationClose))
		refreshIcon = mustIcon(widget.NewIcon(mdicons.NavigationRefresh))
		copyIcon = mustIcon(widget.NewIcon(mdicons.ContentContentCopy))
		disconnectIcon = mustIcon(widget.NewIcon(mdicons.ActionExitToApp))
	})
}

func mustIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}
