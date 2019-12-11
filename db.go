package megasd

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/xml"
	"fmt"
	img "image"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/megasd/image"
	"github.com/bodgit/megasd/metadata"
	_ "github.com/mattn/go-sqlite3"
)

type GameDB struct {
	db *sql.DB
}

func NewGameDB(file string) (*GameDB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?_foreign_keys=on", file))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)

	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS screenshot (id INTEGER PRIMARY KEY NOT NULL, sha1 TEXT NOT NULL UNIQUE, data BLOB NOT NULL)"); err != nil {
		return nil, err
	}

	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS game (id INTEGER PRIMARY KEY NOT NULL, name STRING NOT NULL UNIQUE, screenshot_id INTEGER, genre INTEGER, year INTEGER, FOREIGN KEY(screenshot_id) REFERENCES screenshot(id))"); err != nil {
		return nil, err
	}

	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS checksum(game_id INTEGER NOT NULL, crc TEXT NOT NULL UNIQUE, FOREIGN KEY(game_id) REFERENCES game(id))"); err != nil {
		return nil, err
	}

	return &GameDB{
		db: db,
	}, nil
}

type xmlGameDB struct {
	XMLName   xml.Name      `xml:"GameDB"`
	Games     []xmlGame     `xml:"Game"`
	Checksums []xmlChecksum `xml:"GameCk"`
	Genres    []xmlGenre    `xml:"Genre"`
}

type xmlGame struct {
	XMLName    xml.Name `xml:"Game"`
	ID         int      `xml:"ID"`
	Name       string   `xml:"Name"`
	Year       int64    `xml:"Year"`
	Genre      int64    `xml:"Genre"`
	Screenshot string   `xml:"Screenshot"`
}

type xmlChecksum struct {
	XMLName  xml.Name `xml:"GameCk"`
	Checksum string   `xml:"Checksum"`
	GameID   int      `xml:"GameID"`
}

type xmlGenre struct {
	XMLName xml.Name `xml:"Genre"`
	Genre   int      `xml:"Genre"`
	Name    string   `xml:"Name"`
}

func (db *GameDB) ImportXML(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	var xmlDB xmlGameDB
	if err := xml.Unmarshal(b, &xmlDB); err != nil {
		return err
	}

	if _, err = db.db.Exec("DELETE FROM checksum"); err != nil {
		return err
	}

	if _, err = db.db.Exec("DELETE FROM game"); err != nil {
		return err
	}

	if _, err = db.db.Exec("DELETE FROM screenshot"); err != nil {
		return err
	}

	for _, g := range xmlDB.Games {
		var year sql.NullInt64
		if g.Year != 0 {
			year.Int64 = g.Year
			year.Valid = true
		}

		var genre sql.NullInt64
		if g.Genre != 0 {
			genre.Int64 = g.Genre
			genre.Valid = true
		}

		var screenshot sql.NullInt64
		if g.Screenshot != "" {
			screenshot.Int64, err = db.addScreenshot(filepath.Join(filepath.Dir(file), filepath.Clean(strings.ReplaceAll(g.Screenshot, "\\", string(os.PathSeparator)))))
			if err != nil {
				return err
			}
			screenshot.Valid = true
		}

		game, err := db.addGame(g.Name, year, genre, screenshot)
		if err != nil {
			return err
		}

		for _, c := range xmlDB.Checksums {
			if g.ID == c.GameID {
				db.addChecksum(game, fmt.Sprintf("%08s", strings.ToUpper(c.Checksum)))
			}
		}
	}

	return nil
}

func (db *GameDB) Close() error {
	return db.db.Close()
}

func (db *GameDB) addScreenshot(file string) (int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	h := sha1.New()
	m, _, err := img.Decode(io.TeeReader(f, h))
	if err != nil {
		return 0, err
	}
	sha := fmt.Sprintf("%X", h.Sum(nil))

	var id int64
	switch err := db.db.QueryRow("SELECT id FROM screenshot WHERE sha1 = ?", sha).Scan(&id); err {
	case sql.ErrNoRows:
		b := new(bytes.Buffer)
		if err := image.Encode(b, m); err != nil {
			return 0, err
		}
		result, err := db.db.Exec("INSERT INTO screenshot (sha1, data) VALUES (?, ?)", sha, b.Bytes())
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	case nil:
		return id, nil
	default:
		return 0, err
	}
}

func (db *GameDB) addGame(name string, year, genre, screenshot sql.NullInt64) (int64, error) {
	var id int64
	switch err := db.db.QueryRow("SELECT id FROM game WHERE name = ? AND year = ? AND genre = ? AND screenshot_id = ?", name, year, genre, screenshot).Scan(&id); err {
	case sql.ErrNoRows:
		result, err := db.db.Exec("INSERT INTO game (name, year, genre, screenshot_id) VALUES (?, ?, ?, ?)", name, year, genre, screenshot)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	case nil:
		return id, nil
	default:
		return 0, err
	}
}

func (db *GameDB) addChecksum(game int64, crc string) error {
	if _, err := db.db.Exec("INSERT OR REPLACE INTO checksum (game_id, crc) VALUES (?, ?)", game, crc); err != nil {
		return err
	}
	return nil
}

func (db *GameDB) FindScreenshotByCRC(crc string) ([]byte, error) {
	var year, genre sql.NullInt64
	var data []byte
	switch err := db.db.QueryRow("SELECT g.year, g.genre, s.data FROM checksum AS c JOIN game AS g ON c.game_id = g.id LEFT JOIN screenshot AS s ON g.screenshot_id = s.id WHERE c.crc = ?", crc).Scan(&year, &genre, &data); err {
	case sql.ErrNoRows:
		return nil, nil
	case nil:
		if data == nil {
			return nil, nil
		}

		var screenshot [metadata.ScreenshotSize]byte
		copy(screenshot[:], data)

		// XXX Should only enable this if there is a genre and/or year?
		screenshot[0x700] = 1
		if genre.Valid {
			screenshot[0x701] = byte(genre.Int64)
		}
		if year.Valid {
			screenshot[0x702] = byte(year.Int64 & 0xff)
			screenshot[0x703] = byte(year.Int64 >> 8 & 0xff)
		}

		return screenshot[:], nil
	default:
		return nil, err
	}
}
