# Braille-matrix Display

## ELI5 Summary

This little project takes a normal picture and rebuilds it out of Braille characters so a terminal can "draw" it using text. Then it splits the picture into cyan, magenta, yellow, and black layers and ships those layers with custom terminal commands, which is basically a clever way of faking graphics in a place that normally only knows about letters.

Right now it is a working experiment with a clearer shape than the first pass: it has a runnable Go CLI, supports still images and GIF frames, emits `OSC 9180` layer data, can phase-shift layer alpha for simple animation, and ships a local HTML viewer plus a regression test to keep the viewer sample stream honest. It is still early-stage, though, since the renderer is still using simple thresholding and the `CMYK` split is intentionally basic.

Braille-matrix display experiments in Go.

The current pass treats UTF-8 Braille glyphs as a terminal pixel surface,
renders four separate subtractive channel layers from the source image in
cyan, magenta, yellow, black order, and emits them as Matrix9180 `OSC 9180`
commands.

## Run

```bash
go run ./cmd/runed --width 96 --threshold 128 path/to/image.png
```

Flags:

- `--cols`: final output width in Braille character cells
- `--width`: target width in pixels before Braille packing
- `--matrix-scale`: multiplier for raster resolution before Braille packing
- `--supersample`: box-filter samples per output axis for each Braille dot
- `--threshold`: channel threshold, `0-255`
- `--invert`: restore dark-pixel activation if you want the negative
- `--frames`: number of animation frames to emit for still images
- `--fps`: frame rate for multi-frame output
- `--revolve-fade`: phase-shift layer alpha to animate the `CMYK` stack

## Protocol output

Default output is one or more frames encoded as raw `OSC 9180` control bytes:

- `ESC ] 9180 ; LAYER_START;id=<n>;z=<z>;alpha=<a> BEL`
- `ESC ] 9180 ; OFFSET;id=<n>;x=<offset>;y=<offset> BEL`
- `ESC ] 9180 ; DATA;id=<n>;<utf8 braille payload> BEL`
- `ESC ] 9180 ; LAYER_END;id=<n> BEL`
- `ESC ] 9180 ; FRAME_END BEL`

Protocol-mode `DATA` contains raw Braille glyphs only, with no ANSI escapes.
The current CLI always writes the protocol stream directly; local inspection of
sample frames happens through `matrix9180_viewer.html` and its fixture test.

## Notes

- Output width is rounded to a multiple of `2`.
- Output height is scaled to preserve the source aspect ratio, then rounded to a
  multiple of `4`.
- Output is emitted as four layers in `cmyk` order.
- Layer ids and z-order currently default to `1..4` and `10..13`.
- Current layer alpha defaults to `160` before any animation fade is applied.
- The local viewer understands layer alpha and composites `CMYK` previews from
  the emitted protocol data.
- This version is binary thresholding for dot activation only. No dithering yet.
