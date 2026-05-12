# Braille-matrix Display

## ELI5 Summary

This little project takes a normal picture and rebuilds it out of Braille characters so a terminal can "draw" it using text. Then it splits the picture into red, green, and blue layers and ships those layers with custom terminal commands, which is basically a clever way of faking graphics in a place that normally only knows about letters.

Right now it is a working experiment with a clear shape: it has a runnable Go CLI, supports still images and GIF frames, emits `OSC 9180` layer data, and even prints a grayscale fallback for terminals that do not understand the new protocol. It is still early-stage, though, since the README calls out simple thresholding and no dithering yet.

Braille-matrix display experiments in Go.

The current pass treats UTF-8 Braille glyphs as a terminal pixel surface,
renders three separate channel layers from the source image in red, green,
blue order, and emits them as Matrix9180 `OSC 9180` commands.

## Run

```bash
go run ./cmd/runed --width 96 --threshold 128 path/to/image.png
```

Flags:

- `--width`: target width in pixels before Braille packing
- `--threshold`: channel threshold, `0-255`
- `--invert`: restore dark-pixel activation if you want the negative
- `--no-color`: disable ANSI per-channel shading for `--plain`
- `--plain`: bypass `OSC 9180` and print raw layer text
- `--escape-osc`: escape `OSC 9180` control bytes for inspection

## Protocol output

Default output is a single frame encoded as raw `OSC 9180` control bytes:

- `ESC ] 9180 ; LAYER_START;id=<n>;z=<z>;alpha=<a>;blend=additive BEL`
- `ESC ] 9180 ; OFFSET;id=<n>;x=<offset>;y=<offset> BEL`
- `ESC ] 9180 ; DATA;id=<n>;<utf8 braille payload> BEL`
- `ESC ] 9180 ; LAYER_END;id=<n> BEL`
- `ESC ] 9180 ; FRAME_END BEL`

Use `--escape-osc` when you want to inspect the stream as visible `\x1b` and
`\x07` text. Protocol-mode `DATA` contains raw Braille glyphs only, with no
ANSI escapes.

After `FRAME_END`, `runed` emits a plain grayscale Braille frame as a fallback
for terminals that do not implement `OSC 9180`. That fallback is derived from
the strongest source channel per pixel so saturated color detail survives the
collapse better than with luminance weighting.

## Notes

- Output width is rounded to a multiple of `2`.
- Output height is scaled to preserve the source aspect ratio, then rounded to a
  multiple of `4`.
- Output is emitted as three layers in `rgb` order.
- A grayscale fallback frame is appended below the protocol output.
- Layer ids and z-order currently default to `0`, `1`, `2`.
- Default CRT-style offsets are `r=(-1,0)`, `g=(0,0)`, `b=(1,0)`.
- `--plain` can shade each Braille cell with its average channel value using
  ANSI foreground color for that channel.
- This version is binary thresholding for dot activation only. No dithering yet.
