/*
Package metadata implements the small metadata database written to each
directory on the MegaSD filesystem that contains ROM and/or CD images.
*/
package metadata

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
)

const (
	// Filename is the expected filename used when writing to disk
	Filename   = "games.dbs"
	maxEntries = 1024

	// ScreenshotSize defines the expected size in bytes of each screenshot
	ScreenshotSize = 2048
)

// DB is the metadata database object. It implements the
// encoding.BinaryMarshaler and encoding.BinaryUnmarshaler interfaces.
type DB struct {
	checksums   map[uint32]uint16
	screenshots [][]byte
}

// New returns an empty metadata database
func New() *DB {
	return &DB{
		checksums: make(map[uint32]uint16),
	}
}

// Length returns the number of checksums in the database
func (db *DB) Length() int {
	return len(db.checksums)
}

// Set stores the provided screenshot for the given CRC
func (db *DB) Set(crc uint32, screenshot []byte) error {
	if len(screenshot) != ScreenshotSize {
		return errors.New("incorrect length")
	}
	if _, ok := db.checksums[crc]; !ok {
		db.screenshots = append(db.screenshots, screenshot)
		db.checksums[crc] = uint16(len(db.screenshots) - 1)
	}
	return nil
}

// MarshalBinary encodes the database into binary form and returns the result
func (db *DB) MarshalBinary() ([]byte, error) {
	length := len(db.checksums)

	if length > maxEntries {
		return nil, fmt.Errorf("more than %d entries", maxEntries)
	}

	keys := make([]uint32, 0, len(db.checksums))
	for k := range db.checksums {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	b := new(bytes.Buffer)

	// Write out CRC values
	if err := binary.Write(b, binary.LittleEndian, &keys); err != nil {
		return nil, err
	}
	// Pad to 4096 with 0xff's
	if _, err := b.Write(bytes.Repeat([]byte{0xff, 0xff, 0xff, 0xff}, maxEntries-length)); err != nil {
		return nil, err
	}

	// Write out screenshot indices
	for _, k := range keys {
		v := db.checksums[k]
		if err := binary.Write(b, binary.LittleEndian, &v); err != nil {
			return nil, err
		}
	}
	// Pad to 6144 with 0xff's
	if _, err := b.Write(bytes.Repeat([]byte{0xff, 0xff}, maxEntries-length)); err != nil {
		return nil, err
	}

	// Write out screenshots
	for _, s := range db.screenshots {
		if _, err := b.Write(s); err != nil {
			return nil, err
		}
	}

	return b.Bytes(), nil
}

// UnmarshalBinary decodes the database from binary form
func (db *DB) UnmarshalBinary(b []byte) error {
	r := bytes.NewReader(b)

	db.checksums = make(map[uint32]uint16)
	db.screenshots = nil

	var keys []uint32
	for i := 0; i < maxEntries; i++ {
		var crc uint32
		if err := binary.Read(r, binary.LittleEndian, &crc); err != nil {
			return err
		}
		if crc != 0xffffffff {
			keys = append(keys, crc)
		}
	}

	var maxOffset int
	for i := 0; i < maxEntries; i++ {
		var offset uint16
		if err := binary.Read(r, binary.LittleEndian, &offset); err != nil {
			return err
		}
		if offset != 0xffff && i < len(keys) {
			db.checksums[keys[i]] = offset
			if int(offset) > maxOffset {
				maxOffset = int(offset)
			}
		}
	}

	for i := 0; i <= maxOffset; i++ {
		var screenshot [ScreenshotSize]byte
		if n, err := r.Read(screenshot[:]); n != ScreenshotSize || (err != nil && err != io.EOF) {
			return errors.New("insufficient data")
		}
		db.screenshots = append(db.screenshots, screenshot[:])
	}

	return nil
}
