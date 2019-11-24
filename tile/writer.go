package tile

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"

	"github.com/ericpauley/go-quantize/quantize"
)

type encoder struct {
	w io.Writer
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

func sqDiff(x, y uint32) uint32 {
	d := x - y
	return (d * d) >> 2
}

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

func replaceColor(m *image.Paletted, o, n color.Color) {
	for y := m.Bounds().Min.Y; y < m.Bounds().Max.Y; y++ {
		for x := m.Bounds().Min.X; x < m.Bounds().Max.X; x++ {
			if m.At(x, y) == o {
				m.Set(x, y, n)
			}
		}
	}
}

func reducePalette(m *image.Paletted) color.Palette {
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

	//var palettes [numTiles]color.Palette

	for ty := 0; ty < tileY; ty++ {
		for tx := 0; tx < tileX; tx++ {
			//tile := ty*tileX + tx
			//r := image.Rect(tx*tileWidth, ty*tileHeight, tx*tileWidth+tileWidth-1, ty*tileHeight+tileHeight-1)
		}
	}

	return uniqueColors(dup, b)
}

func (e *encoder) encode(m *image.Paletted) error {
	for ty := 0; ty < tileY; ty++ {
		for tx := 0; tx < tileX; tx++ {
			//tile := ty*tileX + tx
			for y := 0; y < tileHeight; y++ {
				for x := 0; x < tileWidth>>1; x++ {
					dx := tx*tileWidth + x<<1
					dy := ty*tileHeight + y

					if _, err := e.w.Write([]byte{(m.ColorIndexAt(dx, dy) & 0x0f << 4) | m.ColorIndexAt(dx+1, dy)&0x0f}); err != nil {
						return err
					}
				}
			}
		}
	}

	for i := 0; i < numTiles; i++ {
		if _, err := e.w.Write([]byte{0x00}); err != nil {
			return err
		}
	}

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
	if pm == nil || len(pm.Palette) > colorsPerPalette*maxPalettes {
		q := quantize.MedianCutQuantizer{}
		pm = image.NewPaletted(b, q.Quantize(make(color.Palette, 0, colorsPerPalette*maxPalettes), m))
		draw.Draw(pm, b, m, b.Min, draw.Src)
	}

	p := reducePalette(pm)
	npm := image.NewPaletted(b, p)
	draw.Draw(npm, b, pm, b.Min, draw.Src)
	pm = npm

	// Adjust image so that top-left corner is at (0, 0)
	if pm.Rect.Min != (image.Point{}) {
		dup := *pm
		dup.Rect = dup.Rect.Sub(dup.Rect.Min)
		pm = &dup
	}

	e := encoder{w: w}

	return e.encode(pm)
}
