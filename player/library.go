package player

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"go.senan.xyz/taglib"
)

// Loads the database file at `path`
func (lib *Library) Open(path string) error {
	address := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", address)
	if err != nil {
		return err
	}
	lib.db = db

	query := `
	CREATE TABLE IF NOT EXISTS tracks (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		title     TEXT NOT NULL,
		artists   TEXT NOT NULL,
		album     TEXT NOT NULL,
		duration  INTEGER NOT NULL,
		date      TEXT NOT NULL DEFAULT '',
		file_path TEXT NOT NULL UNIQUE
	);
	`
	_, err = lib.db.Exec(query)
	if err != nil {
		return err
	}

	// Safe migration in case the database was previously initialized without date
	_, _ = lib.db.Exec(`ALTER TABLE tracks ADD COLUMN date TEXT NOT NULL DEFAULT ''`)

	return nil
}

// Walks through `path`, extracts metadata, and store into DB
func (lib *Library) Index(path string) error {
	err := filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Warn("Could not access path", "path", filePath, "err", err)
			return nil
		}
		if d.IsDir() {
			return nil
		}

		tags, err := taglib.ReadTags(filePath)
		if err != nil {
			return nil
		}

		var duration int
		properties, err := taglib.ReadProperties(filePath)
		if err == nil {
			duration = int(properties.Length.Seconds())
		}

		title := firstTag(tags[taglib.Title], fallbackTitle(path))
		album := firstTag(tags[taglib.Album], "Unknown Album")

		// Extract date
		dateStr := firstTag(tags[taglib.Date], "")
		if dateStr == "" {
			dateStr = firstTag(tags[taglib.OriginalDate], "")
		}
		parsedDate := parseDate(dateStr)
		var storedDate string
		if !parsedDate.IsZero() {
			storedDate = parsedDate.Format(time.RFC3339)
		}

		// Extract artists
		artistList := tags[taglib.Artist]
		if len(artistList) == 0 {
			artistList = tags[taglib.Artists]
		}

		if len(artistList) == 0 {
			artistList = []string{"Unknown Artist"}
		}

		artistsJSON, err := json.Marshal(artistList)
		if err != nil {
			artistsJSON = []byte(`[]`)
		}

		query := `
		INSERT INTO tracks (title, artists, album, duration, date, file_path)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			title = excluded.title,
			artists = excluded.artists,
			album = excluded.album,
			duration = excluded.duration,
			date = excluded.date;
		`
		_, err = lib.db.Exec(query, title, string(artistsJSON), album, duration, storedDate, filePath)
		if err != nil {
			logger.Warn("Could not insert track", "path", filePath, "err", err)
		}
		return nil
	})
	return err
}

// Load queries the database and populates the in-memory Tracks, Albums, and Artists slices
func (lib *Library) Load() error {
	rows, err := lib.db.Query(`
		SELECT id, title, artists, album, duration, date, file_path
		FROM tracks
		ORDER BY id ASC;
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tracks []Track
	albumMap := make(map[string]*Album)
	artistMap := make(map[string]*Artist)

	for rows.Next() {
		var (
			id          int
			title       string
			artistsJSON string
			albumTitle  string
			durationSec int
			dateStr     string
			filePath    string
		)
		if err := rows.Scan(&id, &title, &artistsJSON, &albumTitle, &durationSec, &dateStr, &filePath); err != nil {
			return err
		}

		var artistNames []string
		_ = json.Unmarshal([]byte(artistsJSON), &artistNames)

		var trackArtists []Artist
		for _, name := range artistNames {
			trackArtists = append(trackArtists, Artist{Name: name})
		}

		track := Track{
			ID:       id,
			Path:     filePath,
			Title:    title,
			Artists:  trackArtists,
			Album:    albumTitle,
			Duration: time.Duration(durationSec) * time.Second,
		}
		tracks = append(tracks, track)

		// Group into Album
		album, exists := albumMap[albumTitle]
		if !exists {
			var year time.Time
			if dateStr != "" {
				year, _ = time.Parse(time.RFC3339, dateStr)
			}
			album = &Album{
				Title: albumTitle,
				Year:  year,
			}
			albumMap[albumTitle] = album
		}
		album.Tracks = append(album.Tracks, track)

		// Register Artists
		for _, name := range artistNames {
			if _, exists := artistMap[name]; !exists {
				artistMap[name] = &Artist{Name: name}
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Link albums to each artist
	for _, album := range albumMap {
		seenArtists := make(map[string]bool)
		for _, track := range album.Tracks {
			for _, artist := range track.Artists {
				if !seenArtists[artist.Name] {
					seenArtists[artist.Name] = true
					if a, ok := artistMap[artist.Name]; ok {
						a.Albums = append(a.Albums, *album)
					}
				}
			}
		}
	}

	// Build final slices
	albums := make([]Album, 0, len(albumMap))
	for _, album := range albumMap {
		albums = append(albums, *album)
	}

	artists := make([]Artist, 0, len(artistMap))
	for _, artist := range artistMap {
		artists = append(artists, *artist)
	}

	lib.Tracks = tracks
	lib.Albums = albums
	lib.Artists = artists

	return nil
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02",
		"2006",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func fallbackTitle(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func firstTag(values []string, fallback string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return sanitize_name(v)
		}
	}
	return sanitize_name(fallback)
}
