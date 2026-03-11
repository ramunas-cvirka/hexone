package appicon

import (
	"image"
	"image/color"
	"testing"
)

func TestDefaultAppIconSourceDecodes(t *testing.T) {
	img, err := defaultAppIconSource()
	if err != nil {
		t.Fatalf("defaultAppIconSource: %v", err)
	}
	if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		t.Fatalf("decoded source bounds = %v", img.Bounds())
	}
}

func TestDefaultAppIconPreparedIsCropped(t *testing.T) {
	src, err := defaultAppIconSource()
	if err != nil {
		t.Fatalf("defaultAppIconSource: %v", err)
	}
	prepared, err := defaultAppIconPrepared()
	if err != nil {
		t.Fatalf("defaultAppIconPrepared: %v", err)
	}
	if prepared.Bounds().Dx() != prepared.Bounds().Dy() {
		t.Fatalf("prepared bounds should stay square, got %v", prepared.Bounds())
	}
	if prepared.Bounds().Dx() >= src.Bounds().Dx() || prepared.Bounds().Dy() >= src.Bounds().Dy() {
		t.Fatalf("prepared icon should crop padding: src=%v prepared=%v", src.Bounds(), prepared.Bounds())
	}
}

func TestRenderDefaultAppIconGeometry(t *testing.T) {
	for _, size := range []int{16, 32, 64, 128} {
		img := renderDefaultAppIcon(size)
		if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
			t.Fatalf("size %d produced %v", size, img.Bounds())
		}
		if alpha := img.RGBAAt(0, 0).A; alpha != 0 {
			t.Fatalf("size %d corner should stay transparent, got alpha=%d", size, alpha)
		}
		center := img.RGBAAt(size/2, size/2)
		if center.A == 0 {
			t.Fatalf("size %d center should be opaque", size)
		}
		visible := visibleAlphaBounds(img, 8)
		minVisible := size * 3 / 4
		if visible.Dx() < minVisible || visible.Dy() < minVisible {
			t.Fatalf("size %d rendered icon is too small inside the canvas: %v", size, visible)
		}
	}
}

func TestMacBundleIconOverscanPct(t *testing.T) {
	if got := macBundleIconOverscanPct(16); got != macBundleTinyOverscanPct {
		t.Fatalf("16px overscan = %d, want %d", got, macBundleTinyOverscanPct)
	}
	if got := macBundleIconOverscanPct(64); got != macBundleSmallOverscanPct {
		t.Fatalf("64px overscan = %d, want %d", got, macBundleSmallOverscanPct)
	}
	if got := macBundleIconOverscanPct(128); got != 0 {
		t.Fatalf("128px overscan = %d, want 0", got)
	}
}

func TestWindowsICOOverscanPct(t *testing.T) {
	if got := windowsICOOverscanPct(16); got != windowsICOTinyOverscanPct {
		t.Fatalf("16px overscan = %d, want %d", got, windowsICOTinyOverscanPct)
	}
	if got := windowsICOOverscanPct(24); got != windowsICOSmallOverscanPct {
		t.Fatalf("24px overscan = %d, want %d", got, windowsICOSmallOverscanPct)
	}
	if got := windowsICOOverscanPct(64); got != windowsICOMediumOverscanPct {
		t.Fatalf("64px overscan = %d, want %d", got, windowsICOMediumOverscanPct)
	}
	if got := windowsICOOverscanPct(128); got != 0 {
		t.Fatalf("128px overscan = %d, want 0", got)
	}
}

func TestTinyOverscannedAppIconGrowsVisibleBounds(t *testing.T) {
	size := 32
	base := visibleAlphaBounds(renderDefaultAppIcon(size), 8)
	overscanned := visibleAlphaBounds(renderOverscannedAppIcon(size, macBundleIconOverscanPct(size)), 8)
	if overscanned.Dx() < base.Dx() || overscanned.Dy() < base.Dy() {
		t.Fatalf("overscanned icon should not shrink visible bounds: base=%v overscanned=%v", base, overscanned)
	}
}

func TestLargeMacBundleIconKeepsFullArtworkVisible(t *testing.T) {
	size := 128
	base := visibleAlphaBounds(renderDefaultAppIcon(size), 8)
	bundle := visibleAlphaBounds(renderOverscannedAppIcon(size, macBundleIconOverscanPct(size)), 8)
	if bundle != base {
		t.Fatalf("large bundle icon should match default bounds: base=%v bundle=%v", base, bundle)
	}
}

func TestRenderWindowsICOAppIconGeometry(t *testing.T) {
	size := 32
	img := renderWindowsICOAppIcon(size)
	if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
		t.Fatalf("windows ico icon bounds=%v want %dx%d", img.Bounds(), size, size)
	}
	if alpha := img.RGBAAt(0, 0).A; alpha != 0 {
		t.Fatalf("windows ico icon corner alpha=%d want 0", alpha)
	}
	if alpha := img.RGBAAt(size/2, size/2).A; alpha == 0 {
		t.Fatal("windows ico icon center should remain visible")
	}
	base := visibleAlphaBounds(renderDefaultAppIcon(size), 8)
	overscanned := visibleAlphaBounds(img, 8)
	if overscanned.Dx() < base.Dx() || overscanned.Dy() < base.Dy() {
		t.Fatalf("windows ico icon should not shrink visible bounds: base=%v overscanned=%v", base, overscanned)
	}
}

func TestPaintRoundedRectKeepsCornersTransparent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	paintRoundedRect(img, img.Bounds(), 14, color.NRGBA{R: 22, G: 26, B: 36, A: 255})
	if alpha := img.RGBAAt(0, 0).A; alpha != 0 {
		t.Fatalf("rounded rect corner alpha=%d want 0", alpha)
	}
	if alpha := img.RGBAAt(32, 1).A; alpha == 0 {
		t.Fatal("rounded rect top edge should remain visible")
	}
	if alpha := img.RGBAAt(32, 32).A; alpha == 0 {
		t.Fatal("rounded rect center should be opaque")
	}
}

func TestRenderMacBundleAppIconGeometry(t *testing.T) {
	size := 128
	bundle := renderMacBundleAppIcon(size)
	if bundle.Bounds().Dx() != size || bundle.Bounds().Dy() != size {
		t.Fatalf("mac bundle icon bounds=%v want %dx%d", bundle.Bounds(), size, size)
	}
	if alpha := bundle.RGBAAt(0, 0).A; alpha != 0 {
		t.Fatalf("mac bundle icon corner alpha=%d want 0", alpha)
	}
	if alpha := bundle.RGBAAt(size/2, size/2).A; alpha == 0 {
		t.Fatal("mac bundle icon center should remain visible")
	}
}

func TestVisibleSquareCropKeepsContentCentered(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	fill := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for y := 10; y < 90; y++ {
		for x := 30; x < 70; x++ {
			src.Set(x, y, fill)
		}
	}
	crop := visibleSquareCrop(src, 8, 10)
	if crop.Dx() != crop.Dy() {
		t.Fatalf("crop should be square, got %v", crop)
	}
	if !crop.In(image.Rect(0, 0, 100, 100)) {
		t.Fatalf("crop should stay inside source bounds, got %v", crop)
	}
	if crop.Min.X > 30 || crop.Max.X < 70 || crop.Min.Y > 10 || crop.Max.Y < 90 {
		t.Fatalf("crop %v should include the visible content", crop)
	}
}

func TestDefaultAppIconX11DataFormat(t *testing.T) {
	data := defaultAppIconX11Data()
	if len(data) == 0 {
		t.Fatal("x11 icon data is empty")
	}
	if data[0] != 16 || data[1] != 16 {
		t.Fatalf("expected first icon to start with 16x16, got %d x %d", data[0], data[1])
	}
}

func TestDefaultAppIconICOHeader(t *testing.T) {
	data, err := defaultAppIconICO()
	if err != nil {
		t.Fatalf("defaultAppIconICO: %v", err)
	}
	if len(data) < 6 {
		t.Fatalf("ico too short: %d", len(data))
	}
	if data[0] != 0 || data[1] != 0 || data[2] != 1 || data[3] != 0 {
		t.Fatalf("invalid ico header: %v", data[:4])
	}
	if data[4] != 8 || data[5] != 0 {
		t.Fatalf("expected 8 icon entries, got %d", int(data[4])|int(data[5])<<8)
	}
}
