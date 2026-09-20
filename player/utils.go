package player

import (
	"math/rand"
	"slices"
	"strings"
)

func (p *Player) SortAlphabetically() {
	sorted_tracks := slices.SortedStableFunc(slices.Values(p.Queue), func(a, b Track) int {
		a_norm := strings.ToLower(a.Title)
		b_norm := strings.ToLower(b.Title)
		return strings.Compare(a_norm, b_norm)
	})
	p.Queue = sorted_tracks
}

func (p *Player) ShuffleQueue() {
	shuffled_tracks := slices.SortedStableFunc(slices.Values(p.Queue), func(a, b Track) int {
		return rand.Int()
	})
	p.Queue = shuffled_tracks
}

func sanitize_name(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "/", "_"))
}
