# Runed Monorepo

This repository uses the Ghostty fork history as the primary git history so
pulling from `upstream` remains straightforward.

## Layout

- repository root: Ghostty fork working tree
- `bd/`: Braille-matrix display project in Go

## Status

The active fork work is currently centered on `bd/` and the custom
`OSC 9180` protocol path. The recent baseline is:

- animated frame output for still images and GIF sources
- four-layer `CMYK` Braille emission instead of three-layer `RGB`
- viewer-side alpha-aware compositing for local inspection
- a regression test that keeps the embedded viewer samples aligned with the
  Go protocol writer

## Near Term

- tune `CMYK` decomposition, especially black-layer balance
- keep the local docs and viewer fixtures in sync with protocol behavior
- continue proving the protocol path against the terminal side in real use

## Git Strategy

- `origin`: Runed fork
- `upstream`: Ghostty upstream

Keep Ghostty's source tree at the repository root. That avoids a large path
rewrite and keeps future merges and upstream patch submission practical.
