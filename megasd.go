/*
Package megasd is a library for maintaining content on the Terraonion MegaSD
cartridge.
*/
package megasd

import "log"

// MegaSD manages an internal game database
type MegaSD struct {
	db     *gameDB
	logger *log.Logger
}

// New creates a new MegaSD instance given the intended path to the database
// and an instance of log.Logger.
func New(file string, logger *log.Logger) (*MegaSD, error) {
	db, err := newGameDB(file)
	if err != nil {
		return nil, err
	}

	return &MegaSD{
		db:     db,
		logger: logger,
	}, nil
}
