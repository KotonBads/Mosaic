package player

import (
	"math/rand"
	"slices"
	"strings"
)

func (p *Player) SortAlphabetically() {
	if len(p.Queue) == 0 {
		return
	}

	current := p.Queue[p.CurrentIdx]

	sorted_tracks := slices.SortedStableFunc(slices.Values(p.Queue), func(a, b Track) int {
		a_norm := strings.ToLower(a.Title)
		b_norm := strings.ToLower(b.Title)
		return strings.Compare(a_norm, b_norm)
	})
	p.Queue = sorted_tracks

	idx := slices.IndexFunc(p.Queue, func(t Track) bool {
		return t.ID == current.ID && t.Path == current.Path
	})
	if idx != -1 {
		p.CurrentIdx = idx
	}
}

func (p *Player) ShuffleQueue() {
	if len(p.Queue) == 0 {
		return
	}

	var current Track
	hasCurrent := p.CurrentIdx >= 0 && p.CurrentIdx < len(p.Queue)
	if hasCurrent {
		current = p.Queue[p.CurrentIdx]
	}

	// only shuffle onwards from current song
	ahead := p.Queue[p.CurrentIdx+1:]
	rand.Shuffle(len(ahead), func(i, j int) {
		ahead[i], ahead[j] = ahead[j], ahead[i]
	})
	p.Queue = append(p.Queue[:1+p.CurrentIdx], ahead...)

	if hasCurrent {
		idx := slices.IndexFunc(p.Queue, func(t Track) bool {
			return t.ID == current.ID && t.Path == current.Path
		})
		if idx != -1 {
			p.CurrentIdx = idx
		}
	}
}
