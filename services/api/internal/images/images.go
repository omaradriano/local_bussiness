// Package images turns an uploaded photo into the WebP sizes the app shows, plus a blurhash placeholder.
// Pure Go (WebP via WASM), so the API image stays static and CGO-free.
package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // decoders
	_ "image/png"

	"github.com/buckket/go-blurhash"
	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Variants by name -> longest side in px. Never upscaled.
var Sizes = map[string]int{"thumb": 160, "card": 480, "detail": 1080}

const quality = 80

// MaxPixels guards against decompression bombs (a tiny file that decodes to a huge bitmap).
const MaxPixels = 40_000_000

var ErrNotImage = errors.New("not a supported image (jpeg, png or webp)")

type Result struct {
	Width    int
	Height   int
	Blurhash string
	Variants map[string][]byte // name -> webp bytes
}

func Process(src []byte) (Result, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil {
		return Result{}, ErrNotImage
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return Result{}, fmt.Errorf("image too large: %dx%d", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return Result{}, ErrNotImage
	}
	b := img.Bounds()
	res := Result{Width: b.Dx(), Height: b.Dy(), Variants: map[string][]byte{}}

	for name, side := range Sizes {
		var buf bytes.Buffer
		if err := webp.Encode(&buf, fit(img, side), webp.Options{Quality: quality}); err != nil {
			return Result{}, fmt.Errorf("encode %s: %w", name, err)
		}
		res.Variants[name] = buf.Bytes()
	}

	// Blurhash from a tiny copy: fast and visually the same.
	if res.Blurhash, err = blurhash.Encode(4, 3, fit(img, 32)); err != nil {
		return Result{}, fmt.Errorf("blurhash: %w", err)
	}
	return res, nil
}

// fit scales img so its longest side is at most `side`, keeping the aspect ratio.
func fit(img image.Image, side int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= side && h <= side {
		return img
	}
	if w >= h {
		h = max(1, h*side/w)
		w = side
	} else {
		w = max(1, w*side/h)
		h = side
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}
