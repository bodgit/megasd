package image

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
	"sort"

	"github.com/ericpauley/go-quantize/quantize"
)

type encoder struct {
	w io.Writer
}

type paletteMap struct {
	palette color.Palette
	tiles   []byte
}

type byPaletteSize []paletteMap

func (p byPaletteSize) Len() int {
	return len(p)
}

func (p byPaletteSize) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p byPaletteSize) Less(i, j int) bool {
	return len(p[i].palette) < len(p[j].palette)
}

func countColors(m *image.Paletted, r image.Rectangle) map[color.Color]int {
	colors := make(map[color.Color]int)
	for y := r.Bounds().Min.Y; y < r.Bounds().Max.Y; y++ {
		for x := r.Bounds().Min.X; x < r.Bounds().Max.X; x++ {
			colors[m.At(x, y)]++
		}
	}
	return colors
}

func uniqueColors(m *image.Paletted, r image.Rectangle) color.Palette {
	h := countColors(m, r)
	p := make(color.Palette, 0, len(h))
	for c := range h {
		p = append(p, c)
	}
	return p
}

// Copied from color.sqDiff
func sqDiff(x, y uint32) uint32 {
	d := x - y
	return (d * d) >> 2
}

// Return the two closest colors in a given palette
func closestColors(p color.Palette) (color.Color, color.Color) {
	var rc1, rc2 color.Color
	bestSum := uint32(1<<32 - 1)
	for i, c1 := range p {
		r1, g1, b1, a1 := c1.RGBA()
		for j, c2 := range p {
			r2, g2, b2, a2 := c2.RGBA()
			if i != j { // Ignore comparing ourselves
				sum := sqDiff(r1, r2) + sqDiff(g1, g2) + sqDiff(b1, b2) + sqDiff(a1, a2)
				if sum < bestSum {
					bestSum, rc1, rc2 = sum, c1, c2
				}
			}
		}
	}
	return rc1, rc2
}

// Replace all occurrences of one color in an image with another
func replaceColor(m *image.Paletted, o, n color.Color) {
	for y := m.Bounds().Min.Y; y < m.Bounds().Max.Y; y++ {
		for x := m.Bounds().Min.X; x < m.Bounds().Max.X; x++ {
			if m.At(x, y) == o {
				m.Set(x, y, n)
			}
		}
	}
}

// Colors in p2 but not in p1
func paletteDifference(p1, p2 color.Palette) (d color.Palette) {
	m := make(map[color.Color]struct{})
	for _, c := range p1 {
		m[c] = struct{}{}
	}
	for _, c := range p2 {
		if _, ok := m[c]; !ok {
			d = append(d, c)
		}
	}
	return
}

// Variation of bin-packing problem; maxPalettes number of bins each with
// capacity of colorsPerPalette. Based on First Fit Decreasing algorithm;
// relies on the incoming palettes being sorted in decreasing size
func packPalette(in, out []paletteMap) ([]paletteMap, bool) {
	switch {
	case len(out) == 0: // First step, use the first (biggest) palette
		return packPalette(in[1:], append(out, in[0]))
	case len(in) == 0: // Finished, is it using three palettes or less?
		return out, len(out) <= maxPalettes
	default:
		// Loop over each current bin (palette)
		for i := range out {
			d := paletteDifference(out[i].palette, in[0].palette)

			// Either the candidate palette is a subset or the
			// difference can fit in the current palette
			if len(d) == 0 || len(d)+len(out[i].palette) <= colorsPerPalette {
				dup := append(out[:0:0], out...)
				if len(d) > 0 {
					dup[i].palette = append(dup[i].palette, d...)
				}
				dup[i].tiles = append(dup[i].tiles, in[0].tiles...)
				if ret, ok := packPalette(in[1:], dup); ok {
					return ret, true
				}
			}
		}
		// Last resort, start a new bin (palette)
		return packPalette(in[1:], append(out, in[0]))
	}
}

func padPalette(p color.Palette) color.Palette {
	// Pad palette to multiple of colorsPerPalette
	if mod := len(p) % colorsPerPalette; mod > 0 {
		for i := 0; i < colorsPerPalette-mod; i++ {
			p = append(p, color.RGBA{0, 0, 0, 0})
		}
	}
	return p
}

func reducePalette(m *image.Paletted) (color.Palette, [numTiles]byte, bool) {
	b := m.Bounds()

	// Create a copy of the image
	dup := image.NewPaletted(b, m.Palette)
	draw.Draw(dup, b, m, b.Min, draw.Src)

	// Map of colors to frequency of occurrence
	global := countColors(dup, b)

	// Loop over every tile and reduce the number of colors per tile to no
	// more than 16
	for ty := 0; ty < tileY; ty++ {
		for tx := 0; tx < tileX; tx++ {
			r := image.Rect(tx*tileWidth, ty*tileHeight, tx*tileWidth+tileWidth-1, ty*tileHeight+tileHeight-1)
			p := uniqueColors(dup, r)
			for len(p) > colorsPerPalette {

				// Find the two closest colors
				c1, c2 := closestColors(p)

				// Keep whichever color appears more frequently
				// in the image and replace any occurrence of
				// the other color
				var c color.Color
				if global[c1] > global[c2] {
					replaceColor(dup, c2, c1)
					c = c2
				} else {
					replaceColor(dup, c1, c2)
					c = c1
				}

				// Forget the less frequent color
				i := p.Index(c)
				p = append(p[:i], p[i+1:]...)
				delete(global, c)
			}
		}
	}

	// Loop over every tile and count the number of colors per tile
	var palettes []paletteMap
	for ty := 0; ty < tileY; ty++ {
		for tx := 0; tx < tileX; tx++ {
			r := image.Rect(tx*tileWidth, ty*tileHeight, tx*tileWidth+tileWidth-1, ty*tileHeight+tileHeight-1)
			palettes = append(palettes, paletteMap{
				palette: uniqueColors(dup, r),
				tiles:   []byte{byte(ty*tileX + tx)},
			})
		}
	}

	// Sort with biggest palettes first
	sort.Sort(sort.Reverse(byPaletteSize(palettes)))

	// Try and pack each tile palette into no more than three palettes
	if packed, ok := packPalette(palettes, []paletteMap{}); ok {
		var tiles [numTiles]byte
		var palette color.Palette
		for i, p := range packed {
			for _, t := range p.tiles {
				tiles[t] = byte(i)
			}
			palette = append(palette, padPalette(p.palette)...)
		}
		return palette, tiles, true
	}

	return nil, [numTiles]byte{}, false
}

func (e *encoder) encode(m *image.Paletted, tiles [numTiles]byte) error {
	// Write out pixel information
	for ty := 0; ty < tileY; ty++ {
		for tx := 0; tx < tileX; tx++ {
			for y := 0; y < tileHeight; y++ {
				for x := 0; x < tileWidth>>1; x++ {
					dx := tx*tileWidth + x<<1
					dy := ty*tileHeight + y

					// This is masking off any bits leaving a 0-15 value
					if _, err := e.w.Write([]byte{(m.ColorIndexAt(dx, dy) & 0x0f << 4) | m.ColorIndexAt(dx+1, dy)&0x0f}); err != nil {
						return err
					}
				}
			}
		}
	}

	// Write out palette indices
	if _, err := e.w.Write(tiles[:]); err != nil {
		return err
	}

	// Write out palette(s) assuming it's already a multiple of 16 colors
	var tmp [2]byte
	for _, c := range m.Palette {
		r, g, b, _ := c.RGBA()

		tmp[0] = byte(b >> 12 & 0x0e)
		tmp[1] = byte(g>>8&0xe0 | r>>12&0x0e)

		if _, err := e.w.Write(tmp[:]); err != nil {
			return err
		}
	}

	return nil
}

// Encode writes the Image m to w in MegaSD tile format.
func Encode(w io.Writer, m image.Image) error {
	b := m.Bounds()
	if b.Dx() != pixelX || b.Dy() != pixelY {
		return errors.New("tile: image is wrong size")
	}

	pm, _ := m.(*image.Paletted)
	if pm == nil {
		if cp, ok := m.ColorModel().(color.Palette); ok {
			pm = image.NewPaletted(b, cp)
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					pm.Set(x, y, cp.Convert(m.At(x, y)))
				}
			}
		}
	}

	// Will default to all zeroes which is correct for <= colorsPerPalette
	var tiles [numTiles]byte

	if pm == nil || len(pm.Palette) > colorsPerPalette {
		q := quantize.MedianCutQuantizer{}

		// Work out the starting maximum colors
		max := colorsPerPalette * maxPalettes
		if pm != nil && len(pm.Palette) < max {
			max = len(pm.Palette)
		}

		// Keep reducing the colors until the palette can be packed
		for i := max; i >= colorsPerPalette; i-- {
			// Create the initial palette
			tmp := image.NewPaletted(b, q.Quantize(make(color.Palette, 0, i), m))
			draw.Draw(tmp, b, m, b.Min, draw.Src)

			// Try and pack it
			if p, t, ok := reducePalette(tmp); ok {
				pm = image.NewPaletted(b, p)
				draw.Draw(pm, b, tmp, b.Min, draw.Src)
				copy(tiles[:], t[:])
				break
			}
		}
	} else {
		pm.Palette = padPalette(pm.Palette)
	}

	// Adjust image so that top-left corner is at (0, 0)
	if pm.Rect.Min != (image.Point{}) {
		dup := *pm
		dup.Rect = dup.Rect.Sub(dup.Rect.Min)
		pm = &dup
	}

	e := encoder{w: w}

	return e.encode(pm, tiles)
}
