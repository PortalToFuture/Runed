package braille

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"
)

const brailleBase = 0x2800

var dotBits = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

var defaultLayerSpecs = []struct {
	id      int
	zIndex  int
	xOffset int
	yOffset int
	channel channel
}{
	{id: 1, zIndex: 10, xOffset: 0, yOffset: 0, channel: channelRed},
	{id: 2, zIndex: 11, xOffset: 0, yOffset: 0, channel: channelGreen},
	{id: 3, zIndex: 12, xOffset: 0, yOffset: 0, channel: channelBlue},
	{id: 4, zIndex: 13, xOffset: 0, yOffset: 0, channel: channelGray},
}

type Options struct {
	TargetWidth int
	Threshold   uint8
	Invert      bool
	Colorize    bool
}

type Layer struct {
	ID      int
	ZIndex  int
	XOffset int
	YOffset int
	Alpha   float64
	Blend   string
	Data    string
}

type channel int

const (
	channelRed channel = iota
	channelGreen
	channelBlue
	channelGray
)

func Render(src image.Image, opts Options) (string, error) {
	layers, err := RenderLayers(src, opts)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, len(layers))
	for _, layer := range layers {
		parts = append(parts, layer.Data)
	}

	return strings.Join(parts, "\n\n"), nil
}

func RenderGrayscale(src image.Image, opts Options) (string, error) {
	grid, err := rasterizeToGrid(src, opts, channelGray)
	if err != nil {
		return "", err
	}

	return encode(grid, opts, channelGray), nil
}

func RenderLayers(src image.Image, opts Options) ([]Layer, error) {
	_, targetWidth, targetHeight, err := resolveTargetSize(src, opts)
	if err != nil {
		return nil, err
	}

	layers := make([]Layer, 0, len(defaultLayerSpecs))
	for _, spec := range defaultLayerSpecs {
		grid := rasterize(src, targetWidth, targetHeight, spec.channel)
		layers = append(layers, Layer{
			ID:      spec.id,
			ZIndex:  spec.zIndex,
			XOffset: spec.xOffset,
			YOffset: spec.yOffset,
			Alpha:   1,
			Blend:   "additive",
			Data:    encode(grid, opts, spec.channel),
		})
	}

	return layers, nil
}

func resolveTargetSize(src image.Image, opts Options) (image.Rectangle, int, int, error) {
	if src == nil {
		return image.Rectangle{}, 0, 0, errors.New("image is nil")
	}

	bounds := src.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return image.Rectangle{}, 0, 0, errors.New("image has no pixels")
	}

	targetWidth := opts.TargetWidth
	if targetWidth <= 0 {
		targetWidth = bounds.Dx()
	}
	targetWidth = roundUp(targetWidth, 2)

	targetHeight := scaleHeight(bounds.Dx(), bounds.Dy(), targetWidth)
	targetHeight = roundUp(targetHeight, 4)

	return bounds, targetWidth, targetHeight, nil
}

func rasterizeToGrid(src image.Image, opts Options, c channel) ([][]uint8, error) {
	_, targetWidth, targetHeight, err := resolveTargetSize(src, opts)
	if err != nil {
		return nil, err
	}

	return rasterize(src, targetWidth, targetHeight, c), nil
}

func encode(grid [][]uint8, opts Options, c channel) string {
	if len(grid) == 0 || len(grid[0]) == 0 {
		return ""
	}

	var b strings.Builder
	cellRows := len(grid) / 4
	cellCols := len(grid[0]) / 2
	b.Grow(cellRows * (cellCols + 1) * 3)

	for cellY := 0; cellY < len(grid); cellY += 4 {
		for cellX := 0; cellX < len(grid[0]); cellX += 2 {
			var mask rune
			var total int
			for y := 0; y < 4; y++ {
				for x := 0; x < 2; x++ {
					value := grid[cellY+y][cellX+x]
					total += int(value)
					if active(value, opts) {
						mask |= dotBits[y][x]
					}
				}
			}
			if opts.Colorize {
				shade := uint8(total / 8)
				writeANSIColor(&b, shade, c)
			}
			b.WriteRune(brailleBase + mask)
		}
		if opts.Colorize {
			b.WriteString("\x1b[0m")
		}
		if cellY+4 < len(grid) {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func active(value uint8, opts Options) bool {
	on := value >= opts.Threshold
	if opts.Invert {
		return !on
	}
	return on
}

func writeANSIColor(b *strings.Builder, shade uint8, c channel) {
	var rgb [3]uint8
	if c == channelGray {
		rgb = [3]uint8{shade, shade, shade}
	} else {
		rgb[c] = shade
	}
	fmt.Fprintf(b, "\x1b[38;2;%d;%d;%dm", rgb[0], rgb[1], rgb[2])
}

func rasterize(src image.Image, width, height int, c channel) [][]uint8 {
	grid := make([][]uint8, height)
	bounds := src.Bounds()

	for y := 0; y < height; y++ {
		row := make([]uint8, width)
		srcY := bounds.Min.Y + y*bounds.Dy()/height
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + x*bounds.Dx()/width
			row[x] = component(src.At(srcX, srcY), c)
		}
		grid[y] = row
	}

	return grid
}

func component(c color.Color, component channel) uint8 {
	r, g, b, _ := c.RGBA()
	if component == channelGray {
		// Preserve saturated detail from any source channel in the grayscale fallback.
		maxValue := maxUint32(uint32(r), uint32(g))
		maxValue = maxUint32(maxValue, uint32(b))
		return uint8(maxValue >> 8)
	}
	values := [3]uint32{uint32(r), uint32(g), uint32(b)}
	return uint8(values[component] >> 8)
}

func maxUint32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func scaleHeight(srcWidth, srcHeight, targetWidth int) int {
	height := srcHeight * targetWidth / srcWidth
	if height < 1 {
		return 1
	}
	return height
}

func roundUp(value, multiple int) int {
	if value%multiple == 0 {
		return value
	}
	return value + multiple - value%multiple
}
