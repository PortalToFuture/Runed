package main

import (
	"testing"

	"runed.ghostty/internal/braille"
)

func TestApplyRevolvingFadeVariesLayerAlpha(t *testing.T) {
	layers := []braille.LayerPayload{
		{ID: 1, Alpha: 160},
		{ID: 2, Alpha: 160},
		{ID: 3, Alpha: 160},
	}

	applyRevolvingFade(layers, 1, 6)

	if layers[0].Alpha == 160 && layers[1].Alpha == 160 && layers[2].Alpha == 160 {
		t.Fatal("expected at least one layer alpha to change")
	}

	for i, layer := range layers {
		if layer.Alpha > 160 {
			t.Fatalf("layer[%d] alpha = %d, want <= 160", i, layer.Alpha)
		}
		if layer.Alpha < 28 {
			t.Fatalf("layer[%d] alpha = %d, want >= 28", i, layer.Alpha)
		}
	}
}

func TestApplyRevolvingFadeNoopForSingleFrame(t *testing.T) {
	layers := []braille.LayerPayload{
		{ID: 1, Alpha: 160},
		{ID: 2, Alpha: 160},
	}

	applyRevolvingFade(layers, 0, 1)

	for i, layer := range layers {
		if layer.Alpha != 160 {
			t.Fatalf("layer[%d] alpha = %d, want 160", i, layer.Alpha)
		}
	}
}
