# Runed Monorepo

This repository uses the Ghostty fork history as the primary git history so
pulling from `upstream` remains straightforward.

## Layout

- repository root: Ghostty fork working tree
- `bd/`: Braille display experiments in Go

## Git Strategy

- `origin`: Runed fork
- `upstream`: Ghostty upstream

Keep Ghostty's source tree at the repository root. That avoids a large path
rewrite and keeps future merges and upstream patch submission practical.
