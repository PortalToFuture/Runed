package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"runed/bd/internal/braille"
)

const (
	oscPrefix = "\x1b]9180;"
	oscSuffix = "\x07"
)

func main() {
	var (
		width     = flag.Int("width", 96, "target output width in pixels; rounded to a multiple of 2")
		threshold = flag.Uint("threshold", 128, "channel threshold from 0-255")
		invert    = flag.Bool("invert", false, "invert the luminance test")
		noColor   = flag.Bool("no-color", false, "disable ANSI per-channel shading for --plain output")
		plain     = flag.Bool("plain", false, "emit raw layer text instead of OSC 9180 protocol")
		escapeOSC = flag.Bool("escape-osc", false, "escape OSC 9180 control bytes for inspection")
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

	colorize := *plain && !*noColor

	layers, err := braille.RenderLayers(src, braille.Options{
		TargetWidth: *width,
		Threshold:   uint8(*threshold),
		Invert:      *invert,
		Colorize:    colorize,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "render braille: %v\n", err)
		os.Exit(1)
	}

	if *plain {
		payloads := make([]string, 0, len(layers))
		for _, layer := range layers {
			payloads = append(payloads, layer.Data)
		}
		fmt.Print(strings.Join(payloads, "\n\n"))
		return
	}

	frame := encodeFrame(layers)
	fallback, err := braille.RenderGrayscale(src, braille.Options{
		TargetWidth: *width,
		Threshold:   uint8(*threshold),
		Invert:      *invert,
		Colorize:    false,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "render grayscale fallback: %v\n", err)
		os.Exit(1)
	}

	if *escapeOSC {
		fmt.Print(escapeControlStream(frame))
		fmt.Print("\n")
		fmt.Print(fallback)
		return
	}

	fmt.Print(frame)
	fmt.Print("\n")
	fmt.Print(fallback)
}

func encodeFrame(layers []braille.Layer) string {
	var b strings.Builder
	for _, layer := range layers {
		fmt.Fprintf(&b, "%sLAYER_START;id=%d;z=%d;alpha=%g;blend=%s%s", oscPrefix, layer.ID, layer.ZIndex, layer.Alpha, layer.Blend, oscSuffix)
		fmt.Fprintf(&b, "%sOFFSET;id=%d;x=%d;y=%d%s", oscPrefix, layer.ID, layer.XOffset, layer.YOffset, oscSuffix)
		fmt.Fprintf(&b, "%sDATA;id=%d;%s%s", oscPrefix, layer.ID, layer.Data, oscSuffix)
		fmt.Fprintf(&b, "%sLAYER_END;id=%d%s", oscPrefix, layer.ID, oscSuffix)
	}
	b.WriteString(oscPrefix)
	b.WriteString("FRAME_END")
	b.WriteString(oscSuffix)

	return b.String()
}

func escapeControlStream(stream string) string {
	var b strings.Builder
	b.Grow(len(stream) + len(stream)/8)

	for _, r := range stream {
		switch r {
		case '\x1b':
			b.WriteString(`\x1b`)
		case '\a':
			b.WriteString(`\x07`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}
