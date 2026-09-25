package player

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/diamondburned/gotk4/pkg/core/glib"
)

var logger = log.WithPrefix("player")

func (p *Player) notify() {
	if p.ControlChange != nil {
		glib.IdleAdd(func() {
			for _, f := range p.ControlChange {
				f()
			}
		})
	}
}

func (p *Player) Subscribe(event PlayerEvents, f any) {
	switch event {
	case ControlChange:
		p.ControlChange = append(p.ControlChange, f.(func()))
	case QueueChange:
		p.QueueChange = append(p.QueueChange, f.(func()))
	case OnTrackChange:
		p.OnTrackChange = append(p.OnTrackChange, f.(func(Track)))
	}
}

func (p *Player) Play() {
	logger.Debug("Play")
	p.Paused = false
	if p.MPV != nil {
		_ = p.MPV.Play()
	}
	p.notify()
}

func (p *Player) Pause() {
	logger.Debug("Pause")
	p.Paused = true
	if p.MPV != nil {
		_ = p.MPV.Pause()
	}
	p.notify()
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
	if p.Shuffle {
		p.ShuffleQueue()
	} else {
		p.SortAlphabetically()
	}
	p.notify()
	for _, f := range p.QueueChange {
		f()
	}
}

func (p *Player) PlayTrack(idx int) {
	if len(p.Queue) == 0 || idx < 0 || idx >= len(p.Queue) {
		return
	}
	logger.Info("Play track", "index", idx, "title", p.Queue[idx].Title)
	track := p.Queue[idx]
	p.CurrentIdx = idx
	p.Pos = 0
	p.SetVolume(20)
	if p.MPV != nil {
		_ = p.MPV.PlayFile(p.Queue[idx].Path)
	}
	p.notify()
	if len(p.OnTrackChange) > 0 {
		glib.IdleAdd(func() {
			for _, f := range p.OnTrackChange {
				f(track)
			}
		})
	}
}

func (p *Player) Next() {
	logger.Debug("Next track", "current", p.CurrentIdx, "queue", len(p.Queue))
	if len(p.Queue) == 0 {
		return
	}
	p.CurrentIdx = (p.CurrentIdx + 1) % len(p.Queue)
	track := p.Queue[p.CurrentIdx]
	p.PlayTrack(p.CurrentIdx)
	if len(p.OnTrackChange) > 0 {
		glib.IdleAdd(func() {
			for _, f := range p.OnTrackChange {
				f(track)
			}
		})
	}
}

func (p *Player) Prev() {
	logger.Debug("Previous track", "current", p.CurrentIdx, "queue", len(p.Queue))
	if len(p.Queue) == 0 {
		return
	}
	p.CurrentIdx = (p.CurrentIdx - 1 + len(p.Queue)) % len(p.Queue)
	track := p.Queue[p.CurrentIdx]
	p.PlayTrack(p.CurrentIdx)
	if len(p.OnTrackChange) > 0 {
		glib.IdleAdd(func() {
			for _, f := range p.OnTrackChange {
				f(track)
			}
		})
	}
}

func (p *Player) Seek(t time.Duration) {
	logger.Debug("Seek", "pos", t)
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
