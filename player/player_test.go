package player

import (
	"testing"
	"time"
)

func TestEstimatedPos(t *testing.T) {
	var nilPlayer *Player
	if got := nilPlayer.EstimatedPos(); got != 0 {
		t.Fatalf("nil player EstimatedPos() = %v, want 0", got)
	}

	p := &Player{
		Pos:           10 * time.Second,
		LastPosUpdate: time.Now().Add(-500 * time.Millisecond),
		Paused:        true,
	}
	if got := p.EstimatedPos(); got != 10*time.Second {
		t.Fatalf("paused EstimatedPos() = %v, want 10s", got)
	}

	p.Paused = false
	got := p.EstimatedPos()
	if got < 10*time.Second+400*time.Millisecond || got > 11*time.Second {
		t.Fatalf("playing EstimatedPos() = %v, want ~10.5s", got)
	}

	// Test clamping to track duration
	p.Queue = []Track{{Title: "Song", Duration: 10 * time.Second}}
	p.CurrentIdx = 0
	p.Pos = 9900 * time.Millisecond
	p.LastPosUpdate = time.Now().Add(-500 * time.Millisecond)
	if got := p.EstimatedPos(); got != 10*time.Second {
		t.Fatalf("clamped EstimatedPos() = %v, want 10s", got)
	}

	// Test clamping negative pos
	p.Pos = -5 * time.Second
	p.Paused = true
	if got := p.EstimatedPos(); got != 0 {
		t.Fatalf("negative EstimatedPos() = %v, want 0", got)
	}
}

func TestResetViewListeners(t *testing.T) {
	p := &Player{}
	p.Subscribe(ControlChange, func() {})
	p.Subscribe(PosChange, func(time.Duration) {})
	p.Subscribe(QueueChange, func() {})
	p.Subscribe(OnTrackChange, func(Track) {})

	if len(p.ControlChange) != 1 || len(p.OnPosChange) != 1 {
		t.Fatalf("expected ControlChange and OnPosChange to have 1 listener")
	}

	p.ResetViewListeners()

	if p.ControlChange != nil || p.OnPosChange != nil {
		t.Fatalf("expected ControlChange and OnPosChange to be nil after ResetViewListeners")
	}
	if len(p.QueueChange) != 1 || len(p.OnTrackChange) != 1 {
		t.Fatalf("expected QueueChange and OnTrackChange to remain intact")
	}
}
