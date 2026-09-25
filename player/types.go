package player

import (
	"bufio"
	"database/sql"
	"net"
	"sync/atomic"
	"time"

	"github.com/quarckster/go-mpris-server/pkg/events"
	"github.com/quarckster/go-mpris-server/pkg/server"

	_ "modernc.org/sqlite"
)

type Library struct {
	db      *sql.DB
	Artists []Artist
	Albums  []Album
	Tracks  []Track

	Player *Player
}

type Artist struct {
	Name   string
	Albums []Album
}

type Album struct {
	Title  string
	Tracks []Track
	Year   time.Time
}

type Track struct {
	ID       int
	Path     string
	Title    string
	Artists  []Artist
	Album    Album
	Duration time.Duration
}

type Client struct {
	conn       net.Conn
	reader     *bufio.Reader
	reqID      atomic.Uint64
	socketPath string

	OnTimePos  func(pos float64)
	OnPause    func(paused bool)
	OnTrackEnd func(reason string)
}

type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatQueue
	RepeatOne
)

type Player struct {
	Pos           time.Duration
	Queue         []Track
	CurrentIdx    int
	Paused        bool
	Shuffle       bool
	Repeat        RepeatMode
	Volume        int
	MPV           *Client
	MPRIS         *MPRIS
	ControlChange []func()
	QueueChange   []func()
	OnTrackChange []func(Track)
}

type PlayerEvents int

const (
	ControlChange PlayerEvents = iota
	QueueChange
	OnTrackChange
)

type MPRIS struct {
	player *Player
	server *server.Server
	events *events.EventHandler
}
