/*
Package megasd is a library for maintaining content on the Terraonion MegaSD
cartridge.
*/
package megasd

import "log"

const screenshotSize = 2048

type MegaSD struct {
	db     *GameDB
	logger *log.Logger
}

func New(db *GameDB, logger *log.Logger) *MegaSD {
	return &MegaSD{
		db:     db,
		logger: logger,
	}
}
