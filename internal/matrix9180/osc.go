package matrix9180

import (
	"fmt"
	"io"

	"runed.ghostty/internal/braille"
)

const (
	oscPrefix = "\x1b]9180;"
	oscSuffix = "\x07"
)

func WriteFrame(w io.Writer, layers []braille.LayerPayload) error {
	for _, layer := range layers {
		if err := writeCommand(w, "LAYER_START;id=%d;z=%d", layer.ID, layer.ZIndex); err != nil {
			return err
		}
		if err := writeCommand(w, "OFFSET;id=%d;x=%d;y=%d", layer.ID, layer.XOffset, layer.YOffset); err != nil {
			return err
		}
		if err := writeCommand(w, "DATA;id=%d;%s", layer.ID, layer.Data); err != nil {
			return err
		}
		if err := writeCommand(w, "LAYER_END;id=%d", layer.ID); err != nil {
			return err
		}
	}

	return writeCommand(w, "FRAME_END")
}

func writeCommand(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, oscPrefix+format+oscSuffix, args...)
	return err
}
