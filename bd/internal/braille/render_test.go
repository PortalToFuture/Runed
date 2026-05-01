package braille

import (
	"image"
	"image/color"
	"testing"
)

func TestEncodeDotMapping(t *testing.T) {
	grid := [][]uint8{
		{255, 0},
		{0, 255},
		{255, 0},
		{0, 255},
	}

	got := encode(grid, Options{Threshold: 128}, channelRed)
	want := string(rune(brailleBase + 0x01 + 0x10 + 0x04 + 0x80))
	if got != want {
		t.Fatalf("encode() = %q, want %q", got, want)
	}
}

func TestEncodeInvert(t *testing.T) {
	grid := [][]uint8{
		{0, 0},
		{0, 0},
		{0, 0},
		{0, 0},
	}

	got := encode(grid, Options{Threshold: 128, Invert: true}, channelRed)
	want := string(rune(brailleBase + 0xFF))
	if got != want {
		t.Fatalf("encode() with invert = %q, want %q", got, want)
	}
}

func TestEncodeColorize(t *testing.T) {
	grid := [][]uint8{
		{255, 255},
		{255, 255},
		{255, 255},
		{255, 255},
	}

	got := encode(grid, Options{Threshold: 128, Colorize: true}, channelGreen)
	want := "\x1b[38;2;0;255;0m" + string(rune(brailleBase+0xFF)) + "\x1b[0m"
	if got != want {
		t.Fatalf("encode() with color = %q, want %q", got, want)
	}
}

func TestRenderOutputsRGBLayersInOrder(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		src.SetRGBA(0, y, color.RGBA{R: 255, A: 255})
		src.SetRGBA(1, y, color.RGBA{G: 255, A: 255})
	}

	got, err := Render(src, Options{TargetWidth: 2, Threshold: 128})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := string(rune(brailleBase+0x47)) +
		"\n\n" + string(rune(brailleBase+0xB8)) +
		"\n\n" + string(rune(brailleBase)) +
		"\n\n" + string(rune(brailleBase+0xFF))
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestRenderLayersOutputsRGBMetadataInOrder(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		src.SetRGBA(0, y, color.RGBA{R: 255, A: 255})
		src.SetRGBA(1, y, color.RGBA{G: 255, A: 255})
	}

	got, err := RenderLayers(src, Options{TargetWidth: 2, Threshold: 128})
	if err != nil {
		t.Fatalf("RenderLayers() error = %v", err)
	}

	if len(got) != 4 {
		t.Fatalf("len(RenderLayers()) = %d, want 4", len(got))
	}

	wantMeta := []struct {
		id int
		z  int
		x  int
		y  int
	}{
		{id: 1, z: 10, x: 0, y: 0},
		{id: 2, z: 11, x: 0, y: 0},
		{id: 3, z: 12, x: 0, y: 0},
		{id: 4, z: 13, x: 0, y: 0},
	}
	for i, layer := range got {
		if layer.ID != wantMeta[i].id || layer.ZIndex != wantMeta[i].z {
			t.Fatalf("layer[%d] metadata = %+v, want ID=%d ZIndex=%d", i, layer, wantMeta[i].id, wantMeta[i].z)
		}
		if layer.XOffset != wantMeta[i].x || layer.YOffset != wantMeta[i].y || layer.Alpha != 1 || layer.Blend != "additive" {
			t.Fatalf("layer[%d] defaults = %+v", i, layer)
		}
	}

	if got[0].Data != string(rune(brailleBase+0x47)) {
		t.Fatalf("red layer = %q", got[0].Data)
	}
	if got[1].Data != string(rune(brailleBase+0xB8)) {
		t.Fatalf("green layer = %q", got[1].Data)
	}
	if got[2].Data != string(rune(brailleBase)) {
		t.Fatalf("blue layer = %q", got[2].Data)
	}
	if got[3].Data != string(rune(brailleBase+0xFF)) {
		t.Fatalf("luma layer = %q", got[3].Data)
	}
}

func TestRenderGrayscale(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		src.SetRGBA(0, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		src.SetRGBA(1, y, color.RGBA{A: 255})
	}

	got, err := RenderGrayscale(src, Options{TargetWidth: 2, Threshold: 128})
	if err != nil {
		t.Fatalf("RenderGrayscale() error = %v", err)
	}

	want := string(rune(brailleBase + 0x47))
	if got != want {
		t.Fatalf("RenderGrayscale() = %q, want %q", got, want)
	}
}

func TestRenderGrayscalePreservesChromaDetail(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		src.SetRGBA(0, y, color.RGBA{B: 255, A: 255})
		src.SetRGBA(1, y, color.RGBA{A: 255})
	}

	got, err := RenderGrayscale(src, Options{TargetWidth: 2, Threshold: 128})
	if err != nil {
		t.Fatalf("RenderGrayscale() error = %v", err)
	}

	want := string(rune(brailleBase + 0x47))
	if got != want {
		t.Fatalf("RenderGrayscale() chroma detail = %q, want %q", got, want)
	}
}
