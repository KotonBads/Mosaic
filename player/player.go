package player

import (
	"slices"
	"time"

	"github.com/charmbracelet/log"
	"github.com/diamondburned/gotk4/pkg/core/glib"
)

var logger = log.WithPrefix("player")

func (p *Player) Notify() {
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
	case PosChange:
		p.OnPosChange = append(p.OnPosChange, f.(func(time.Duration)))
	}
}

func (p *Player) ResetViewListeners() {
	if p.MPRIS != nil && len(p.ControlChange) > 0 {
		p.ControlChange = p.ControlChange[:1]
	} else {
		p.ControlChange = nil
	}
	p.OnPosChange = nil
}

func (p *Player) UpdatePos(pos time.Duration) {
	p.Pos = pos
	p.LastPosUpdate = time.Now()
	if len(p.OnPosChange) > 0 {
		callbacks := slices.Clone(p.OnPosChange)
		glib.IdleAdd(func() {
			for _, f := range callbacks {
				f(pos)
			}
		})
	}
}

func (p *Player) EstimatedPos() time.Duration {
	if p == nil {
		return 0
	}
	est := p.Pos
	if !p.Paused && !p.LastPosUpdate.IsZero() {
		est += time.Since(p.LastPosUpdate)
	}
	if est < 0 {
		est = 0
	}
	if p.CurrentIdx >= 0 && p.CurrentIdx < len(p.Queue) {
		if dur := p.Queue[p.CurrentIdx].Duration; dur > 0 && est > dur {
			est = dur
		}
	}
	return est
}

func (p *Player) Play() {
	logger.Debug("Play")
	p.Paused = false
	p.LastPosUpdate = time.Now()
	if p.MPV != nil {
		_ = p.MPV.Play()
	}
	p.Notify()
}

func (p *Player) Pause() {
	logger.Debug("Pause")
	p.Pos = p.EstimatedPos()
	p.Paused = true
	p.LastPosUpdate = time.Now()
	if p.MPV != nil {
		_ = p.MPV.Pause()
	}
	p.Notify()
}

func (p *Player) PlayPause() {
	logger.Debug("Play/Pause", "state", p.Paused)
	if !p.Paused {
		p.Pos = p.EstimatedPos()
	}
	p.Paused = !p.Paused
	p.LastPosUpdate = time.Now()
	if p.MPV != nil {
		_ = p.MPV.TogglePause()
	}
	p.Notify()
}

func (p *Player) SetRepeat() {
	logger.Debug("Set repeat", "state", p.Repeat)
	p.Repeat = (p.Repeat + 1) % 3
	p.Notify()
}

func (p *Player) SetShuffle() {
	logger.Debug("Set shuffle", "state", p.Shuffle)
	p.Shuffle = !p.Shuffle
	if p.Shuffle {
		p.ShuffleQueue()
	} else {
		p.SortAlphabetically()
	}
	p.Notify()
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
	p.LastPosUpdate = time.Now()
	if p.MPV != nil {
		_ = p.MPV.PlayFile(p.Queue[idx].Path)
	}
	p.Notify()
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
	p.PlayTrack(p.CurrentIdx)
}

func (p *Player) Prev() {
	logger.Debug("Previous track", "current", p.CurrentIdx, "queue", len(p.Queue))
	if len(p.Queue) == 0 {
		return
	}
	p.CurrentIdx = (p.CurrentIdx - 1 + len(p.Queue)) % len(p.Queue)
	p.PlayTrack(p.CurrentIdx)
}

func (p *Player) Seek(t time.Duration) {
	logger.Debug("Seek", "pos", t)
	p.Pos = t
	p.LastPosUpdate = time.Now()
	if p.MPV != nil {
		_ = p.MPV.Seek(t.Seconds())
	}
	p.Notify()
	if len(p.OnPosChange) > 0 {
		callbacks := slices.Clone(p.OnPosChange)
		glib.IdleAdd(func() {
			for _, f := range callbacks {
				f(t)
			}
		})
	}
}

func (p *Player) SetVolume(v int) {
	if v < 0 {
		v = 0
	} else if v > 100 {
		v = 100
	}
	logger.Debug("Set volume", "volume", v)
	if v > 0 {
		p.Muted = false
		p.PreMuteVolume = v
	} else {
		p.Muted = true
	}
	p.Volume = v
	if p.MPV != nil {
		_ = p.MPV.SetVolume(float64(v))
	}
}

func (p *Player) ToggleMute() {
	if !p.Muted && p.Volume > 0 {
		p.PreMuteVolume = p.Volume
		p.Muted = true
		p.Volume = 0
	} else {
		p.Muted = false
		restore := p.PreMuteVolume
		if restore <= 0 {
			restore = 20
		}
		p.Volume = restore
	}
	logger.Debug("Toggle mute", "muted", p.Muted, "volume", p.Volume)
	if p.MPV != nil {
		_ = p.MPV.SetVolume(float64(p.Volume))
	}
	p.Notify()
}
