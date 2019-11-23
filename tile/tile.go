/*
Package tile implements a MegaSD tile decoder and encoder.

The format is defined as 64 by 40 pixels exactly which is split into forty 8
by 8 tiles. Up to three 16 color palettes can be defined and each tile can
use only one of these palettes.
*/
package tile

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
