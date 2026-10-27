package images

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/gen2brain/webp"
)

func sample(t *testing.T, w, h int, enc func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 120, 255})
		}
	}
	var buf bytes.Buffer
	if err := enc(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var jpg = func(b *bytes.Buffer, i image.Image) error { return jpeg.Encode(b, i, nil) }
var pngEnc = func(b *bytes.Buffer, i image.Image) error { return png.Encode(b, i) }

func TestProcessMakesWebPVariantsAndKeepsAspect(t *testing.T) {
	res, err := Process(sample(t, 2000, 1500, jpg))
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 2000 || res.Height != 1500 || res.Blurhash == "" {
		t.Fatalf("meta: %+v", res)
	}
	want := map[string][2]int{"thumb": {160, 120}, "card": {480, 360}, "detail": {1080, 810}}
	for name, wh := range want {
		data, ok := res.Variants[name]
		if !ok {
			t.Fatalf("missing variant %s", name)
		}
		cfg, err := webp.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s is not webp: %v", name, err)
		}
		if cfg.Width != wh[0] || cfg.Height != wh[1] {
			t.Errorf("%s = %dx%d, want %dx%d", name, cfg.Width, cfg.Height, wh[0], wh[1])
		}
	}
}

func TestProcessNeverUpscales(t *testing.T) {
	res, err := Process(sample(t, 300, 600, pngEnc)) // portrait, smaller than card/detail
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := webp.DecodeConfig(bytes.NewReader(res.Variants["detail"]))
	if cfg.Width != 300 || cfg.Height != 600 {
		t.Fatalf("detail = %dx%d, want original 300x600", cfg.Width, cfg.Height)
	}
	thumb, _ := webp.DecodeConfig(bytes.NewReader(res.Variants["thumb"]))
	if thumb.Width != 80 || thumb.Height != 160 { // longest side 160
		t.Fatalf("thumb = %dx%d", thumb.Width, thumb.Height)
	}
}

func TestProcessRejectsNonImages(t *testing.T) {
	if _, err := Process([]byte("%PDF-1.7 no soy imagen")); err == nil {
		t.Fatal("want error for non-image")
	}
}

func TestProcessIsDeterministic(t *testing.T) {
	src := sample(t, 640, 480, jpg)
	a, _ := Process(src)
	b, _ := Process(src)
	if !bytes.Equal(a.Variants["card"], b.Variants["card"]) || a.Blurhash != b.Blurhash {
		t.Fatal("same input must give the same output (job retries are idempotent)")
	}
}
