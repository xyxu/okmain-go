package okmain

import (
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xyxu/okmain-go/internal/rng"
	"github.com/xyxu/okmain-go/internal/sampling"
)

func TestSinglePixelExact(t *testing.T) {
	buf := []uint8{128, 64, 32}
	input, err := NewInputImage(1, 1, buf)
	if err != nil {
		t.Fatal(err)
	}
	colors := Colors(input)
	if len(colors) != 1 || colors[0] != (RGB{128, 64, 32}) {
		t.Fatalf("got %#v", colors)
	}
}

func TestUniformImageExact(t *testing.T) {
	buf := make([]uint8, 0, 300)
	for i := 0; i < 100; i++ {
		buf = append(buf, 200, 100, 50)
	}
	input, err := NewInputImage(10, 10, buf)
	if err != nil {
		t.Fatal(err)
	}
	colors := Colors(input)
	if len(colors) != 1 || colors[0] != (RGB{200, 100, 50}) {
		t.Fatalf("got %#v", colors)
	}
}

func TestDominantColorIsFirst(t *testing.T) {
	w, h := uint16(20), uint16(20)
	buf := make([]uint8, int(w)*int(h)*3)
	for y := 0; y < int(h); y++ {
		for x := 0; x < int(w); x++ {
			idx := (y*int(w) + x) * 3
			if x >= 2 && x < 18 && y >= 2 && y < 18 {
				buf[idx] = 255
			} else {
				buf[idx], buf[idx+1], buf[idx+2] = 40, 40, 40
			}
		}
	}
	input, err := NewInputImage(w, h, buf)
	if err != nil {
		t.Fatal(err)
	}
	colors := Colors(input)
	if len(colors) == 0 || colors[0].R <= 150 || colors[0].G >= 80 {
		t.Fatalf("got %#v", colors)
	}
}

func TestDeterministic(t *testing.T) {
	buf := []uint8{255, 0, 0, 0, 255, 0, 0, 0, 255}
	input, err := NewInputImage(3, 1, buf)
	if err != nil {
		t.Fatal(err)
	}
	a := Colors(input)
	b := Colors(input)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("%#v != %#v", a, b)
	}
	want := []RGB{{0, 0, 255}, {0, 255, 0}, {255, 0, 0}}
	if !reflect.DeepEqual(a, want) {
		t.Fatalf("got %#v want %#v", a, want)
	}
}

func TestCheckerMatchesRust(t *testing.T) {
	buf := make([]uint8, 4*4*3)
	palette := [][3]uint8{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 255, 255}}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := palette[(x+y)%4]
			idx := (y*4 + x) * 3
			buf[idx], buf[idx+1], buf[idx+2] = c[0], c[1], c[2]
		}
	}
	input, err := NewInputImage(4, 4, buf)
	if err != nil {
		t.Fatal(err)
	}
	want := []RGB{{0, 0, 255}, {0, 255, 0}, {255, 0, 0}, {255, 255, 255}}
	if got := Colors(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestNewInputImageFromImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	img.SetRGBA(1, 0, color.RGBA{G: 255, A: 255})
	img.SetRGBA(0, 1, color.RGBA{B: 255, A: 255})
	img.SetRGBA(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	input, err := NewInputImageFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint8{255, 0, 0, 0, 255, 0, 0, 0, 255, 255, 255, 255}
	if !reflect.DeepEqual(input.Buf, want) {
		t.Fatalf("got %#v want %#v", input.Buf, want)
	}
}
func TestNewInputImageFromFileDecodesPNG(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "input.png")
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 4, G: 5, B: 6, A: 255})
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	input, err := NewInputImageFromFile(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint8{4, 5, 6}
	if input.Width != 1 || input.Height != 1 || !reflect.DeepEqual(input.Buf, want) {
		t.Fatalf("got %dx%d %#v", input.Width, input.Height, input.Buf)
	}
}

// TestNewInputImageFromFileDecodesGIF tests that GIF files can be decoded.
func TestNewInputImageFromFileDecodesGIF(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "input.gif")
	img := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{
		color.RGBA{R: 10, G: 20, B: 30, A: 255},
		color.RGBA{R: 40, G: 50, B: 60, A: 255},
	})
	img.SetColorIndex(0, 0, 0)
	img.SetColorIndex(1, 0, 1)
	img.SetColorIndex(0, 1, 1)
	img.SetColorIndex(1, 1, 0)

	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(file, img, nil); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	input, err := NewInputImageFromFile(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	// Check that we got 4 pixels (2x2)
	if int(input.Width)*int(input.Height)*3 != len(input.Buf) {
		t.Fatalf("unexpected buffer size: got %d pixels", len(input.Buf)/3)
	}
	// Check first pixel (color index 0 -> palette entry 0 = 10,20,30)
	if input.Buf[0] != 10 || input.Buf[1] != 20 || input.Buf[2] != 30 {
		t.Fatalf("first pixel got %v want [10 20 30]", input.Buf[0:3])
	}
}

func TestInputErrors(t *testing.T) {
	_, err := NewInputImage(0, 0, nil)
	if !errors.Is(err, ErrEmptyBuffer) {
		t.Fatalf("got %v", err)
	}
	_, err = NewInputImage(1, 2, []uint8{1, 2})
	if err == nil || !strings.Contains(err.Error(), "of 3") {
		t.Fatalf("got %v", err)
	}
}

func TestDistanceMask(t *testing.T) {
	if got := DistanceMask(0.3, 101, 101, 50, 50); abs32(got-1.0) >= 1e-6 {
		t.Fatalf("center got %v", got)
	}
	if got := DistanceMask(0.3, 100, 100, 0, 0); abs32(got-0.1) >= 1e-6 {
		t.Fatalf("corner got %v", got)
	}
	if got := DistanceMask(0.3, 1, 1, 0, 0); abs32(got-0.1) >= 1e-6 {
		t.Fatalf("1x1 got %v", got)
	}
}

func TestBlockSize(t *testing.T) {
	if got := sampling.BlockSize(100, 100); got != 1 {
		t.Fatalf("got %d", got)
	}
	if got := sampling.BlockSize(499, 499); got != 1 {
		t.Fatalf("got %d", got)
	}
	if got := sampling.BlockSize(1000, 1000); got != 4 {
		t.Fatalf("got %d", got)
	}
}

func TestHex(t *testing.T) {
	if got := (RGB{57, 82, 69}).Hex(); got != "#395245" {
		t.Fatalf("got %s", got)
	}
}

func TestRNGSequence(t *testing.T) {
	rng := rng.NewXoshiro256PlusPlus(314159)
	wantRange := []int{85, 72, 1, 38, 40}
	for i, w := range wantRange {
		if got := rng.RandomRange(0, 100); got != w {
			t.Fatalf("range[%d] got %d want %d", i, got, w)
		}
	}
}
