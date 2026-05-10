package braille

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestEncodeDotMapping(t *testing.T) {
	grid := [][]uint8{
		{255, 0},
		{0, 255},
		{255, 0},
		{0, 255},
	}

	got := encode(grid, Options{Threshold: 128})
	want := string(rune(brailleBase + 0x01 + 0x10 + 0x04 + 0x80))
	if got != want {
		t.Fatalf("encode() = %q, want %q", got, want)
	}
}

func TestEncodeMidtoneUsesPartialDitherCoverage(t *testing.T) {
	grid := [][]uint8{
		{128, 128},
		{128, 128},
		{128, 128},
		{128, 128},
	}

	got := encode(grid, Options{Threshold: 128})
	want := string(rune(brailleBase + 0x01 + 0x04 + 0x10 + 0x08 + 0x80))
	if got != want {
		t.Fatalf("encode() midtone = %q, want %q", got, want)
	}
}

func TestEncodeInvert(t *testing.T) {
	grid := [][]uint8{
		{0, 0},
		{0, 0},
		{0, 0},
		{0, 0},
	}

	got := encode(grid, Options{Threshold: 128, Invert: true})
	want := string(rune(brailleBase + 0xFF))
	if got != want {
		t.Fatalf("encode() with invert = %q, want %q", got, want)
	}
}

func TestRenderLayersOutputsFixedRGBMetadata(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		src.SetRGBA(0, y, color.RGBA{R: 255, A: 255})
		src.SetRGBA(1, y, color.RGBA{G: 255, A: 255})
	}

	got, err := RenderLayers(src, Options{TargetWidth: 2, Threshold: 128})
	if err != nil {
		t.Fatalf("RenderLayers() error = %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("len(RenderLayers()) = %d, want 3", len(got))
	}

	wantMeta := []struct {
		id      int
		zIndex  int
		xOffset int
		yOffset int
		data    string
	}{
		{id: 1, zIndex: 10, xOffset: 0, yOffset: 0, data: string(rune(brailleBase + 0x47))},
		{id: 2, zIndex: 11, xOffset: 0, yOffset: 0, data: string(rune(brailleBase + 0xB8))},
		{id: 3, zIndex: 12, xOffset: 0, yOffset: 0, data: string(rune(brailleBase))},
	}

	for i, want := range wantMeta {
		layer := got[i]
		if layer.ID != want.id || layer.ZIndex != want.zIndex {
			t.Fatalf("layer[%d] metadata = %+v", i, layer)
		}
		if layer.XOffset != want.xOffset || layer.YOffset != want.yOffset {
			t.Fatalf("layer[%d] offsets = %+v", i, layer)
		}
		if layer.Alpha != 160 {
			t.Fatalf("layer[%d] alpha = %d, want 160", i, layer.Alpha)
		}
		if layer.Data != want.data {
			t.Fatalf("layer[%d] data = %q, want %q", i, layer.Data, want.data)
		}
	}
}

func TestRenderLayersResolutionScaleIncreasesMatrixDensity(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 4))
	for y := 0; y < 4; y++ {
		src.SetRGBA(0, y, color.RGBA{R: 255, A: 255})
	}

	got, err := RenderLayers(src, Options{
		TargetWidth:     2,
		ResolutionScale: 2,
		Threshold:       128,
	})
	if err != nil {
		t.Fatalf("RenderLayers() error = %v", err)
	}

	lines := strings.Split(got[0].Data, "\n")
	if len(lines) != 2 {
		t.Fatalf("scaled red layer rows = %d, want 2 (%q)", len(lines), got[0].Data)
	}
	if lines[0] != "⣿⠀" || lines[1] != "⣿⠀" {
		t.Fatalf("scaled red layer = %q, want left-column detail in both rows", got[0].Data)
	}
}

func TestRenderLayersTargetColumnsControlsFinalGridWidth(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}

	got, err := RenderLayers(src, Options{
		TargetColumns:   3,
		TargetWidth:     2,
		ResolutionScale: 4,
		Threshold:       128,
	})
	if err != nil {
		t.Fatalf("RenderLayers() error = %v", err)
	}

	lines := strings.Split(got[0].Data, "\n")
	if len(lines) != 2 {
		t.Fatalf("grid rows = %d, want 2 (%q)", len(lines), got[0].Data)
	}
	if lines[0] != "⣿⣿⣿" || lines[1] != "⣿⣿⣿" {
		t.Fatalf("grid rows = %q, want two full rows of %q", got[0].Data, "⣿⣿⣿")
	}
}
