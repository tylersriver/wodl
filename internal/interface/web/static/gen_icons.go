//go:build ignore

// gen_icons writes the PNG launcher icons used by the PWA manifest. Re-run with
// `go run ./internal/interface/web/static/gen_icons.go` whenever the look-and-
// feel changes; the resulting PNGs are committed under static/.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var (
	bg     = color.NRGBA{R: 0x1d, G: 0x23, B: 0x2a, A: 0xff} // base-200 dark
	fg     = color.NRGBA{R: 0x60, G: 0x5d, B: 0xff, A: 0xff} // daisyui primary (purple-blue)
	plate  = color.NRGBA{R: 0x7c, G: 0x8b, B: 0xa0, A: 0xff} // muted steel for plates
	border = color.NRGBA{R: 0x2b, G: 0x32, B: 0x3b, A: 0xff}
)

// barbell draws a centered horizontal barbell silhouette: a thin grip with two
// thick plate stacks at each end, on a dark rounded-rect background.
func barbell(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	// Background
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bg)
		}
	}

	// Rounded-rect "tile" margin
	margin := size / 16
	radius := size / 6
	fillRoundedRect(img, margin, margin, size-margin, size-margin, radius, border)
	inner := margin + size/64
	fillRoundedRect(img, inner, inner, size-inner, size-inner, radius-size/64, bg)

	// Geometry of the barbell
	cy := size / 2
	barHeight := size / 18
	barY0 := cy - barHeight/2
	barY1 := cy + barHeight/2
	barX0 := size/4 + size/16
	barX1 := size - barX0

	plateWidth := size / 9
	plateHeight := size / 3
	plateY0 := cy - plateHeight/2
	plateY1 := cy + plateHeight/2

	// Inner plates (closer to center) — primary color
	innerPlateX0L := barX0 - plateWidth/2
	innerPlateX1L := innerPlateX0L + plateWidth
	innerPlateX1R := barX1 + plateWidth/2
	innerPlateX0R := innerPlateX1R - plateWidth

	// Outer plates (caps) — steel grey
	outerPlateW := size / 12
	outerPlateH := size / 5
	outerY0 := cy - outerPlateH/2
	outerY1 := cy + outerPlateH/2
	outerX0L := innerPlateX0L - outerPlateW - size/64
	outerX1L := outerX0L + outerPlateW
	outerX0R := innerPlateX1R + size/64
	outerX1R := outerX0R + outerPlateW

	// Draw bar between the inner plates
	fillRect(img, innerPlateX1L, barY0, innerPlateX0R, barY1, plate)

	// Draw inner plates
	fillRoundedRect(img, innerPlateX0L, plateY0, innerPlateX1L, plateY1, size/40, fg)
	fillRoundedRect(img, innerPlateX0R, plateY0, innerPlateX1R, plateY1, size/40, fg)

	// Draw outer plate caps
	fillRoundedRect(img, outerX0L, outerY0, outerX1L, outerY1, size/50, plate)
	fillRoundedRect(img, outerX0R, outerY0, outerX1R, outerY1, size/50, plate)

	return img
}

func fillRect(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	bounds := img.Bounds()
	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}
	if x1 > bounds.Max.X {
		x1 = bounds.Max.X
	}
	if y1 > bounds.Max.Y {
		y1 = bounds.Max.Y
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.Set(x, y, c)
		}
	}
}

// fillRoundedRect fills the [x0,x1)x[y0,y1) box with c, clipping the four
// corners with a circular radius. Cheap and aliased — good enough for icons.
func fillRoundedRect(img *image.NRGBA, x0, y0, x1, y1, r int, c color.NRGBA) {
	if r <= 0 {
		fillRect(img, x0, y0, x1, y1, c)
		return
	}
	// Center band
	fillRect(img, x0, y0+r, x1, y1-r, c)
	// Top and bottom bands (clipped)
	fillRect(img, x0+r, y0, x1-r, y0+r, c)
	fillRect(img, x0+r, y1-r, x1-r, y1, c)
	// Corners
	drawCornerArc(img, x0+r, y0+r, r, -1, -1, c)
	drawCornerArc(img, x1-r-1, y0+r, r, 1, -1, c)
	drawCornerArc(img, x0+r, y1-r-1, r, -1, 1, c)
	drawCornerArc(img, x1-r-1, y1-r-1, r, 1, 1, c)
}

func drawCornerArc(img *image.NRGBA, cx, cy, r, sx, sy int, c color.NRGBA) {
	for dy := 0; dy <= r; dy++ {
		for dx := 0; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				img.Set(cx+sx*dx, cy+sy*dy, c)
			}
		}
	}
}

func writePNG(path string, img *image.NRGBA) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

func main() {
	_, self, _, _ := runtime.Caller(0)
	dir := filepath.Dir(self)
	writePNG(filepath.Join(dir, "icon-192.png"), barbell(192))
	writePNG(filepath.Join(dir, "icon-512.png"), barbell(512))
	// Maskable variant: same art with extra safe-zone padding so OS launchers
	// can crop it into circles/squircles without losing the barbell.
	writePNG(filepath.Join(dir, "icon-512-maskable.png"), maskable(512))
	log.Println("wrote icons to", dir)
}

func maskable(size int) *image.NRGBA {
	// Render onto a 1.4x canvas, then center-crop so the safe zone matches the
	// W3C maskable-icon spec (artwork fills inner ~80% of the icon).
	full := int(float64(size) * 1.25)
	src := barbell(full)
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dst.Set(x, y, bg)
		}
	}
	off := (full - size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dst.Set(x, y, src.NRGBAAt(x+off, y+off))
		}
	}
	return dst
}
