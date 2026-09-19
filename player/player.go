package player

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
)

func (p *Player) notify() {
	if p.OnChange != nil {
		glib.IdleAdd(p.OnChange)
	}
}

func (p *Player) PlayPause() {
	logger.Debug("Play/Pause", "state", p.Paused)
	p.Paused = !p.Paused
	if p.MPV != nil {
		_ = p.MPV.TogglePause()
	}
	p.notify()
}

func (p *Player) SetRepeat() {
	logger.Debug("Set repeat", "state", p.Repeat)
	p.Repeat = (p.Repeat + 1) % 3
	p.notify()
}

func (p *Player) SetShuffle() {
	logger.Debug("Set shuffle", "state", p.Shuffle)
	p.Shuffle = !p.Shuffle
	p.notify()
}

func (p *Player) Next() {
	logger.Debug("Next track", "current", p.CurrentIdx, "queue", len(p.Queue))
	if len(p.Queue) == 0 {
		return
	}
	p.CurrentIdx = max((p.CurrentIdx+1)%len(p.Queue), 0)
	if p.CurrentIdx >= len(p.Queue) {
		p.CurrentIdx = len(p.Queue) - 1
	}
	if p.MPV != nil {
		_ = p.MPV.PlayFile(p.Queue[p.CurrentIdx].Path)
	}
	p.notify()
}

func (p *Player) Prev() {
	logger.Debug("Previous track", "current", p.CurrentIdx, "queue", len(p.Queue))
	if len(p.Queue) == 0 {
		return
	}
	p.CurrentIdx = max((p.CurrentIdx - 1 + len(p.Queue)) % len(p.Queue), 0)
	if p.CurrentIdx >= len(p.Queue) {
		p.CurrentIdx = len(p.Queue) - 1
	}
	if p.MPV != nil {
		_ = p.MPV.PlayFile(p.Queue[p.CurrentIdx].Path)
	}
	p.notify()
}

func (p *Player) Seek(t time.Duration) {
	logger.Debug("Seek", "time", t)
	p.Pos = t
	if p.MPV != nil {
		_ = p.MPV.Seek(t.Seconds())
	}
	p.notify()
}

func (p *Player) SetVolume(v int) {
	logger.Debug("Set volume", "volume", v)
	p.Volume = v
	if p.MPV != nil {
		_ = p.MPV.SetVolume(float64(v))
	}
	p.notify()
}
