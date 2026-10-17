package ui

import (
	"math"
	"testing"
	"time"

	"github.com/AvengeMedia/dankgo/lyrics"
	"github.com/KotonBads/mosaic/player"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

func TestAppCSSParsesCleanly(t *testing.T) {
	provider := gtk.NewCSSProvider()
	var parseErrs []string
	provider.ConnectParsingError(func(sec *gtk.CSSSection, err error) {
		parseErrs = append(parseErrs, err.Error())
	})
	provider.LoadFromBytes(glib.NewBytes([]byte(css)))
	if len(parseErrs) > 0 {
		t.Fatalf("GTK4 CSS parser reported errors in app.css: %v", parseErrs)
	}
}

func TestGetLyricsClientSingleton(t *testing.T) {
	c1 := GetLyricsClient()
	c2 := GetLyricsClient()
	if c1 == nil || c1 != c2 {
		t.Fatalf("expected non-nil singleton *lyrics.Client, got %v and %v", c1, c2)
	}
}

func TestBuildLyricsRequest(t *testing.T) {
	track := player.Track{
		Title: "Starlight",
		Artists: []player.Artist{
			{Name: "Muse"},
			{Name: "Guest"},
		},
		Album:    player.Album{Title: "Black Holes and Revelations"},
		Duration: 240 * time.Second,
		Path:     "/music/starlight.flac",
	}

	req := BuildLyricsRequest(track)
	if req.Title != "Starlight" {
		t.Fatalf("expected Title %q, got %q", "Starlight", req.Title)
	}
	if req.Artist != "Muse, Guest" {
		t.Fatalf("expected Artist %q, got %q", "Muse, Guest", req.Artist)
	}
	if req.Album != "Black Holes and Revelations" {
		t.Fatalf("expected Album %q, got %q", "Black Holes and Revelations", req.Album)
	}
	if req.Duration != 240*time.Second {
		t.Fatalf("expected Duration %v, got %v", 240*time.Second, req.Duration)
	}
	if req.FileURL != "/music/starlight.flac" {
		t.Fatalf("expected FileURL %q, got %q", "/music/starlight.flac", req.FileURL)
	}
}

func TestPreprocessTimeline_InterludesAndWordOffsets(t *testing.T) {
	lines := []lyrics.Line{
		{
			Start: 6.5,
			End:   9.0,
			Text:  "Hello world",
			Words: []lyrics.Word{
				{Start: 6.5, End: 7.5, Text: "Hello "},
				{Start: 7.5, End: 9.0, Text: "world"},
			},
		},
		{
			Start:      16.0,
			End:        0, // Should infer End from last word End (18.5)
			Text:       "Café résumé",
			Background: true,
			Words: []lyrics.Word{
				{Start: 16.0, End: 17.2, Text: "Café "},
				{Start: 17.2, End: 18.5, Text: "résumé"},
			},
		},
		{
			Start: 20.0,
			End:   0,
			Text:  "Final line",
		},
	}

	entries := preprocessTimeline(lines, 26.0)
	// Expected:
	// 0: Interlude [0.0, 6.5] (since lines[0].Start >= 5.0)
	// 1: Line "Hello world" [6.5, 9.0]
	// 2: Interlude [9.0, 16.0] (since 16.0 - 9.0 = 7.0 >= 6.0)
	// 3: Line "Café résumé" [16.0, 18.5] (inferred from last word End)
	// 4: Line "Final line" [20.0, 26.0] (inferred from track duration)
	if len(entries) != 5 {
		t.Fatalf("expected 5 timeline entries, got %d", len(entries))
	}

	if !entries[0].isInterlude || entries[0].start != 0.0 || entries[0].end != 6.5 {
		t.Fatalf("unexpected leading interlude: %+v", entries[0])
	}

	line1 := entries[1]
	if line1.isInterlude || len(line1.words) != 2 {
		t.Fatalf("expected 2 words in first lyric line, got %+v", line1)
	}
	if line1.words[0].startByte != 0 || line1.words[0].endByte != 6 {
		t.Fatalf("unexpected byte span for 'Hello ': [%d, %d)", line1.words[0].startByte, line1.words[0].endByte)
	}
	if line1.words[1].startByte != 6 || line1.words[1].endByte != 11 {
		t.Fatalf("unexpected byte span for 'world': [%d, %d)", line1.words[1].startByte, line1.words[1].endByte)
	}

	if !entries[2].isInterlude || entries[2].start != 9.0 || entries[2].end != 16.0 {
		t.Fatalf("unexpected mid-song interlude: %+v", entries[2])
	}

	// Verify multi-byte UTF-8 rune spans in "Café résumé"
	line2 := entries[3]
	if !line2.background || line2.end != 18.5 {
		t.Fatalf("expected background line with inferred end 18.5, got %+v", line2)
	}
	if len(line2.words) != 2 {
		t.Fatalf("expected 2 words in UTF-8 line, got %d", len(line2.words))
	}
	// "Café " is 'C'(1) + 'a'(1) + 'f'(1) + 'é'(2) + ' '(1) = 6 bytes, 5 runes
	w0 := line2.words[0]
	if w0.startByte != 0 || w0.endByte != 6 || len(w0.runes) != 5 {
		t.Fatalf("unexpected UTF-8 word[0] span: %+v", w0)
	}
	if line2.text[w0.runes[3].startByte:w0.runes[3].endByte] != "é" {
		t.Fatalf("expected 4th rune of word[0] to be 'é', got %q", line2.text[w0.runes[3].startByte:w0.runes[3].endByte])
	}

	// "résumé" starts at byte 6, length 8 bytes ('r'(1)+'é'(2)+'s'(1)+'u'(1)+'m'(1)+'é'(2)), 6 runes
	w1 := line2.words[1]
	if w1.startByte != 6 || w1.endByte != 14 || len(w1.runes) != 6 {
		t.Fatalf("unexpected UTF-8 word[1] span: %+v", w1)
	}
	if line2.text[w1.runes[5].startByte:w1.runes[5].endByte] != "é" {
		t.Fatalf("expected last rune of word[1] to be 'é', got %q", line2.text[w1.runes[5].startByte:w1.runes[5].endByte])
	}

	line3 := entries[4]
	if line3.start != 20.0 || line3.end != 26.0 {
		t.Fatalf("expected final line [20.0, 26.0], got [%f, %f]", line3.start, line3.end)
	}

	// Verify active timeline index resolution
	if idx := findActiveTimelineIndex(entries, 3.0); idx != 0 {
		t.Fatalf("expected index 0 at 3.0s, got %d", idx)
	}
	if idx := findActiveTimelineIndex(entries, 7.2); idx != 1 {
		t.Fatalf("expected index 1 at 7.2s, got %d", idx)
	}
	if idx := findActiveTimelineIndex(entries, 12.0); idx != 2 {
		t.Fatalf("expected index 2 at 12.0s, got %d", idx)
	}
	if idx := findActiveTimelineIndex(entries, 19.2); idx != 3 {
		t.Fatalf("expected index 3 in short gap at 19.2s, got %d", idx)
	}
	if idx := findActiveTimelineIndex(entries, 22.0); idx != 4 {
		t.Fatalf("expected index 4 at 22.0s, got %d", idx)
	}
}

func TestComputeWordProgress(t *testing.T) {
	if got := computeWordProgress(1.0, 2.0, 4.0); got != 0.0 {
		t.Fatalf("expected 0.0 before start, got %f", got)
	}
	if got := computeWordProgress(2.0, 2.0, 4.0); got != 0.0 {
		t.Fatalf("expected 0.0 at start, got %f", got)
	}
	if got := computeWordProgress(3.0, 2.0, 4.0); math.Abs(got-0.5) > 1e-6 {
		t.Fatalf("expected 0.5 at midpoint, got %f", got)
	}
	if got := computeWordProgress(4.0, 2.0, 4.0); got != 1.0 {
		t.Fatalf("expected 1.0 at end, got %f", got)
	}
	if got := computeWordProgress(5.0, 2.0, 4.0); got != 1.0 {
		t.Fatalf("expected 1.0 after end, got %f", got)
	}
	if got := computeWordProgress(2.5, 3.0, 3.0); got != 0.0 {
		t.Fatalf("expected 0.0 before degenerate span, got %f", got)
	}
	if got := computeWordProgress(3.5, 3.0, 3.0); got != 1.0 {
		t.Fatalf("expected 1.0 after degenerate span, got %f", got)
	}
}

func TestComputeWordRuneAlphas(t *testing.T) {
	if got := computeWordRuneAlphas(0.5, 0, 22000, 65535); got != nil {
		t.Fatalf("expected nil for 0 runes, got %v", got)
	}

	unsung := computeWordRuneAlphas(0.0, 5, 22000, 65535)
	for i, a := range unsung {
		if a != 22000 {
			t.Fatalf("expected rune %d alpha 22000 at progress 0, got %d", i, a)
		}
	}

	sung := computeWordRuneAlphas(1.0, 5, 22000, 65535)
	for i, a := range sung {
		if a != 65535 {
			t.Fatalf("expected rune %d alpha 65535 at progress 1, got %d", i, a)
		}
	}

	mid := computeWordRuneAlphas(0.5, 6, 22000, 65535)
	if len(mid) != 6 {
		t.Fatalf("expected 6 rune alphas, got %d", len(mid))
	}
	if mid[0] <= 22000 {
		t.Fatalf("expected leading rune to be lit at progress 0.5, got %d", mid[0])
	}
	if mid[5] >= 65535 {
		t.Fatalf("expected trailing rune to not yet be fully sung at progress 0.5, got %d", mid[5])
	}
	for i := 1; i < len(mid); i++ {
		if mid[i] > mid[i-1] {
			t.Fatalf("expected non-increasing alpha wave left-to-right, got %v", mid)
		}
	}
}

func TestComputeWordRise(t *testing.T) {
	if got := computeWordRise(0.0, 2.2); got != 0 {
		t.Fatalf("expected 0 rise at progress 0, got %d", got)
	}
	if got := computeWordRise(-0.2, 2.2); got != 0 {
		t.Fatalf("expected 0 rise for negative progress, got %d", got)
	}

	peak := computeWordRise(0.5, 2.2)
	expectedPeak := int(math.Round(2.2 * float64(pango.SCALE)))
	if peak != expectedPeak {
		t.Fatalf("expected peak rise %d at progress 0.5, got %d", expectedPeak, peak)
	}

	settled := computeWordRise(1.0, 2.2)
	expectedSettled := int(math.Round(0.5 * float64(pango.SCALE)))
	if settled != expectedSettled {
		t.Fatalf("expected settled rise %d at progress 1.0, got %d", expectedSettled, settled)
	}
}

func TestResolveLyricsResult_EmbeddedPriority(t *testing.T) {
	syncedEmbedded := parseEmbeddedLyrics("[00:01.00] <00:01.00>Hello <00:01.50>world\n[00:03.00] Second line")
	if syncedEmbedded == nil || len(syncedEmbedded.Synced) != 2 {
		t.Fatalf("expected 2 synced lines from embedded LRC, got %+v", syncedEmbedded)
	}
	plainEmbedded := parseEmbeddedLyrics("Just plain embedded lyrics\nLine two")
	if plainEmbedded == nil || len(plainEmbedded.Synced) != 0 || plainEmbedded.Plain == "" {
		t.Fatalf("expected plain embedded lyrics, got %+v", plainEmbedded)
	}

	onlinePlain := &lyrics.Result{
		Found:  true,
		Source: lyrics.LRCLIB,
		Lyrics: lyrics.Lyrics{Plain: "Online plain text"},
	}
	onlineSynced := &lyrics.Result{
		Found:  true,
		Source: lyrics.BetterLyrics,
		Lyrics: lyrics.Lyrics{
			Synced: []lyrics.Line{{Start: 1.0, End: 2.0, Text: "Online synced"}},
		},
	}

	// 1. Synced online beats synced embedded (and doesn't even invoke fetchEmbedded)
	called := false
	res, err := resolveLyricsResult(onlineSynced, nil, func() *lyrics.Lyrics {
		called = true
		return syncedEmbedded
	})
	if err != nil || res != onlineSynced || called {
		t.Fatalf("expected onlineSynced without calling fetchEmbedded, got res=%+v called=%v", res, called)
	}

	// 2. Synced embedded beats plain-text online
	res, err = resolveLyricsResult(onlinePlain, nil, func() *lyrics.Lyrics {
		return syncedEmbedded
	})
	if err != nil || res == nil || len(res.Lyrics.Synced) != 2 {
		t.Fatalf("expected synced embedded to beat plain online, got %+v", res)
	}

	// 3. Plain-text online beats plain-text embedded
	res, err = resolveLyricsResult(onlinePlain, nil, func() *lyrics.Lyrics {
		return plainEmbedded
	})
	if err != nil || res != onlinePlain {
		t.Fatalf("expected plain online to beat plain embedded, got %+v", res)
	}

	// 4. Plain-text embedded acts as backup when online is not found
	res, err = resolveLyricsResult(&lyrics.Result{Found: false}, nil, func() *lyrics.Lyrics {
		return plainEmbedded
	})
	if err != nil || res == nil || !res.Found || res.Lyrics.Plain != plainEmbedded.Plain {
		t.Fatalf("expected plain embedded backup when online not found, got %+v", res)
	}
}

