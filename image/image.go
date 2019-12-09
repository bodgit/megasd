/*
Package image implements a MegaSD image decoder and encoder.

The format is defined as 64 by 40 pixels exactly which is split into forty 8
by 8 tiles. Up to three 16 color palettes can be defined and each tile can
use only one of these palettes.

The file is written as 1280 bytes of pixel information; a 4-bit index for each
pixel, followed by 40 bytes of palette index, one per tile and finally up to
three 32 byte palettes of 16 colors where each color is stored as a packed
16-bit value. There is no compression so the resulting file is either 1352,
1384, or 1416 bytes in size depending on the number of palettes used.
*/
package image

const (
	tileWidth        = 8
	tileHeight       = tileWidth
	tilePixels       = tileWidth * tileHeight
	tileX            = 8
	tileY            = 5
	numTiles         = tileX * tileY
	colorsPerPalette = 16
	maxPalettes      = 3
	pixelX           = tileWidth * tileX
	pixelY           = tileHeight * tileY
	numPixels        = pixelX * pixelY
	pixelBytes       = numPixels >> 1
)
