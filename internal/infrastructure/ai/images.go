package ai

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // register GIF decoding
	"image/jpeg"
	_ "image/png" // register PNG decoding
	"math"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP decoding

	"github.com/tyler/wodl/internal/application/common"
)

// jpegQuality is a deliberate compromise: high enough that small text on a
// whiteboard photo stays legible, low enough to keep uploads small.
const jpegQuality = 85

// shrinkToFit scales an image down so its longest edge is at most maxEdge,
// re-encoding as JPEG. Vision models bill by pixel area, so a phone screenshot
// sent at full resolution can blow a token-per-minute budget on its own — this
// is what keeps an import inside a modest rate limit.
//
// Images already within the limit are returned untouched, so nothing is
// re-encoded (and degraded) for no reason. A maxEdge of zero disables scaling.
func shrinkToFit(img common.BoardImage, maxEdge int) (common.BoardImage, error) {
	if maxEdge <= 0 {
		return img, nil
	}

	decoded, _, err := image.Decode(bytes.NewReader(img.Data))
	if err != nil {
		// Not something we can resize; send it as-is and let the API decide.
		return img, nil
	}

	bounds := decoded.Bounds()
	longest := max(bounds.Dx(), bounds.Dy())
	if longest <= maxEdge {
		return img, nil
	}

	scale := float64(maxEdge) / float64(longest)
	width := int(math.Round(float64(bounds.Dx()) * scale))
	height := int(math.Round(float64(bounds.Dy()) * scale))
	if width < 1 || height < 1 {
		return img, nil
	}

	// CatmullRom keeps edges crisp, which matters when the payload is text.
	resized := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(resized, resized.Bounds(), decoded, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return common.BoardImage{}, fmt.Errorf("resizing the image: %w", err)
	}

	return common.BoardImage{MediaType: "image/jpeg", Data: buf.Bytes()}, nil
}
