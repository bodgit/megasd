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
	// TODO Support more than 16 colors
	if pm == nil || len(pm.Palette) > colorsPerPalette { // colorsPerPalette*maxPalettes {
		q := quantize.MedianCutQuantizer{}
		pm = image.NewPaletted(b, q.Quantize(make(color.Palette, 0, colorsPerPalette), m))
		draw.Draw(pm, b, m, b.Min, draw.Src)
	}

	// Adjust image so that top-left corner is at (0, 0)
	if pm.Rect.Min != (image.Point{}) {
		dup := *pm
		dup.Rect = dup.Rect.Sub(dup.Rect.Min)
		pm = &dup
	}

	e := encoder{w: w}

	return e.encode(pm)
}
