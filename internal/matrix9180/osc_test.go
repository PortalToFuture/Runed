package matrix9180

import (
	"strings"
	"testing"

	"runed.ghostty/internal/braille"
)

func TestWriteFrame(t *testing.T) {
	layers := []braille.LayerPayload{
		{ID: 1, ZIndex: 10, XOffset: -1, YOffset: 0, Alpha: 160, Data: "C"},
		{ID: 2, ZIndex: 11, XOffset: 0, YOffset: 0, Alpha: 160, Data: "M"},
		{ID: 3, ZIndex: 12, XOffset: 1, YOffset: 0, Alpha: 160, Data: "Y"},
		{ID: 4, ZIndex: 13, XOffset: 2, YOffset: 0, Alpha: 160, Data: "K"},
	}

	var b strings.Builder
	if err := WriteFrame(&b, layers); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}

	want := "" +
		"\x1b]9180;LAYER_START;id=1;z=10;alpha=160\x07" +
		"\x1b]9180;OFFSET;id=1;x=-1;y=0\x07" +
		"\x1b]9180;DATA;id=1;C\x07" +
		"\x1b]9180;LAYER_END;id=1\x07" +
		"\x1b]9180;LAYER_START;id=2;z=11;alpha=160\x07" +
		"\x1b]9180;OFFSET;id=2;x=0;y=0\x07" +
		"\x1b]9180;DATA;id=2;M\x07" +
		"\x1b]9180;LAYER_END;id=2\x07" +
		"\x1b]9180;LAYER_START;id=3;z=12;alpha=160\x07" +
		"\x1b]9180;OFFSET;id=3;x=1;y=0\x07" +
		"\x1b]9180;DATA;id=3;Y\x07" +
		"\x1b]9180;LAYER_END;id=3\x07" +
		"\x1b]9180;LAYER_START;id=4;z=13;alpha=160\x07" +
		"\x1b]9180;OFFSET;id=4;x=2;y=0\x07" +
		"\x1b]9180;DATA;id=4;K\x07" +
		"\x1b]9180;LAYER_END;id=4\x07" +
		"\x1b]9180;FRAME_END\x07"

	if b.String() != want {
		t.Fatalf("WriteFrame() = %q, want %q", b.String(), want)
	}
}
