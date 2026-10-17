package ui

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/KotonBads/mosaic/player"
	"github.com/charmbracelet/log"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"

	_ "embed"
)

var logger = log.WithPrefix("ui")

func test_queue() []*gtk.ListBoxRow {
	queue := make([]*gtk.ListBoxRow, 0, 100)

	for idx := range 100 {
		row := gtk.NewListBoxRow()
		row.SetChild(gtk.NewLabel(fmt.Sprintf("Item %d", idx)))
		queue = append(queue, row)
	}
	return queue
}

func scrolled_list(p *player.Player, onSelect func(idx int)) (*gtk.ListBox, gtk.Widgetter) {
	list := gtk.NewListBox()
	list.SetVExpand(true)
	list.AddCSSClass("sidebar")

	items := p.Queue

	for _, item := range items {
		row := gtk.NewListBoxRow()
		row.SetChild(QueueElement(item))
		list.Append(row)
	}

	list.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		if row == nil {
			return
		}
		idx := row.Index()
		if idx >= 0 && idx < len(items) {
			p.PlayTrack(idx)
			if onSelect != nil {
				onSelect(idx)
			}
		}
	})

	logger.Debug("Initialized queue list", "itemCount", len(items))

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scrolled.SetChild(list)
	return list, scrolled
}

func App() {
	library := player.Library{}

	configDir, err := os.UserConfigDir()
	if err != nil {
		logger.Error("Failed to get user config dir", "err", err)
	}
	library.Open(configDir + "/mosaic/library.db")
	library.Index("/home/koton-bads/Music/")
	library.Load()

	library.Player = &player.Player{
		Queue: library.Tracks,
	}
	p := library.Player

	p.SortAlphabetically()
	p.CurrentIdx = 0

	p.MPV = &player.Client{}
	err = p.MPV.Init()
	if err != nil {
		logger.Error("Failed to init MPV client", "err", err)
	}
	p.MPV.OnTimePos = func(pos float64) {
		p.UpdatePos(time.Duration(pos * float64(time.Second)))
	}

	p.MPRIS = &player.MPRIS{}
	p.MPRIS.Init(p)

	logger.Info("Prefetching lyrics")
	go func() {
		client := GetLyricsClient()
		var wg sync.WaitGroup
		sem := make(chan struct{}, 4)
		for _, track := range p.Queue {
			wg.Add(1)
			go func(t player.Track) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				request := BuildLyricsRequest(t)
				client.Lookup(context.TODO(), request)
			}(track)
		}
		wg.Wait()
		logger.Info("Lyrics prefetch complete")
	}()

	app := NewWindow(func(win *adw.ApplicationWindow) {
		logger.Info("Building Libadwaita OverlaySplitView responsive layout")

		root_overlay := gtk.NewOverlay()
		root_overlay.SetHExpand(true)
		root_overlay.SetVExpand(true)
		root_overlay.SetOverflow(gtk.OverflowHidden)
		root_overlay.AddCSSClass("ambient-overlay")

		bg_picture := gtk.NewPicture()
		bg_picture.SetCanShrink(true)
		bg_picture.SetContentFit(gtk.ContentFitCover)
		bg_picture.SetHExpand(true)
		bg_picture.SetVExpand(true)
		bg_picture.AddCSSClass("ambient-bg")
		root_overlay.SetChild(bg_picture)

		split_view := adw.NewOverlaySplitView()
		split_view.SetHExpand(true)
		split_view.SetVExpand(true)
		split_view.SetMinSidebarWidth(260)
		split_view.SetMaxSidebarWidth(450)
		split_view.SetSidebarWidthFraction(0.35)

		root_overlay.AddOverlay(split_view)

		breakpoint := adw.NewBreakpoint(adw.BreakpointConditionParse("max-width: 630px"))
		breakpoint.AddSetter(split_view, "collapsed", true)

		showLyrics := false

		player_refresh := func(track player.Track) {
			p.ResetViewListeners()

			p.CurrentIdx = slices.IndexFunc(p.Queue, func(found player.Track) bool {
				return found.ID == track.ID
			})

			ambientTex, err := GetBlurredAmbientArt(track)
			if err == nil && ambientTex != nil {
				bg_picture.SetPaintable(ambientTex)
			}

			player_area := gtk.NewBox(gtk.OrientationVertical, 22)
			player_area.AddCSSClass("player-stage")
			player_area.SetVAlign(gtk.AlignCenter)
			player_area.SetHExpand(true)
			player_area.SetVExpand(true)
			player_area.SetMarginStart(24)
			player_area.SetMarginEnd(24)
			player_area.SetMarginTop(24)
			player_area.SetMarginBottom(24)

			stage_stack := gtk.NewStack()
			stage_stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
			stage_stack.SetTransitionDuration(280)
			stage_stack.SetHhomogeneous(false)
			stage_stack.SetVhomogeneous(true)
			stage_stack.SetHAlign(gtk.AlignCenter)
			stage_stack.SetVAlign(gtk.AlignCenter)

			album_art := gtk.NewAspectFrame(0.5, 0.5, 1.0, false)
			album_art.SetOverflow(gtk.OverflowHidden)
			album_art.AddCSSClass("album-art")
			album_art.SetSizeRequest(320, 320)
			album_art.SetCursorFromName("pointer")
			album_art.SetTooltipText("Click to view synced lyrics")

			picture, err := AlbumArt(track)
			if err == nil && picture != nil {
				album_art.SetChild(picture)
			}

			art_clamped := adw.NewClamp()
			art_clamped.SetChild(album_art)
			art_clamped.SetMaximumSize(320)

			art_click := gtk.NewGestureClick()
			art_click.SetPropagationPhase(gtk.PhaseBubble)
			art_click.ConnectReleased(func(nPress int, x, y float64) {
				showLyrics = true
				stage_stack.SetVisibleChildName("lyrics")
			})
			album_art.AddController(art_click)

			stage_stack.AddNamed(art_clamped, "art")

			lyricsWidget := Lyrics(p, func() {
				showLyrics = false
				stage_stack.SetVisibleChildName("art")
			})

			lyrics_clamped := adw.NewClamp()
			lyrics_clamped.SetChild(lyricsWidget)
			lyrics_clamped.SetMaximumSize(440)

			stage_stack.AddNamed(lyrics_clamped, "lyrics")

			if showLyrics {
				stage_stack.SetVisibleChildName("lyrics")
			} else {
				stage_stack.SetVisibleChildName("art")
			}

			player_area.Append(stage_stack)

			song_info := gtk.NewBox(gtk.OrientationVertical, 4)
			song_info.AddCSSClass("song-info-box")
			song_info.SetHExpand(true)
			song_info.SetHAlign(gtk.AlignCenter)
			song_info.SetMarginTop(4)

			song_title := gtk.NewLabel(track.Title)
			song_title.AddCSSClass("track-title")
			song_title.SetEllipsize(pango.EllipsizeEnd)
			song_title.SetMaxWidthChars(36)
			song_info.Append(song_title)

			artistName := join_artist_name(track.Artists)
			if artistName != "" {
				song_artist := gtk.NewLabel(artistName)
				song_artist.AddCSSClass("track-artist")
				song_artist.SetEllipsize(pango.EllipsizeEnd)
				song_artist.SetMaxWidthChars(40)
				song_info.Append(song_artist)
			}

			if track.Album.Title != "" && track.Album.Title != track.Title {
				song_album := gtk.NewLabel(track.Album.Title)
				song_album.AddCSSClass("track-album")
				song_album.SetEllipsize(pango.EllipsizeEnd)
				song_album.SetMaxWidthChars(42)
				song_info.Append(song_album)
			}

			player_area.Append(song_info)

			controls := PlayerControls(p)
			controls_clamped := adw.NewClamp()
			controls_clamped.SetChild(controls)
			controls_clamped.SetMaximumSize(420)
			player_area.Append(controls_clamped)

			split_view.SetContent(player_area)
		}

		queue_refresh := func() {
			queue := Queue(p, func(track player.Track) {
				p.PlayTrack(p.CurrentIdx)
			})
			split_view.SetSidebar(queue)
		}

		queue_refresh()
		logger.Info("Setting player screen to song index: ", "index", p.CurrentIdx)
		player_refresh(p.Queue[p.CurrentIdx])
		p.SetVolume(20)
		p.Notify()

		p.MPV.OnTrackEnd = func(reason string) {
			if reason == "stop" {
				return
			}
			p.Next()
		}
		p.Subscribe(player.OnTrackChange, player_refresh)

		win.SetContent(root_overlay)
		win.AddBreakpoint(breakpoint)
	})

	exitCode := app.Run(os.Args)
	if exitCode != 0 {
		logger.Warn("Application exited with non-zero status", "code", exitCode)
	} else {
		logger.Info("Mosaic shutdown gracefully")
	}
	p.MPV.Close()
	os.Exit(exitCode)
}
