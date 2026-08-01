package ai

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"strings"
	"testing"

	"github.com/tyler/wodl/internal/application/common"
)

func syntheticJPEG(t *testing.T, w, h int) common.BoardImage {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Some structure, so the encoder doesn't collapse it to nothing.
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encoding fixture: %v", err)
	}
	return common.BoardImage{MediaType: "image/jpeg", Data: buf.Bytes()}
}

func dimensions(t *testing.T, img common.BoardImage) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	return cfg.Width, cfg.Height
}

func TestShrinkToFit_ScalesDownAndKeepsAspect(t *testing.T) {
	original := syntheticJPEG(t, 2000, 1000)

	shrunk, err := shrinkToFit(original, 1024)
	if err != nil {
		t.Fatalf("shrinkToFit: %v", err)
	}

	w, h := dimensions(t, shrunk)
	if w != 1024 {
		t.Errorf("long edge should be capped at 1024, got %d", w)
	}
	if h != 512 {
		t.Errorf("aspect ratio should be preserved (expected 512), got %d", h)
	}
	if len(shrunk.Data) >= len(original.Data) {
		t.Errorf("expected a smaller payload, got %d bytes from %d", len(shrunk.Data), len(original.Data))
	}
}

func TestShrinkToFit_LeavesSmallImagesAlone(t *testing.T) {
	original := syntheticJPEG(t, 800, 600)

	shrunk, err := shrinkToFit(original, 1024)
	if err != nil {
		t.Fatalf("shrinkToFit: %v", err)
	}
	if !bytes.Equal(shrunk.Data, original.Data) {
		t.Error("an image already within the cap should not be re-encoded")
	}
}

func TestShrinkToFit_ZeroDisablesScaling(t *testing.T) {
	original := syntheticJPEG(t, 3000, 3000)

	shrunk, err := shrinkToFit(original, 0)
	if err != nil {
		t.Fatalf("shrinkToFit: %v", err)
	}
	if !bytes.Equal(shrunk.Data, original.Data) {
		t.Error("maxEdge 0 should disable scaling")
	}
}

func TestShrinkToFit_PassesThroughUndecodableData(t *testing.T) {
	original := common.BoardImage{MediaType: "image/jpeg", Data: []byte("not an image")}

	shrunk, err := shrinkToFit(original, 1024)
	if err != nil {
		t.Fatalf("undecodable data should not error: %v", err)
	}
	if !bytes.Equal(shrunk.Data, original.Data) {
		t.Error("undecodable data should pass through untouched")
	}
}

func TestShrinkToFit_TallImageCapsOnHeight(t *testing.T) {
	shrunk, err := shrinkToFit(syntheticJPEG(t, 500, 2000), 1000)
	if err != nil {
		t.Fatalf("shrinkToFit: %v", err)
	}
	w, h := dimensions(t, shrunk)
	if h != 1000 || w != 250 {
		t.Errorf("expected 250x1000, got %dx%d", w, h)
	}
}

// TestShrinkToFit_RealBoards reports the reduction on actual screenshots, which
// is what determines whether an import fits a token-per-minute budget. Vision
// models bill roughly by pixel area, so that ratio is the number that matters.
func TestShrinkToFit_RealBoards(t *testing.T) {
	fixtures := os.Getenv("WODL_BOARD_FIXTURES")
	if fixtures == "" {
		t.Skip("set WODL_BOARD_FIXTURES to measure real screenshots")
	}

	totalBefore, totalAfter := 0, 0
	for _, path := range strings.Split(fixtures, ",") {
		path = strings.TrimSpace(path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		original := common.BoardImage{MediaType: mediaTypeFor(path), Data: data}
		bw, bh := dimensions(t, original)

		shrunk, err := shrinkToFit(original, DefaultGroqMaxImageEdge)
		if err != nil {
			t.Fatalf("shrinkToFit: %v", err)
		}
		aw, ah := dimensions(t, shrunk)

		totalBefore += bw * bh
		totalAfter += aw * ah
		t.Logf("%-14s %dx%d (%d KB) -> %dx%d (%d KB)",
			path[strings.LastIndex(path, "/")+1:],
			bw, bh, len(original.Data)/1024, aw, ah, len(shrunk.Data)/1024)
	}

	ratio := float64(totalAfter) / float64(totalBefore)
	t.Logf("total pixel area: %.0f%% of original (%.1fx reduction)", ratio*100, 1/ratio)
	if ratio >= 1 {
		t.Error("expected the real screenshots to be reduced")
	}
}
