package player

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLibraryLoad(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	var lib Library
	if err := lib.Open(dbPath); err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Insert test data directly into the DB to test Load logic
	query := `
	INSERT INTO tracks (title, artists, album, duration, date, file_path)
	VALUES
		('Song 1', '["Daft Punk", "Pharrell Williams"]', 'Random Access Memories', 275, '2013-05-17T00:00:00Z', '/path/1.flac'),
		('Song 2', '["Daft Punk"]', 'Random Access Memories', 214, '2013-05-17T00:00:00Z', '/path/2.flac'),
		('Song 3', '["Queen", "David Bowie"]', 'Hot Space', 248, '1982-05-21T00:00:00Z', '/path/3.flac');
	`
	if _, err := lib.db.Exec(query); err != nil {
		t.Fatalf("Exec insert failed: %v", err)
	}

	if err := lib.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(lib.Tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(lib.Tracks))
	}
	if len(lib.Albums) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(lib.Albums))
	}

	// Check track duration converted to time.Duration
	if lib.Tracks[0].Duration != 275*time.Second {
		t.Errorf("expected 275s, got %v", lib.Tracks[0].Duration)
	}

	// Check artist list on track 0
	if len(lib.Tracks[0].Artists) != 2 {
		t.Fatalf("expected 2 artists on track 0, got %d", len(lib.Tracks[0].Artists))
	}
	if lib.Tracks[0].Artists[0].Name != "Daft Punk" || lib.Tracks[0].Artists[1].Name != "Pharrell Williams" {
		t.Errorf("unexpected artists: %v", lib.Tracks[0].Artists)
	}

	// Check album year
	for _, album := range lib.Albums {
		if album.Title == "Random Access Memories" {
			if album.Year.Year() != 2013 {
				t.Errorf("expected year 2013, got %d", album.Year.Year())
			}
			if len(album.Tracks) != 2 {
				t.Errorf("expected 2 tracks in album, got %d", len(album.Tracks))
			}
		}
	}
}

func TestLibraryIndex(t *testing.T) {
	lib := Library{}
	err := lib.Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := lib.Index("/home/koton-bads/Music/Daily Mix 1/"); err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	err = lib.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	for _, track := range lib.Tracks {
		t.Logf(`
			Title: %s
			Album: %s
			Artists: %v
			`, track.Title, track.Album, track.Artists)
	}
}

func TestLibraryArtists(t *testing.T) {
	lib := Library{}
	err := lib.Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	err = lib.Index("/home/koton-bads/Music/Daily Mix 1/")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	err = lib.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	for _, artist := range lib.Artists {
		t.Logf(`
			Name: %s
			Albums: %v
			`, artist.Name, artist.Albums)
	}
}
