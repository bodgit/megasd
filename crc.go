package megasd

import (
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"

	crc "github.com/bodgit/megasd/crc32"
	"github.com/vchimishuk/chub/cue"
)

const (
	sectorHeader  = 16
	sectorSize    = 2048
	sectorTrailer = 288
)

const (
	filenameTrim = 56
)

func firstDataTrack(sheet *cue.Sheet) (string, cue.TrackDataType, error) {
	for _, file := range sheet.Files {
		for _, track := range file.Tracks {
			switch track.DataType {
			case cue.DataTypeMode1_2048, cue.DataTypeMode1_2352:
				return file.Name, track.DataType, nil
			}
		}
	}
	return "", cue.DataTypeAudio, errors.New("audio-only CDs are not supported for hashing")
}

func crcCueFile(file string) (string, error) {
	sheet, err := cue.ParseFile(file)
	if err != nil {
		return "", err
	}

	fileName, dataType, err := firstDataTrack(sheet)
	if err != nil {
		return "", nil
	}

	f, err := os.Open(filepath.Join(filepath.Dir(file), fileName))
	if err != nil {
		return "", nil
	}
	defer f.Close()

	if dataType == cue.DataTypeMode1_2352 {
		if _, err := f.Seek(sectorHeader, os.SEEK_CUR); err != nil {
			return "", nil
		}
	}

	h := crc32.NewIEEE()
	var b bytes.Buffer

	if _, err := io.CopyN(io.MultiWriter(h, &b), f, sectorSize); err != nil {
		return "", nil
	}

	if bytes.Compare(b.Bytes()[0x100:0x104], []byte{'S', 'E', 'G', 'A'}) != 0 {
		return "", errors.New("invalid signature")
	}

	if dataType == cue.DataTypeMode1_2352 {
		if _, err := f.Seek(sectorTrailer, os.SEEK_CUR); err != nil {
			return "", nil
		}
	}

	return fmt.Sprintf("%.*X", crc32.Size<<1, h.Sum(nil)), nil
}

func crcFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	if _, err = f.Seek(info.Size()&0xfff, os.SEEK_CUR); err != nil {
		return "", nil
	}

	h := crc32.NewIEEE()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%.*X", crc32.Size<<1, h.Sum(nil)), nil
}

func crcFilename(filename string) string {
	var b [filenameTrim]byte
	copy(b[:], []byte(fmt.Sprintf("%.*s", filenameTrim, strings.ToUpper(filename))))
	return fmt.Sprintf("%.*X", crc32.Size<<1, crc.Update(0xffffffff, b[:]))
}
