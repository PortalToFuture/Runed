package matrix9180

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"runed.ghostty/internal/braille"
)

var sampleBlockRe = regexp.MustCompile(`const (sampleFrame|sampleFrameLuma) = \[(?s:(.*?))\]\.join\("\\n"\);`)
var quotedLineRe = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

func TestViewerSampleFrameMatchesProtocolWriter(t *testing.T) {
	html := readViewerHTML(t)
	got := extractSampleLines(t, html, "sampleFrame")

	layers := []braille.LayerPayload{
		{ID: 1, ZIndex: 10, XOffset: 2, YOffset: 1, Alpha: 160, Data: "⣿⣶⣀"},
		{ID: 2, ZIndex: 11, XOffset: 2, YOffset: 1, Alpha: 112, Data: "⣀⣿⣶"},
		{ID: 3, ZIndex: 12, XOffset: 2, YOffset: 1, Alpha: 96, Data: "⢀⣾⣿"},
		{ID: 4, ZIndex: 13, XOffset: 3, YOffset: 2, Alpha: 72, Data: "⣀⣀⣤"},
	}

	assertViewerSampleMatchesLayers(t, got, layers)
}

func TestViewerSampleFrameLumaMatchesProtocolWriter(t *testing.T) {
	html := readViewerHTML(t)
	got := extractSampleLines(t, html, "sampleFrameLuma")

	layers := []braille.LayerPayload{
		{ID: 1, ZIndex: 10, XOffset: 2, YOffset: 1, Alpha: 160, Data: "⢀⣴⣿⣦"},
		{ID: 2, ZIndex: 11, XOffset: 3, YOffset: 1, Alpha: 140, Data: "⣾⣿⣤⡀"},
		{ID: 3, ZIndex: 12, XOffset: 2, YOffset: 1, Alpha: 120, Data: "⣀⣶⣿⣶"},
		{ID: 4, ZIndex: 13, XOffset: 2, YOffset: 1, Alpha: 96, Data: "⢀⣾⣿⣿⣷\n⢸⣿⣶⣶⡇\n⠈⢿⣿⡿⠁"},
	}

	assertViewerSampleMatchesLayers(t, got, layers)
}

func readViewerHTML(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "matrix9180_viewer.html")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read viewer html: %v", err)
	}
	return string(data)
}

func extractSampleLines(t *testing.T, html, name string) []string {
	t.Helper()

	matches := sampleBlockRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if match[1] != name {
			continue
		}

		block := match[2]
		quoted := quotedLineRe.FindAllStringSubmatch(block, -1)
		lines := make([]string, 0, len(quoted))
		for _, q := range quoted {
			line, err := strconv.Unquote(`"` + q[1] + `"`)
			if err != nil {
				t.Fatalf("unquote %s line: %v", name, err)
			}
			lines = append(lines, line)
		}
		return lines
	}

	t.Fatalf("sample %s not found", name)
	return nil
}

func assertViewerSampleMatchesLayers(t *testing.T, got []string, layers []braille.LayerPayload) {
	t.Helper()

	var b strings.Builder
	if err := WriteFrame(&b, layers); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}

	want := extractOSCCommands(b.String())
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("viewer sample mismatch\n got: %q\nwant: %q", got, want)
	}
}

func extractOSCCommands(raw string) []string {
	parts := strings.Split(raw, oscPrefix)
	commands := make([]string, 0, len(parts))
	for _, part := range parts[1:] {
		if idx := strings.Index(part, oscSuffix); idx >= 0 {
			commands = append(commands, part[:idx])
		}
	}
	return commands
}
