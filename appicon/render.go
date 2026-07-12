// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package appicon

import (
	"bytes"
	"encoding/binary"
	"fmt"
	resources "hexone"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/png"
	"os"
	"sync"

	"gioui.org/app"
	xdraw "golang.org/x/image/draw"
)

const (
	AppID    = "hexone"
	AppTitle = "hexone"

	iconVisibleAlphaThreshold   = 24
	iconVisibleMarginPct        = 0
	linuxDesktopIconOverscanPct = 8
	windowsICOTinyOverscanPct   = 0
	windowsICOSmallOverscanPct  = 0
	windowsICOMediumOverscanPct = 0
	windowsPackageOverscanPct   = 0
	macBundleTinyOverscanPct    = 10
	macBundleSmallOverscanPct   = 6
	macBundleBackdropRadiusPct  = 23
)

var macBundleBackdropColor = color.NRGBA{R: 22, G: 26, B: 36, A: 255}

func init() {
	if app.ID == "" {
		app.ID = AppID
	}
}

var (
	iconImageCache  sync.Map
	iconICOData     []byte
	iconICOMu       sync.Mutex
	iconPNGCache    sync.Map
	macIconPNGCache sync.Map
	x11IconData     []uint32
	x11IconDataMu   sync.Mutex

	iconSourceOnce   sync.Once
	iconSourceImg    image.Image
	iconSourceErr    error
	iconPreparedOnce sync.Once
	iconPreparedImg  image.Image
	iconPreparedErr  error
)

func renderDefaultAppIcon(size int) *image.RGBA {
	if size < 16 {
		size = 16
	}
	if cached, ok := iconImageCache.Load(size); ok {
		return cached.(*image.RGBA)
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	src, err := defaultAppIconPrepared()
	if err == nil && src != nil {
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	}

	iconImageCache.Store(size, dst)
	return dst
}

func renderOverscannedAppIcon(size int, overscanPct int) *image.RGBA {
	if size < 16 {
		size = 16
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	src, err := defaultAppIconPrepared()
	if err != nil || src == nil {
		return dst
	}
	overscan := size * overscanPct / 100
	drawRect := image.Rect(-overscan, -overscan, size+overscan, size+overscan)
	xdraw.CatmullRom.Scale(dst, drawRect, src, src.Bounds(), draw.Over, nil)
	return dst
}

func defaultAppIconSource() (image.Image, error) {
	iconSourceOnce.Do(func() {
		iconSourceImg, iconSourceErr = png.Decode(bytes.NewReader(resources.AppIconPNG()))
	})
	return iconSourceImg, iconSourceErr
}

func defaultAppIconPrepared() (image.Image, error) {
	iconPreparedOnce.Do(func() {
		src, err := defaultAppIconSource()
		if err != nil {
			iconPreparedErr = err
			return
		}
		// Crop away the transparent halo around the source art before scaling it
		// into tiny taskbar sizes. The square crop preserves all visible artwork
		// while filling the shell slot as tightly as its aspect ratio permits.
		iconPreparedImg = cloneImageRect(src, visibleSquareCrop(src, iconVisibleAlphaThreshold, iconVisibleMarginPct))
	})
	return iconPreparedImg, iconPreparedErr
}

func defaultAppIconPNG(size int) ([]byte, error) {
	if cached, ok := iconPNGCache.Load(size); ok {
		return cached.([]byte), nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, renderDefaultAppIcon(size)); err != nil {
		return nil, err
	}
	data := append([]byte(nil), buf.Bytes()...)
	iconPNGCache.Store(size, data)
	return data, nil
}

func windowsPackageAppIconPNG(size int) ([]byte, error) {
	img := image.Image(renderOverscannedAppIcon(size, windowsPackageOverscanPct))
	if size >= 512 {
		// Store certification limits package images to 200 KiB. Large, true-color
		// scale-400 artwork can exceed that even with maximum DEFLATE compression,
		// so use a high-quality indexed PNG for those variants.
		colors := append(color.Palette{color.NRGBA{}}, palette.Plan9[:255]...)
		indexed := image.NewPaletted(img.Bounds(), colors)
		draw.FloydSteinberg.Draw(indexed, indexed.Bounds(), img, img.Bounds().Min)
		img = indexed
	}
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, err
	}
	return append([]byte(nil), buf.Bytes()...), nil
}

func desktopAppIconPNG(size int) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, renderOverscannedAppIcon(size, linuxDesktopIconOverscanPct)); err != nil {
		return nil, err
	}
	return append([]byte(nil), buf.Bytes()...), nil
}

func macBundleAppIconPNG(size int) ([]byte, error) {
	if cached, ok := macIconPNGCache.Load(size); ok {
		return cached.([]byte), nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, renderMacBundleAppIcon(size)); err != nil {
		return nil, err
	}
	data := append([]byte(nil), buf.Bytes()...)
	macIconPNGCache.Store(size, data)
	return data, nil
}

func macBundleIconOverscanPct(size int) int {
	switch {
	case size <= 32:
		return macBundleTinyOverscanPct
	case size <= 64:
		return macBundleSmallOverscanPct
	default:
		return 0
	}
}

func windowsICOOverscanPct(size int) int {
	switch {
	case size <= 20:
		return windowsICOTinyOverscanPct
	case size <= 32:
		return windowsICOSmallOverscanPct
	case size <= 64:
		return windowsICOMediumOverscanPct
	default:
		return 0
	}
}

func renderMacBundleAppIcon(size int) *image.RGBA {
	if size < 16 {
		size = 16
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	paintRoundedRect(dst, dst.Bounds(), float64(size)*macBundleBackdropRadiusPct/100, macBundleBackdropColor)
	src, err := defaultAppIconPrepared()
	if err != nil || src == nil {
		return dst
	}
	overscan := size * macBundleIconOverscanPct(size) / 100
	drawRect := image.Rect(-overscan, -overscan, size+overscan, size+overscan)
	xdraw.CatmullRom.Scale(dst, drawRect, src, src.Bounds(), draw.Over, nil)
	return dst
}

func renderWindowsICOAppIcon(size int) *image.RGBA {
	return renderOverscannedAppIcon(size, windowsICOOverscanPct(size))
}

func defaultAppIconICO() ([]byte, error) {
	iconICOMu.Lock()
	defer iconICOMu.Unlock()
	if iconICOData != nil {
		return append([]byte(nil), iconICOData...), nil
	}

	sizes := []int{16, 20, 24, 32, 40, 48, 64, 256}
	pngParts := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, renderWindowsICOAppIcon(size)); err != nil {
			return nil, err
		}
		data := append([]byte(nil), buf.Bytes()...)
		pngParts = append(pngParts, data)
	}

	const (
		iconDirSize   = 6
		iconEntrySize = 16
	)
	total := iconDirSize + len(pngParts)*iconEntrySize
	for _, data := range pngParts {
		total += len(data)
	}
	buf := bytes.NewBuffer(make([]byte, 0, total))
	writeLE16(buf, 0)
	writeLE16(buf, 1)
	writeLE16(buf, uint16(len(pngParts)))

	offset := iconDirSize + len(pngParts)*iconEntrySize
	for i, size := range sizes {
		wb := byte(size)
		hb := byte(size)
		if size >= 256 {
			wb = 0
			hb = 0
		}
		buf.WriteByte(wb)
		buf.WriteByte(hb)
		buf.WriteByte(0)
		buf.WriteByte(0)
		writeLE16(buf, 1)
		writeLE16(buf, 32)
		writeLE32(buf, uint32(len(pngParts[i])))
		writeLE32(buf, uint32(offset))
		offset += len(pngParts[i])
	}
	for _, data := range pngParts {
		buf.Write(data)
	}
	iconICOData = append([]byte(nil), buf.Bytes()...)
	return append([]byte(nil), iconICOData...), nil
}

func WriteICO(path string) error {
	data, err := defaultAppIconICO()
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("empty icon path")
	}
	return os.WriteFile(path, data, 0o644)
}

func WritePNG(path string, size int) error {
	data, err := defaultAppIconPNG(size)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("empty icon path")
	}
	return os.WriteFile(path, data, 0o644)
}

// WriteWindowsPackagePNG writes tightly cropped transparent MSIX artwork.
// Windows selects the matching qualified asset for each shell context.
func WriteWindowsPackagePNG(path string, size int) error {
	data, err := windowsPackageAppIconPNG(size)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("empty icon path")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteDesktopPNG(path string, size int) error {
	data, err := desktopAppIconPNG(size)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("empty icon path")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteICNS(path string) error {
	if path == "" {
		return fmt.Errorf("empty icon path")
	}
	type iconChunk struct {
		typ  string
		size int
	}
	chunks := []iconChunk{
		{typ: "ic11", size: 32},
		{typ: "ic12", size: 64},
		{typ: "ic07", size: 128},
		{typ: "ic13", size: 256},
		{typ: "ic08", size: 256},
		{typ: "ic14", size: 512},
		{typ: "ic09", size: 512},
		{typ: "ic10", size: 1024},
	}
	type encodedChunk struct {
		typ  string
		data []byte
	}
	encoded := make([]encodedChunk, 0, len(chunks))
	total := 8
	for _, chunk := range chunks {
		data, err := macBundleAppIconPNG(chunk.size)
		if err != nil {
			return err
		}
		encoded = append(encoded, encodedChunk{typ: chunk.typ, data: data})
		total += 8 + len(data)
	}

	buf := bytes.NewBuffer(make([]byte, 0, total))
	buf.WriteString("icns")
	writeBE32(buf, uint32(total))
	for _, chunk := range encoded {
		buf.WriteString(chunk.typ)
		writeBE32(buf, uint32(8+len(chunk.data)))
		buf.Write(chunk.data)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func defaultAppIconX11Data() []uint32 {
	x11IconDataMu.Lock()
	defer x11IconDataMu.Unlock()
	if x11IconData != nil {
		return x11IconData
	}
	sizes := []int{16, 32, 64, 128}
	out := make([]uint32, 0, 2+len(sizes)*64*64)
	for _, size := range sizes {
		img := renderDefaultAppIcon(size)
		out = append(out, uint32(size), uint32(size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				i := img.PixOffset(x, y)
				r := uint32(img.Pix[i+0])
				g := uint32(img.Pix[i+1])
				b := uint32(img.Pix[i+2])
				a := uint32(img.Pix[i+3])
				out = append(out, (a<<24)|(r<<16)|(g<<8)|b)
			}
		}
	}
	x11IconData = out
	return x11IconData
}

func writeLE16(buf *bytes.Buffer, v uint16) {
	var tmp [2]byte
	binary.LittleEndian.PutUint16(tmp[:], v)
	buf.Write(tmp[:])
}

func writeLE32(buf *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	buf.Write(tmp[:])
}

func writeBE32(buf *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], v)
	buf.Write(tmp[:])
}

func visibleSquareCrop(src image.Image, alphaThreshold uint8, marginPct int) image.Rectangle {
	srcBounds := src.Bounds()
	visible := visibleAlphaBounds(src, alphaThreshold)
	if visible.Empty() {
		return srcBounds
	}

	side := visible.Dx()
	if visible.Dy() > side {
		side = visible.Dy()
	}
	if side <= 0 {
		return srcBounds
	}

	margin := side * marginPct / 100
	if margin < 1 {
		margin = 1
	}
	side += margin * 2
	if side > srcBounds.Dx() {
		side = srcBounds.Dx()
	}
	if side > srcBounds.Dy() {
		side = srcBounds.Dy()
	}

	centerX := visible.Min.X + visible.Dx()/2
	centerY := visible.Min.Y + visible.Dy()/2
	minX := centerX - side/2
	minY := centerY - side/2
	crop := image.Rect(minX, minY, minX+side, minY+side)
	if crop.Min.X < srcBounds.Min.X {
		crop = crop.Add(image.Pt(srcBounds.Min.X-crop.Min.X, 0))
	}
	if crop.Min.Y < srcBounds.Min.Y {
		crop = crop.Add(image.Pt(0, srcBounds.Min.Y-crop.Min.Y))
	}
	if crop.Max.X > srcBounds.Max.X {
		crop = crop.Add(image.Pt(srcBounds.Max.X-crop.Max.X, 0))
	}
	if crop.Max.Y > srcBounds.Max.Y {
		crop = crop.Add(image.Pt(0, srcBounds.Max.Y-crop.Max.Y))
	}
	return crop.Intersect(srcBounds)
}

func paintRoundedRect(dst *image.RGBA, rect image.Rectangle, radius float64, fill color.NRGBA) {
	if dst == nil || rect.Empty() || fill.A == 0 {
		return
	}
	width := rect.Dx()
	height := rect.Dy()
	if width < 1 || height < 1 {
		return
	}
	maxRadius := float64(width)
	if float64(height) < maxRadius {
		maxRadius = float64(height)
	}
	maxRadius *= 0.5
	if radius < 0 {
		radius = 0
	}
	if radius > maxRadius {
		radius = maxRadius
	}
	const samples = 4
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			coverage := roundedRectPixelCoverage(x, y, rect, radius, samples)
			if coverage <= 0 {
				continue
			}
			a := uint8(float64(fill.A)*coverage + 0.5)
			dst.SetRGBA(x, y, color.RGBA{R: fill.R, G: fill.G, B: fill.B, A: a})
		}
	}
}

func roundedRectPixelCoverage(px, py int, rect image.Rectangle, radius float64, samples int) float64 {
	if samples < 1 {
		samples = 1
	}
	hits := 0
	total := samples * samples
	step := 1.0 / float64(samples)
	for sy := 0; sy < samples; sy++ {
		y := float64(py) + (float64(sy)+0.5)*step
		for sx := 0; sx < samples; sx++ {
			x := float64(px) + (float64(sx)+0.5)*step
			if pointInRoundedRect(x, y, rect, radius) {
				hits++
			}
		}
	}
	return float64(hits) / float64(total)
}

func pointInRoundedRect(x, y float64, rect image.Rectangle, radius float64) bool {
	left := float64(rect.Min.X)
	top := float64(rect.Min.Y)
	right := float64(rect.Max.X)
	bottom := float64(rect.Max.Y)
	if x < left || x >= right || y < top || y >= bottom {
		return false
	}
	if radius <= 0 {
		return true
	}
	if x >= left+radius && x < right-radius {
		return true
	}
	if y >= top+radius && y < bottom-radius {
		return true
	}

	cx := x
	switch {
	case x < left+radius:
		cx = left + radius
	case x >= right-radius:
		cx = right - radius
	}
	cy := y
	switch {
	case y < top+radius:
		cy = top + radius
	case y >= bottom-radius:
		cy = bottom - radius
	}
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= radius*radius
}

func visibleAlphaBounds(src image.Image, alphaThreshold uint8) image.Rectangle {
	srcBounds := src.Bounds()
	minX, minY := srcBounds.Max.X, srcBounds.Max.Y
	maxX, maxY := srcBounds.Min.X-1, srcBounds.Min.Y-1
	for y := srcBounds.Min.Y; y < srcBounds.Max.Y; y++ {
		for x := srcBounds.Min.X; x < srcBounds.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if uint8(a>>8) <= alphaThreshold {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX || maxY < minY {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

func cloneImageRect(src image.Image, rect image.Rectangle) *image.RGBA {
	rect = rect.Intersect(src.Bounds())
	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)
	return dst
}
