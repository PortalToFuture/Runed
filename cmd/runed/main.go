package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"time"

	"runed.ghostty/internal/braille"
	"runed.ghostty/internal/matrix9180"
)

func main() {
	var (
		cols        = flag.Int("cols", 300, "final braille grid width in character cells")
		width       = flag.Int("width", 120, "base output width in pixels before braille packing; ignored when --cols is set")
		matrixScale = flag.Int("matrix-scale", 2, "multiplier for the braille matrix resolution before packing when using --width")
		supersample = flag.Int("supersample", 2, "box-filter samples per output axis when rasterizing each braille dot")
		threshold   = flag.Uint("threshold", 128, "channel threshold from 0-255")
		invert      = flag.Bool("invert", false, "invert the channel threshold test")
		frames      = flag.Int("frames", 1, "number of animation frames to emit for still images")
		fps         = flag.Float64("fps", 12, "frame rate for multi-frame animation output")
		revolveFade = flag.Bool("revolve-fade", false, "phase-shift CMYK layer alpha to create a revolving fade animation")
	)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <image>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	data, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open image: %v\n", err)
		os.Exit(1)
	}

	srcFrames, err := decodeFrames(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode image: %v\n", err)
		os.Exit(1)
	}

	opts := braille.Options{
		TargetColumns:    *cols,
		TargetWidth:      *width,
		ResolutionScale:  *matrixScale,
		SupersampleScale: *supersample,
		Threshold:        uint8(*threshold),
		Invert:           *invert,
	}

	emitFrames := *frames
	if len(srcFrames) > 1 {
		emitFrames = len(srcFrames)
	}

	if err := writeAnimation(os.Stdout, srcFrames, opts, animationOptions{
		Frames:      emitFrames,
		FPS:         *fps,
		RevolveFade: *revolveFade,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "write frame: %v\n", err)
		os.Exit(1)
	}
}

type animationOptions struct {
	Frames      int
	FPS         float64
	RevolveFade bool
}

func decodeFrames(data []byte) ([]image.Image, error) {
	if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil && len(g.Image) > 0 {
		frames := make([]image.Image, 0, len(g.Image))
		for _, frame := range g.Image {
			frames = append(frames, frame)
		}
		return frames, nil
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return []image.Image{src}, nil
}

func writeAnimation(w io.Writer, srcFrames []image.Image, opts braille.Options, anim animationOptions) error {
	if len(srcFrames) == 0 {
		return nil
	}

	frameCount := anim.Frames
	if frameCount <= 0 {
		frameCount = 1
	}

	var frameDelay time.Duration
	if anim.FPS > 0 {
		frameDelay = time.Duration(float64(time.Second) / anim.FPS)
	}

	for i := 0; i < frameCount; i++ {
		src := srcFrames[i%len(srcFrames)]
		layers, err := braille.RenderLayers(src, opts)
		if err != nil {
			return err
		}
		if anim.RevolveFade {
			applyRevolvingFade(layers, i, frameCount)
		}
		if err := matrix9180.WriteFrame(w, layers); err != nil {
			return err
		}
		if frameDelay > 0 && i+1 < frameCount {
			time.Sleep(frameDelay)
		}
	}

	return nil
}

func applyRevolvingFade(layers []braille.LayerPayload, frameIndex, frameCount int) {
	if frameCount <= 1 {
		return
	}

	const (
		minAlphaFactor = 0.18
		maxAlphaFactor = 1.0
	)

	phaseBase := 2 * math.Pi * float64(frameIndex) / float64(frameCount)
	for i := range layers {
		phase := phaseBase + 2*math.Pi*float64(i)/float64(len(layers))
		fade := 0.5 + 0.5*math.Sin(phase)
		alphaFactor := minAlphaFactor + (maxAlphaFactor-minAlphaFactor)*fade
		layers[i].Alpha = uint8(math.Round(float64(layers[i].Alpha) * alphaFactor))
	}
}
