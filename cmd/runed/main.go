package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

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

	file, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open image: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	src, _, err := image.Decode(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode image: %v\n", err)
		os.Exit(1)
	}

	layers, err := braille.RenderLayers(src, braille.Options{
		TargetColumns:    *cols,
		TargetWidth:      *width,
		ResolutionScale:  *matrixScale,
		SupersampleScale: *supersample,
		Threshold:        uint8(*threshold),
		Invert:           *invert,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "render braille: %v\n", err)
		os.Exit(1)
	}

	if err := matrix9180.WriteFrame(os.Stdout, layers); err != nil {
		fmt.Fprintf(os.Stderr, "write frame: %v\n", err)
		os.Exit(1)
	}
}
