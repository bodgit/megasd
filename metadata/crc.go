package metadata

import (
	"fmt"
	"strings"

	"github.com/bodgit/megasd/crc32"
)

const filenameTrim = 56

// CRCFilename computes the CRC of a given filename using the same algorithm as
// implemented in the MegaSD firmware
func CRCFilename(filename string) uint32 {
	var b [filenameTrim]byte
	copy(b[:], []byte(fmt.Sprintf("%.*s", filenameTrim, strings.ToUpper(filename))))
	return crc32.Update(0xffffffff, b[:])
}
