package braille

import (
	"errors"
	"image"
	"image/color"
	"math"
	"strings"
)

const brailleBase = 0x2800

var dotBits = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

var ditherRanks = [4][2]uint8{
	{0, 4},
	{6, 2},
	{3, 7},
	{5, 1},
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
	TargetColumns    int
	TargetWidth      int
	ResolutionScale  int
	SupersampleScale int
	Threshold        uint8
	Invert           bool
}

type LayerPayload struct {
	ID      int
	ZIndex  int
	XOffset int
	YOffset int
	Alpha   uint8
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

	targetWidth := resolveTargetWidth(bounds, opts)

	targetHeight := scaleHeight(bounds.Dx(), bounds.Dy(), targetWidth)
	targetHeight = roundUp(targetHeight, 4)

	layers := make([]LayerPayload, 0, len(defaultLayerSpecs))
	for _, spec := range defaultLayerSpecs {
		grid := rasterize(src, targetWidth, targetHeight, spec.channel, opts)
		layers = append(layers, LayerPayload{
			ID:      spec.id,
			ZIndex:  spec.zIndex,
			XOffset: spec.xOffset,
			YOffset: spec.yOffset,
			Alpha:   160,
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
					if active(grid[cellY+y][cellX+x], x, y, opts) {
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

func active(value uint8, x, y int, opts Options) bool {
	level := int(value) + 128 - int(opts.Threshold)
	if level < 0 {
		level = 0
	} else if level > 255 {
		level = 255
	}

	on := level > ditherThreshold(x, y)
	if opts.Invert {
		return !on
	}
	return on
}

func ditherThreshold(x, y int) int {
	rank := int(ditherRanks[y][x])
	return rank * 255 / 8
}

func rasterize(src image.Image, width, height int, c channel, opts Options) [][]uint8 {
	grid := make([][]uint8, height)
	bounds := src.Bounds()
	supersample := resolveSupersampleScale(opts.SupersampleScale)

	for y := 0; y < height; y++ {
		row := make([]uint8, width)
		for x := 0; x < width; x++ {
			row[x] = sampleComponent(src, bounds, width, height, x, y, c, supersample)
		}
		grid[y] = row
	}

	return grid
}

func sampleComponent(
	src image.Image,
	bounds image.Rectangle,
	width, height, x, y int,
	componentID channel,
	supersample int,
) uint8 {
	cellWidth := float64(bounds.Dx()) / float64(width)
	cellHeight := float64(bounds.Dy()) / float64(height)
	startX := float64(bounds.Min.X) + float64(x)*cellWidth
	startY := float64(bounds.Min.Y) + float64(y)*cellHeight

	var total float64
	sampleCount := supersample * supersample
	for sampleY := 0; sampleY < supersample; sampleY++ {
		py := startY + (float64(sampleY)+0.5)*cellHeight/float64(supersample)
		srcY := clampCoordinate(int(math.Floor(py)), bounds.Min.Y, bounds.Max.Y-1)
		for sampleX := 0; sampleX < supersample; sampleX++ {
			px := startX + (float64(sampleX)+0.5)*cellWidth/float64(supersample)
			srcX := clampCoordinate(int(math.Floor(px)), bounds.Min.X, bounds.Max.X-1)
			total += float64(component(src.At(srcX, srcY), componentID))
		}
	}

	return uint8(math.Round(total / float64(sampleCount)))
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

func resolveTargetWidth(bounds image.Rectangle, opts Options) int {
	if opts.TargetColumns > 0 {
		return roundUp(opts.TargetColumns*2, 2)
	}

	targetWidth := opts.TargetWidth
	if targetWidth <= 0 {
		targetWidth = bounds.Dx()
	}

	targetWidth *= resolveResolutionScale(opts.ResolutionScale)
	return roundUp(targetWidth, 2)
}

func resolveResolutionScale(scale int) int {
	if scale <= 0 {
		return 1
	}
	return scale
}

func resolveSupersampleScale(scale int) int {
	if scale <= 0 {
		return 2
	}
	return scale
}

func clampCoordinate(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
