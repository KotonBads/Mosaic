package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/quarckster/go-mpris-server/pkg/events"
	"github.com/quarckster/go-mpris-server/pkg/server"
	"github.com/quarckster/go-mpris-server/pkg/types"
)

func sanitize_name(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "/", "_"))
}

func (m *MPRIS) Init(p *Player) {
	m.player = p
	m.server = server.NewServer("mosaic", m, m)

	go func() {
		if err := m.server.Listen(); err != nil {
			logger.Warn("MPRIS server stopped", "err", err)
		}
	}()

	// Wait briefly for the D-Bus connection to be established
	for range 10 {
		if m.server.Connection() != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 3. Event handler used to notify D-Bus of updates
	m.events = events.NewEventHandler(m.server)

	// 4. Hook into Player events
	p.Subscribe(OnTrackChange, func(_ Track) {
		if m.events != nil {
			_ = m.events.Player.OnTitle()
			_ = m.events.Player.OnPlayPause()
		}
	})

	p.Subscribe(ControlChange, func() {
		if m.events != nil {
			_ = m.events.Player.OnPlayPause()
		}
	})
}

func (m *MPRIS) Identity() (string, error)              { return "Mosaic", nil }
func (m *MPRIS) CanQuit() (bool, error)                 { return false, nil }
func (m *MPRIS) Quit() error                            { return nil }
func (m *MPRIS) CanRaise() (bool, error)                { return false, nil }
func (m *MPRIS) Raise() error                           { return nil }
func (m *MPRIS) HasTrackList() (bool, error)            { return false, nil }
func (m *MPRIS) SupportedUriSchemes() ([]string, error) { return []string{"file"}, nil }
func (m *MPRIS) SupportedMimeTypes() ([]string, error) {
	return []string{"audio/mpeg", "audio/flac"}, nil
}

// ========================================================
// 2. Player Adapter (org.mpris.MediaPlayer2.Player)
// ========================================================

// Controls
func (m *MPRIS) Play() error      { m.player.Play(); return nil }
func (m *MPRIS) Pause() error     { m.player.Pause(); return nil }
func (m *MPRIS) PlayPause() error { m.player.PlayPause(); return nil }
func (m *MPRIS) Stop() error      { m.player.Pause(); return nil }
func (m *MPRIS) Next() error      { m.player.Next(); return nil }
func (m *MPRIS) Previous() error  { m.player.Prev(); return nil }

// Status & Metadata getters
func (m *MPRIS) PlaybackStatus() (types.PlaybackStatus, error) {
	if m.player.Paused {
		return types.PlaybackStatusPaused, nil
	}
	return types.PlaybackStatusPlaying, nil
}

func (m *MPRIS) Metadata() (types.Metadata, error) {
	if len(m.player.Queue) == 0 || m.player.CurrentIdx < 0 || m.player.CurrentIdx >= len(m.player.Queue) {
		return types.Metadata{}, nil
	}

	cache_dir, _ := os.UserCacheDir()
	track := m.player.Queue[m.player.CurrentIdx]
	artists := make([]string, len(track.Artists))
	for i, a := range track.Artists {
		artists[i] = a.Name
	}
	art := filepath.Join(
		cache_dir, "mosaic", "art",
		fmt.Sprintf("%s - %s.png", strings.Join(artists, ", "), sanitize_name(track.Album.Title)),
	)

	return types.Metadata{
		TrackId:        dbus.ObjectPath(fmt.Sprintf("/org/mosaic/track/%d", track.ID)),
		Title:          track.Title,
		Artist:         artists,
		Album:          track.Album.Title,
		ArtUrl:         "file://" + art,
		Length:         types.Microseconds(track.Duration.Microseconds()),
		ContentCreated: track.Album.Year.Format(time.RFC3339),
	}, nil
}

// Permissions
func (m *MPRIS) CanControl() (bool, error)    { return true, nil }
func (m *MPRIS) CanPlay() (bool, error)       { return true, nil }
func (m *MPRIS) CanPause() (bool, error)      { return true, nil }
func (m *MPRIS) CanGoNext() (bool, error)     { return true, nil }
func (m *MPRIS) CanGoPrevious() (bool, error) { return true, nil }
func (m *MPRIS) CanSeek() (bool, error)       { return false, nil }

// Unused optional stubs
func (m *MPRIS) Position() (int64, error)                     { return m.player.Pos.Microseconds(), nil }
func (m *MPRIS) Seek(types.Microseconds) error                { return nil }
func (m *MPRIS) SetPosition(string, types.Microseconds) error { return nil }
func (m *MPRIS) OpenUri(string) error                         { return nil }
func (m *MPRIS) Volume() (float64, error)                     { return float64(m.player.Volume) / 100, nil }
func (m *MPRIS) SetVolume(v float64) error {
	m.player.Volume = int(v * 100)
	return nil
}
func (m *MPRIS) Rate() (float64, error)        { return 1.0, nil }
func (m *MPRIS) SetRate(float64) error         { return nil }
func (m *MPRIS) MinimumRate() (float64, error) { return 1.0, nil }
func (m *MPRIS) MaximumRate() (float64, error) { return 1.0, nil }
