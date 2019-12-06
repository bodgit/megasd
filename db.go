package megasd

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	"github.com/bodgit/megasd/tile"
	_ "github.com/mattn/go-sqlite3"
)

/*
type gameDB struct {
	CRC   uint32
	Index uint16
}

type byCRC []gameDB

func (g byCRC) Len() int {
	return len(g)
}

func (g byCRC) Swap(i, j int) {
	g[i], g[j] = g[j], g[i]
}

func (g byCRC) Less(i, j int) bool {
	return g[i].CRC < g[j].CRC
}

// sort.Sort(byCRC(...))
*/

type GameDB struct {
	db *sql.DB
}

func NewDB(file string) (*GameDB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?_foreign_keys=on", file))
	if err != nil {
		return nil, err
	}

	return &GameDB{
		db: db,
	}, nil
}

func (db *GameDB) Close() error {
	return db.db.Close()
}

func (db *GameDB) AddScreenshot(file string) (int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	h := sha1.New()
	m, _, err := image.Decode(io.TeeReader(f, h))
	if err != nil {
		return 0, err
	}
	sha := fmt.Sprintf("%X", h.Sum(nil))

	var id int64
	switch err := db.db.QueryRow("SELECT id FROM screenshot WHERE sha1 = :sha1", sql.Named("sha1", sha)).Scan(&id); err {
	case sql.ErrNoRows:
		b := new(bytes.Buffer)
		if err := tile.Encode(b, m); err != nil {
			return 0, err
		}
		result, err := db.db.Exec("INSERT INTO screenshot (sha1, tile) VALUES (:sha1, :tile)", sql.Named("sha1", sha), sql.Named("tile", b.Bytes()))
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

func (db *GameDB) AddGame(year int, genre genre, screenshot int64) (int64, error) {
	var id int64
	switch err := db.db.QueryRow("SELECT id FROM game WHERE year = :year, genre = :genre, screenshot_id = :screenshot_id", sql.Named("year", year), sql.Named("genre", genre), sql.Named("screenshot_id", screenshot)).Scan(&id); err {
	case sql.ErrNoRows:
		result, err := db.db.Exec("INSERT INTO game (year, genre, screenshot_id) VALUES(:year, :genre, :screenshot_id)", sql.Named("year", year), sql.Named("genre", genre), sql.Named("screenshot_id", screenshot))
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

func (db *GameDB) AddChecksum(game int64, crc string) error {
	return nil
}
