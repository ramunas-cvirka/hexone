package appicon

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
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

	iconVisibleAlphaThreshold = 24
	iconVisibleMarginPct      = 0
)

// Embed the canonical icon artwork so every platform icon path is derived from
// the same source image and does not depend on the working directory.
//
//go:embed hexone_icon_art.png
var embeddedIconArtPNG []byte

func init() {
	if app.ID == "" {
		app.ID = AppID
	}
}

var (
	iconImageCache sync.Map
	iconICOData    []byte
	iconICOMu      sync.Mutex
	iconPNGCache   sync.Map
	x11IconData    []uint32
	x11IconDataMu  sync.Mutex

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

func defaultAppIconSource() (image.Image, error) {
	iconSourceOnce.Do(func() {
		iconSourceImg, iconSourceErr = png.Decode(bytes.NewReader(embeddedIconArtPNG))
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
		// into tiny taskbar sizes. Keep only a near-zero safety margin so the
		// icon fills the slot more like native taskbar icons.
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

func defaultAppIconICO() ([]byte, error) {
	iconICOMu.Lock()
	defer iconICOMu.Unlock()
	if iconICOData != nil {
		return append([]byte(nil), iconICOData...), nil
	}

	sizes := []int{16, 32, 48, 256}
	pngParts := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		data, err := defaultAppIconPNG(size)
		if err != nil {
			return nil, err
		}
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
