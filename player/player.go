package player

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
)

func (p *Player) notify() {
	if p.ControlChange != nil {
		glib.IdleAdd(p.ControlChange)
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
	logger.Info("Set shuffle", "state", p.Shuffle)
	p.Shuffle = !p.Shuffle
	if p.Shuffle {
		p.ShuffleQueue()
	} else {
		p.SortAlphabetically()
	}
	p.notify()
	p.QueueChange()
}

func (p *Player) PlayTrack(idx int) {
	if len(p.Queue) == 0 || idx < 0 || idx >= len(p.Queue) {
		return
	}
	logger.Debug("Play track", "index", idx, "title", p.Queue[idx].Title)
	track := p.Queue[idx]
	p.CurrentIdx = idx
	p.Pos = 0
	if p.MPV != nil {
		_ = p.MPV.PlayFile(p.Queue[idx].Path)
	}
	p.notify()
	if p.OnTrackChange != nil {
		glib.IdleAdd(func() { p.OnTrackChange(track) })
	}
}

func (p *Player) Next() {
	logger.Debug("Next track", "current", p.CurrentIdx, "queue", len(p.Queue))
	if len(p.Queue) == 0 {
		return
	}
	nextIdx := (p.CurrentIdx + 1) % len(p.Queue)
	track := p.Queue[nextIdx]
	p.PlayTrack(nextIdx)
	if p.OnTrackChange != nil {
		glib.IdleAdd(func() { p.OnTrackChange(track) })
	}
}

func (p *Player) Prev() {
	logger.Debug("Previous track", "current", p.CurrentIdx, "queue", len(p.Queue))
	if len(p.Queue) == 0 {
		return
	}
	prevIdx := (p.CurrentIdx - 1 + len(p.Queue)) % len(p.Queue)
	track := p.Queue[prevIdx]
	p.PlayTrack(prevIdx)
	if p.OnTrackChange != nil {
		glib.IdleAdd(func() { p.OnTrackChange(track) })
	}
}

func (p *Player) Seek(t time.Duration) {
	logger.Info("Seek", "pos", t)
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
