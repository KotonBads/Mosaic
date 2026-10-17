package ui

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/AvengeMedia/dankgo/lyrics"
	"github.com/KotonBads/mosaic/player"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"go.senan.xyz/taglib"
)

const (
	lyricUnsungAlpha     uint16  = 24000
	lyricSungAlpha       uint16  = 57500
	lyricActiveWordAlpha uint16  = 65535
	lyricAfterglowSec    float64 = 0.25
	lyricMaxRisePt       float64 = 2.2
	lyricLeadSec         float64 = 0.04
)

var (
	lyricsClientOnce   sync.Once
	sharedLyricsClient *lyrics.Client
)

// GetLyricsClient returns a shared *lyrics.Client configured with the user's cache directory.
func GetLyricsClient() *lyrics.Client {
	lyricsClientOnce.Do(func() {
		cacheDir, err := os.UserCacheDir()
		if err != nil || cacheDir == "" {
			cacheDir = os.TempDir()
		}
		sharedLyricsClient = lyrics.New(lyrics.Options{
			CacheDir: filepath.Join(cacheDir, "mosaic", "lyrics"),
		})
	})
	return sharedLyricsClient
}

// BuildLyricsRequest constructs a lyrics.Request for the given player.Track.
func BuildLyricsRequest(track player.Track) lyrics.Request {
	return lyrics.Request{
		Title:    track.Title,
		Artist:   join_artist_name(track.Artists),
		Album:    track.Album.Title,
		Duration: track.Duration,
		FileURL:  track.Path,
	}
}

// parseEmbeddedLyrics parses raw embedded tag text as TTML (if XML) or LRC (plain, synced, or word-synced).
func parseEmbeddedLyrics(raw string) *lyrics.Lyrics {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "<") {
		if parsed, err := lyrics.ParseTTML([]byte(trimmed)); err == nil && parsed != nil && !parsed.Empty() {
			return parsed
		}
	}
	if parsed, err := lyrics.ParseLRC([]byte(raw)); err == nil && parsed != nil && !parsed.Empty() {
		return parsed
	}
	return nil
}

// readEmbeddedLyrics reads embedded lyrics metadata from the audio file via taglib.
// If multiple lyrics tags are present, synced lyrics take precedence over plain text.
func readEmbeddedLyrics(path string) *lyrics.Lyrics {
	if path == "" {
		return nil
	}
	tags, err := taglib.ReadTags(path)
	if err != nil || len(tags) == 0 {
		return nil
	}

	var bestPlain *lyrics.Lyrics
	keys := []string{taglib.Lyrics, "UNSYNCEDLYRICS", "SYLT", "USLT"}
	for _, k := range keys {
		for _, raw := range tags[k] {
			parsed := parseEmbeddedLyrics(raw)
			if parsed == nil {
				continue
			}
			if len(parsed.Synced) > 0 {
				return parsed
			}
			if bestPlain == nil {
				bestPlain = parsed
			}
		}
	}
	return bestPlain
}

// resolveLyricsResult applies the priority rules between an online/cached Lookup result
// and embedded audio tag lyrics:
//  1. Synced (or instrumental) online/cached result wins.
//  2. Synced embedded lyrics win over plain-text-only online/cached results.
//  3. Plain-text online/cached result wins over plain-text embedded lyrics.
//  4. Embedded lyrics (synced or plain) act as backup when online/cached lookup fails or is empty.
func resolveLyricsResult(online *lyrics.Result, onlineErr error, fetchEmbedded func() *lyrics.Lyrics) (*lyrics.Result, error) {
	onlineValid := online != nil && (online.Found || !online.Lyrics.Empty())

	// 1. Online/cached has synced lines or is explicitly marked instrumental -> top priority
	if onlineValid && (len(online.Lyrics.Synced) > 0 || online.Lyrics.Instrumental) {
		return online, nil
	}

	var embedded *lyrics.Lyrics
	if fetchEmbedded != nil {
		embedded = fetchEmbedded()
	}

	// 2. Synced embedded wins over plain-text online (or missing online)
	if embedded != nil && len(embedded.Synced) > 0 {
		return &lyrics.Result{
			Found:  true,
			Source: lyrics.Sidecar,
			Lyrics: *embedded,
		}, nil
	}

	// 3. Online/cached plain text wins if embedded isn't synced
	if onlineValid && strings.TrimSpace(online.Lyrics.Plain) != "" {
		return online, nil
	}

	// 4. Backup: embedded plain text (or instrumental) when online had nothing
	if embedded != nil && !embedded.Empty() {
		return &lyrics.Result{
			Found:  true,
			Source: lyrics.Sidecar,
			Lyrics: *embedded,
		}, nil
	}

	return online, onlineErr
}

// LookupTrackLyrics looks up lyrics from sidecars/cache/online providers and falls back to
// (or upgrades plain-text results with) lyrics embedded in the audio file's metadata tags.
func LookupTrackLyrics(ctx context.Context, track player.Track) (*lyrics.Result, error) {
	req := BuildLyricsRequest(track)
	online, err := GetLyricsClient().Lookup(ctx, req)
	return resolveLyricsResult(online, err, func() *lyrics.Lyrics {
		return readEmbeddedLyrics(track.Path)
	})
}

type runeSpan struct {
	startByte uint
	endByte   uint
}

type wordSpan struct {
	start     float64
	end       float64
	text      string
	startByte uint
	endByte   uint
	runes     []runeSpan
}

type timelineEntry struct {
	isInterlude bool
	start       float64
	end         float64
	text        string
	background  bool
	words       []wordSpan
}

// preprocessTimeline converts raw synced lyrics lines into a timeline with precomputed
// UTF-8 byte/rune offsets and automatic instrumental interludes.
func preprocessTimeline(lines []lyrics.Line, duration float64) []timelineEntry {
	if len(lines) == 0 {
		return nil
	}

	entries := make([]timelineEntry, 0, len(lines)+4)
	prevEnd := 0.0

	for i, line := range lines {
		start := float64(line.Start)
		end := float64(line.End)

		if end <= start {
			for _, w := range line.Words {
				if float64(w.End) > end {
					end = float64(w.End)
				}
			}
		}

		if end <= start {
			nextStart := -1.0
			for j := i + 1; j < len(lines); j++ {
				if !lines[j].Background && float64(lines[j].Start) > start {
					nextStart = float64(lines[j].Start)
					break
				}
			}
			if nextStart < 0 {
				for j := i + 1; j < len(lines); j++ {
					if float64(lines[j].Start) > start {
						nextStart = float64(lines[j].Start)
						break
					}
				}
			}

			if nextStart > start {
				end = math.Min(nextStart, start+8.0)
			} else if duration > start {
				end = duration
			} else {
				end = start + 5.0
			}
		}

		if i == 0 {
			if start >= 5.0 {
				entries = append(entries, timelineEntry{
					isInterlude: true,
					start:       0.0,
					end:         start,
					text:        "• • •",
				})
			}
		} else if start-prevEnd >= 6.0 {
			entries = append(entries, timelineEntry{
				isInterlude: true,
				start:       prevEnd,
				end:         start,
				text:        "• • •",
			})
		}

		var words []wordSpan
		if len(line.Words) > 0 && line.Text != "" {
			words = make([]wordSpan, 0, len(line.Words))
			cursor := 0
			textLen := len(line.Text)

			for wIdx, w := range line.Words {
				if w.Text == "" || cursor >= textLen {
					continue
				}

				rem := line.Text[cursor:]
				var wStartByte, wEndByte uint
				matched := false

				if strings.HasPrefix(rem, w.Text) {
					wStartByte = uint(cursor)
					wEndByte = uint(cursor + len(w.Text))
					cursor += len(w.Text)
					matched = true
				} else if idx := strings.Index(rem, w.Text); idx >= 0 {
					wStartByte = uint(cursor + idx)
					wEndByte = uint(cursor + idx + len(w.Text))
					cursor += idx + len(w.Text)
					matched = true
				} else if trimmed := strings.TrimSpace(w.Text); trimmed != "" {
					if idx := strings.Index(rem, trimmed); idx >= 0 {
						wStartByte = uint(cursor + idx)
						wEndByte = uint(cursor + idx + len(trimmed))
						cursor += idx + len(trimmed)
						matched = true
					}
				}

				if !matched || wEndByte <= wStartByte || int(wEndByte) > textLen {
					continue
				}

				wStart := float64(w.Start)
				wEnd := float64(w.End)
				if wEnd <= wStart {
					if wIdx+1 < len(line.Words) && float64(line.Words[wIdx+1].Start) > wStart {
						wEnd = float64(line.Words[wIdx+1].Start)
					} else if end > wStart {
						wEnd = end
					} else {
						wEnd = wStart + 0.3
					}
				}

				wordSlice := line.Text[wStartByte:wEndByte]
				runes := make([]runeSpan, 0, utf8.RuneCountInString(wordSlice))
				for relIdx, r := range wordSlice {
					rStart := wStartByte + uint(relIdx)
					rEnd := rStart + uint(utf8.RuneLen(r))
					runes = append(runes, runeSpan{
						startByte: rStart,
						endByte:   rEnd,
					})
				}

				words = append(words, wordSpan{
					start:     wStart,
					end:       wEnd,
					text:      wordSlice,
					startByte: wStartByte,
					endByte:   wEndByte,
					runes:     runes,
				})
			}
		}

		entries = append(entries, timelineEntry{
			isInterlude: false,
			start:       start,
			end:         end,
			text:        line.Text,
			background:  line.Background,
			words:       words,
		})

		if end > prevEnd {
			prevEnd = end
		}
	}

	return entries
}

// computeWordProgress returns the normalized [0, 1] progress for a word span at time now.
func computeWordProgress(now, start, end float64) float64 {
	if now <= start {
		return 0.0
	}
	if end <= start || now >= end {
		return 1.0
	}
	return (now - start) / (end - start)
}

// computeWordRuneAlphas computes a smooth feathered alpha sweep across numRunes glyphs.
func computeWordRuneAlphas(progress float64, numRunes int, unsungAlpha, sungAlpha uint16) []uint16 {
	if numRunes <= 0 {
		return nil
	}
	alphas := make([]uint16, numRunes)
	if progress <= 0.0 {
		for i := range alphas {
			alphas[i] = unsungAlpha
		}
		return alphas
	}
	if progress >= 1.0 {
		for i := range alphas {
			alphas[i] = sungAlpha
		}
		return alphas
	}

	const feather = 1.35
	frontier := progress * (float64(numRunes) + feather)
	base := float64(unsungAlpha)
	span := float64(sungAlpha) - base

	for i := 0; i < numRunes; i++ {
		local := (frontier - float64(i)) / feather
		if local <= 0.0 {
			alphas[i] = unsungAlpha
			continue
		}
		if local >= 1.0 {
			alphas[i] = sungAlpha
			continue
		}
		// Smoothstep easing across the glyph frontier
		eased := local * local * (3.0 - 2.0*local)
		alphas[i] = uint16(math.Round(base + eased*span))
	}
	return alphas
}

// computeWordRise computes the vertical Pango rise (in Pango units) for a word at the given progress.
func computeWordRise(progress float64, maxRisePt float64) int {
	if progress <= 0.0 {
		return 0
	}
	if progress >= 1.0 {
		return int(math.Round(0.5 * float64(pango.SCALE)))
	}
	sineLift := math.Sin(progress*math.Pi) * maxRisePt
	settledRamp := 0.5 * progress
	liftPt := math.Max(sineLift, settledRamp)
	return int(math.Round(liftPt * float64(pango.SCALE)))
}

func findActiveTimelineIndex(entries []timelineEntry, now float64) int {
	if len(entries) == 0 || now < entries[0].start {
		return -1
	}

	activeIdx := -1
	for i, e := range entries {
		if now >= e.start && now < e.end {
			activeIdx = i
		}
	}
	if activeIdx >= 0 {
		return activeIdx
	}

	for i := len(entries) - 1; i >= 0; i-- {
		if now >= entries[i].start {
			return i
		}
	}
	return -1
}

type lyricRowBinding struct {
	entry timelineEntry
	row   *gtk.Box
	label *gtk.Label
	dots  [3]*gtk.Label
}

func buildEmptyState(iconName, titleText, subtitleText string) *gtk.Box {
	emptyBox := gtk.NewBox(gtk.OrientationVertical, 10)
	emptyBox.AddCSSClass("lyrics-empty-box")
	emptyBox.SetHAlign(gtk.AlignCenter)
	emptyBox.SetVAlign(gtk.AlignCenter)
	emptyBox.SetHExpand(true)
	emptyBox.SetVExpand(true)

	icon := gtk.NewImageFromIconName(iconName)
	icon.SetPixelSize(42)
	icon.AddCSSClass("dim-label")

	title := gtk.NewLabel(titleText)
	title.AddCSSClass("lyrics-empty-title")
	title.SetHAlign(gtk.AlignCenter)

	emptyBox.Append(icon)
	emptyBox.Append(title)
	if subtitleText != "" {
		subtitle := gtk.NewLabel(subtitleText)
		subtitle.AddCSSClass("lyrics-empty-subtitle")
		subtitle.SetHAlign(gtk.AlignCenter)
		subtitle.SetWrap(true)
		subtitle.SetJustify(gtk.JustifyCenter)
		emptyBox.Append(subtitle)
	}
	return emptyBox
}

// Lyrics constructs the 60fps word-by-word synced lyrics widget for the current track.
func Lyrics(p *player.Player, onClose func()) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("lyrics-card")
	box.SetSizeRequest(440, 320)
	box.SetHExpand(false)
	box.SetVExpand(false)
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignCenter)
	box.SetOverflow(gtk.OverflowHidden)

	overlay := gtk.NewOverlay()
	overlay.SetHExpand(true)
	overlay.SetVExpand(true)

	// Scrollable Lyrics Stage (.lyrics-scroll)
	scrollable := gtk.NewScrolledWindow()
	scrollable.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scrollable.SetHExpand(true)
	scrollable.SetVExpand(true)
	scrollable.AddCSSClass("lyrics-scroll")

	var userScrollUntil time.Time
	scrollCtrl := gtk.NewEventControllerScroll(gtk.EventControllerScrollVertical)
	scrollCtrl.ConnectScroll(func(dx, dy float64) bool {
		userScrollUntil = time.Now().Add(2500 * time.Millisecond)
		scrollable.AddCSSClass("user-scrolling")
		return false
	})
	scrollable.AddController(scrollCtrl)

	closeBtn := gtk.NewButtonFromIconName("window-close-symbolic")
	closeBtn.AddCSSClass("lyrics-close-btn")
	closeBtn.AddCSSClass("flat")
	closeBtn.AddCSSClass("circular")
	closeBtn.SetTooltipText("Return to Album Art")
	closeBtn.SetCursorFromName("pointer")
	closeBtn.SetHAlign(gtk.AlignEnd)
	closeBtn.SetVAlign(gtk.AlignStart)
	closeBtn.SetMarginTop(6)
	closeBtn.SetMarginEnd(6)
	closeBtn.ConnectClicked(func() {
		if onClose != nil {
			onClose()
		}
	})

	loadingBox := buildEmptyState(
		"audio-x-generic-symbolic",
		"Loading Lyrics…",
		"",
	)
	scrollable.SetChild(loadingBox)

	overlay.SetChild(scrollable)
	overlay.AddOverlay(closeBtn)
	box.Append(overlay)

	if p == nil || len(p.Queue) == 0 || p.CurrentIdx < 0 || p.CurrentIdx >= len(p.Queue) {
		scrollable.SetChild(buildEmptyState(
			"audio-x-generic-symbolic",
			"No Lyrics Found",
			"",
		))
		return box
	}

	track := p.Queue[p.CurrentIdx]
	expectedTrackID := track.ID

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		result, err := LookupTrackLyrics(ctx, track)

		glib.IdleAdd(func() {
			if p.CurrentIdx < 0 || p.CurrentIdx >= len(p.Queue) || p.Queue[p.CurrentIdx].ID != expectedTrackID {
				return
			}

			if err != nil || result == nil || (!result.Found && result.Lyrics.Empty()) {
				scrollable.SetChild(buildEmptyState(
					"audio-x-generic-symbolic",
					"No Lyrics Found",
					"",
				))
				return
			}

			if result.Lyrics.Instrumental {
				scrollable.SetChild(buildEmptyState(
					"audio-x-generic-symbolic",
					"Instrumental",
					"",
				))
				return
			}

			if len(result.Lyrics.Synced) == 0 {
				if strings.TrimSpace(result.Lyrics.Plain) == "" {
					scrollable.SetChild(buildEmptyState(
						"audio-x-generic-symbolic",
						"No Lyrics Found",
						"",
					))
					return
				}

				plainLabel := gtk.NewLabel(result.Lyrics.Plain)
				plainLabel.AddCSSClass("lyrics-plain-text")
				plainLabel.SetXAlign(0.5)
				plainLabel.SetHAlign(gtk.AlignCenter)
				plainLabel.SetJustify(gtk.JustifyCenter)
				plainLabel.SetWrap(true)
				plainLabel.SetWrapMode(pango.WrapWordChar)
				plainLabel.SetMarginStart(20)
				plainLabel.SetMarginEnd(20)
				plainLabel.SetMarginTop(20)
				plainLabel.SetMarginBottom(20)
				scrollable.SetChild(plainLabel)
				return
			}

			entries := preprocessTimeline(result.Lyrics.Synced, track.Duration.Seconds())

			linesBox := gtk.NewBox(gtk.OrientationVertical, 10)
			linesBox.SetMarginStart(16)
			linesBox.SetMarginEnd(16)

			topSpacer := gtk.NewBox(gtk.OrientationVertical, 0)
			topSpacer.SetSizeRequest(-1, 145)
			linesBox.Append(topSpacer)

			rows := make([]lyricRowBinding, len(entries))
			for i, entry := range entries {
				entryCopy := entry
				if entryCopy.isInterlude {
					interludeBox := gtk.NewBox(gtk.OrientationHorizontal, 8)
					interludeBox.AddCSSClass("lyric-row")
					interludeBox.AddCSSClass("lyric-interlude")
					interludeBox.AddCSSClass("lyric-line-future")
					interludeBox.SetHAlign(gtk.AlignCenter)
					interludeBox.SetCursorFromName("pointer")

					var dots [3]*gtk.Label
					for d := 0; d < 3; d++ {
						dot := gtk.NewLabel("●")
						dot.AddCSSClass("interlude-dot")
						dot.SetOpacity(0.28)
						interludeBox.Append(dot)
						dots[d] = dot
					}

					click := gtk.NewGestureClick()
					click.ConnectReleased(func(nPress int, x, y float64) {
						userScrollUntil = time.Time{}
						p.Seek(time.Duration(entryCopy.start * float64(time.Second)))
					})
					interludeBox.AddController(click)

					linesBox.Append(interludeBox)
					rows[i] = lyricRowBinding{
						entry: entryCopy,
						row:   interludeBox,
						dots:  dots,
					}
					continue
				}

				rowBox := gtk.NewBox(gtk.OrientationVertical, 0)
				rowBox.AddCSSClass("lyric-row")
				rowBox.SetHAlign(gtk.AlignCenter)
				rowBox.SetCursorFromName("pointer")

				lbl := gtk.NewLabel(entryCopy.text)
				lbl.SetWrap(true)
				lbl.SetWrapMode(pango.WrapWordChar)
				lbl.SetXAlign(0.5)
				lbl.SetHAlign(gtk.AlignCenter)
				lbl.SetJustify(gtk.JustifyCenter)
				lbl.AddCSSClass("lyric-line")
				if entryCopy.background {
					lbl.AddCSSClass("lyric-line-bg")
				}
				lbl.AddCSSClass("lyric-line-future")

				rowBox.Append(lbl)

				click := gtk.NewGestureClick()
				click.ConnectReleased(func(nPress int, x, y float64) {
					userScrollUntil = time.Time{}
					p.Seek(time.Duration(entryCopy.start * float64(time.Second)))
				})
				rowBox.AddController(click)

				linesBox.Append(rowBox)
				rows[i] = lyricRowBinding{
					entry: entryCopy,
					row:   rowBox,
					label: lbl,
				}
			}

			bottomSpacer := gtk.NewBox(gtk.OrientationVertical, 0)
			bottomSpacer.SetSizeRequest(-1, 145)
			linesBox.Append(bottomSpacer)

			scrollable.SetChild(linesBox)

			prevActiveIdx := -2
			lastPangoPos := -1.0
			box.AddTickCallback(func(widget gtk.Widgetter, frameClock gdk.FrameClocker) bool {
				if p.CurrentIdx < 0 || p.CurrentIdx >= len(p.Queue) || p.Queue[p.CurrentIdx].ID != expectedTrackID {
					return false
				}

				now := p.EstimatedPos().Seconds() + lyricLeadSec
				activeIdx := findActiveTimelineIndex(entries, now)
				idxChanged := activeIdx != prevActiveIdx

				if idxChanged {
					for idx := range rows {
						b := &rows[idx]
						if b.entry.isInterlude {
							b.row.RemoveCSSClass("lyric-interlude-active")
							b.row.RemoveCSSClass("lyric-line-past")
							b.row.RemoveCSSClass("lyric-line-future")
							if idx < activeIdx {
								b.row.AddCSSClass("lyric-line-past")
								for _, dot := range b.dots {
									dot.RemoveCSSClass("interlude-dot-lit")
									dot.RemoveCSSClass("interlude-dot-pulse")
									dot.SetOpacity(0.22)
								}
							} else if idx == activeIdx {
								b.row.AddCSSClass("lyric-interlude-active")
							} else {
								b.row.AddCSSClass("lyric-line-future")
								for _, dot := range b.dots {
									dot.RemoveCSSClass("interlude-dot-lit")
									dot.RemoveCSSClass("interlude-dot-pulse")
									dot.SetOpacity(0.28)
								}
							}
							continue
						}

						b.row.RemoveCSSClass("lyric-row-active")
						b.label.RemoveCSSClass("lyric-line-past")
						b.label.RemoveCSSClass("lyric-line-active")
						b.label.RemoveCSSClass("lyric-line-future")

						if idx < activeIdx {
							b.label.AddCSSClass("lyric-line-past")
							b.label.SetAttributes(nil)
						} else if idx == activeIdx {
							b.row.AddCSSClass("lyric-row-active")
							b.label.AddCSSClass("lyric-line-active")
						} else {
							b.label.AddCSSClass("lyric-line-future")
							b.label.SetAttributes(nil)
						}
					}
					prevActiveIdx = activeIdx
				}

				if activeIdx >= 0 && activeIdx < len(rows) {
					active := &rows[activeIdx]

					if active.entry.isInterlude {
						dur := active.entry.end - active.entry.start
						prog := 0.0
						if dur > 0 {
							prog = math.Max(0.0, math.Min(1.0, (now-active.entry.start)/dur))
						}
						wallSec := float64(time.Now().UnixMilli()) / 1000.0
						for dIdx, dot := range active.dots {
							threshold := float64(dIdx) / 3.0
							nextThreshold := float64(dIdx+1) / 3.0
							pulse := 0.5 + 0.5*math.Sin(wallSec*4.2-float64(dIdx)*0.85)
							if prog >= nextThreshold {
								dot.AddCSSClass("interlude-dot-lit")
								dot.RemoveCSSClass("interlude-dot-pulse")
								dot.SetOpacity(0.85 + 0.15*pulse)
							} else if prog >= threshold {
								dot.AddCSSClass("interlude-dot-lit")
								dot.AddCSSClass("interlude-dot-pulse")
								local := (prog - threshold) / (nextThreshold - threshold)
								dot.SetOpacity(0.35 + 0.65*local*(0.7+0.3*pulse))
							} else {
								dot.RemoveCSSClass("interlude-dot-lit")
								dot.RemoveCSSClass("interlude-dot-pulse")
								dot.SetOpacity(0.22 + 0.12*pulse)
							}
						}
					} else if len(active.entry.words) > 0 && active.label != nil && (idxChanged || math.Abs(now-lastPangoPos) >= 0.002) {
						lastPangoPos = now
						attrList := pango.NewAttrList()
						for _, w := range active.entry.words {
							prog := computeWordProgress(now, w.start, w.end)

							if prog <= 0.0 {
								alphaAttr := pango.NewAttrForegroundAlpha(lyricUnsungAlpha)
								alphaAttr.SetStartIndex(w.startByte)
								alphaAttr.SetEndIndex(w.endByte)
								attrList.Insert(alphaAttr)
							} else if prog >= 1.0 {
								// Subtle peak glow that smoothly settles after the word finishes
								alpha := lyricSungAlpha
								if elapsed := now - w.end; elapsed >= 0 && elapsed < lyricAfterglowSec {
									t := elapsed / lyricAfterglowSec
									alpha = uint16(math.Round(float64(lyricActiveWordAlpha) - t*float64(lyricActiveWordAlpha-lyricSungAlpha)))
								}
								alphaAttr := pango.NewAttrForegroundAlpha(alpha)
								alphaAttr.SetStartIndex(w.startByte)
								alphaAttr.SetEndIndex(w.endByte)
								attrList.Insert(alphaAttr)
							} else {
								alphas := computeWordRuneAlphas(prog, len(w.runes), lyricUnsungAlpha, lyricActiveWordAlpha)
								for rIdx, rSpan := range w.runes {
									alphaAttr := pango.NewAttrForegroundAlpha(alphas[rIdx])
									alphaAttr.SetStartIndex(rSpan.startByte)
									alphaAttr.SetEndIndex(rSpan.endByte)
									attrList.Insert(alphaAttr)
								}
							}
						}
						active.label.SetAttributes(attrList)
					}

					if time.Now().After(userScrollUntil) {
						scrollable.RemoveCSSClass("user-scrolling")
						if bounds, ok := active.row.ComputeBounds(linesBox); ok {
							viewportH := float64(scrollable.Height())
							if viewportH > 0 {
								vadj := scrollable.VAdjustment()
								if vadj != nil {
									targetY := float64(bounds.Y()) + (float64(bounds.Height()) * 0.5) - (viewportH * 0.5)
									maxScroll := math.Max(0.0, vadj.Upper()-vadj.PageSize())
									targetY = math.Max(0.0, math.Min(maxScroll, targetY))

									curY := vadj.Value()
									diff := targetY - curY
									if math.Abs(diff) > 0.5 {
										vadj.SetValue(curY + diff*0.11)
									}
								}
							}
						}
					}
				}

				return true
			})
		})
	}()

	return box
}
