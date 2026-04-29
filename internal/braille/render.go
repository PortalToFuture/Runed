package braille

import (
	"errors"
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
}

type Options struct {
	TargetWidth int
	Threshold   uint8
	Invert      bool
}

type LayerPayload struct {
	ID      int
	ZIndex  int
	XOffset int
	YOffset int
	Data    string
}

type channel int

const (
	channelRed channel = iota
	channelGreen
	channelBlue
)

func RenderLayers(src image.Image, opts Options) ([]LayerPayload, error) {
	if src == nil {
		return nil, errors.New("image is nil")
	}

	bounds := src.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return nil, errors.New("image has no pixels")
	}

	targetWidth := opts.TargetWidth
	if targetWidth <= 0 {
		targetWidth = bounds.Dx()
	}
	targetWidth = roundUp(targetWidth, 2)

	targetHeight := scaleHeight(bounds.Dx(), bounds.Dy(), targetWidth)
	targetHeight = roundUp(targetHeight, 4)

	layers := make([]LayerPayload, 0, len(defaultLayerSpecs))
	for _, spec := range defaultLayerSpecs {
		grid := rasterize(src, targetWidth, targetHeight, spec.channel)
		layers = append(layers, LayerPayload{
			ID:      spec.id,
			ZIndex:  spec.zIndex,
			XOffset: spec.xOffset,
			YOffset: spec.yOffset,
			Data:    encode(grid, opts),
		})
	}

	return layers, nil
}

func encode(grid [][]uint8, opts Options) string {
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
			for y := 0; y < 4; y++ {
				for x := 0; x < 2; x++ {
					if active(grid[cellY+y][cellX+x], opts) {
						mask |= dotBits[y][x]
					}
				}
			}
			b.WriteRune(brailleBase + mask)
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
	values := [3]uint32{uint32(r), uint32(g), uint32(b)}
	return uint8(values[component] >> 8)
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
