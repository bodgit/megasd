package image

import (
	"errors"
	"image"
	"image/color"
	"io"
)

var (
	errNotEnough  = errors.New("tile: not enough image data")
	errTooMuch    = errors.New("tile: too much image data")
	errBadPalette = errors.New("tile: invalid palette index")
)

func readFull(r io.Reader, b []byte) error {
	_, err := io.ReadFull(r, b)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

func upperNibble(b byte) byte {
	return b & 0xf0
}

func lowerNibble(b byte) byte {
	return b & 0x0f
}

type decoder struct {
	r io.Reader

	numPalettes int

	image   *image.Paletted
	palette color.Palette

	// Enough to hold the pixels and palette indices
	tmp [pixelBytes + numTiles]byte
}

func (d *decoder) readPixelsAndPaletteIndices() error {
	if err := readFull(d.r, d.tmp[:]); err != nil {
		return err
	}

	for _, b := range d.tmp[pixelBytes:] {
		if b >= maxPalettes {
			return errBadPalette
		}
		if int(b) > d.numPalettes {
			d.numPalettes = int(b)
		}
	}
	d.numPalettes++
	return nil
}

func (d *decoder) readPalette() error {
	d.palette = make(color.Palette, colorsPerPalette*d.numPalettes)
	for i := range d.palette {
		var tmp [2]byte
		if err := readFull(d.r, tmp[:]); err != nil {
			return err
		}
		// Color is packed as 0000BBB0GGG0RRR0
		d.palette[i] = color.RGBA{
			lowerNibble(tmp[1]) << 4,
			upperNibble(tmp[1]),
			lowerNibble(tmp[0]) << 4,
			0xff,
		}
	}
	return nil
}

func (d *decoder) decode(r io.Reader, configOnly bool) error {
	d.r = r

	if err := d.readPixelsAndPaletteIndices(); err != nil {
		if err != io.ErrUnexpectedEOF {
			return err
		}
		return errNotEnough
	}

	if err := d.readPalette(); err != nil {
		if err != io.ErrUnexpectedEOF {
			return err
		}
		return errNotEnough
	}

	if n, err := r.Read(d.tmp[:1]); n != 0 || (err != io.EOF && err != io.ErrUnexpectedEOF) {
		if err != nil {
			return err
		}
		return errTooMuch
	}

	if configOnly {
		return nil
	}

	d.image = image.NewPaletted(image.Rect(0, 0, pixelX, pixelY), d.palette)

	for ty := 0; ty < tileY; ty++ {
		for tx := 0; tx < tileX; tx++ {
			tile := ty*tileX + tx
			for y := 0; y < tileHeight; y++ {
				for x := 0; x < tileWidth>>1; x++ {
					i := tile*tilePixels>>1 + y*tileWidth>>1 + x

					dx := tx*tileWidth + x<<1
					dy := ty*tileHeight + y

					p := d.tmp[pixelBytes+tile] * colorsPerPalette

					d.image.SetColorIndex(dx+0, dy, p+upperNibble(d.tmp[i])>>4)
					d.image.SetColorIndex(dx+1, dy, p+lowerNibble(d.tmp[i]))
				}
			}
		}
	}

	return nil
}

// Decode reads a MegaSD tile from r and returns it as an image.Image.
func Decode(r io.Reader) (image.Image, error) {
	var d decoder
	if err := d.decode(r, false); err != nil {
		return nil, err
	}
	return d.image, nil
}

// DecodeConfig returns the color model and dimensions of a MegaSD tile without
// decoding the entire tile.
func DecodeConfig(r io.Reader) (image.Config, error) {
	var d decoder
	if err := d.decode(r, true); err != nil {
		return image.Config{}, err
	}
	return image.Config{
		ColorModel: d.palette,
		Width:      pixelX,
		Height:     pixelY,
	}, nil
}
