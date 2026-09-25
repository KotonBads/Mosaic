package ui

import (
	"fmt"
	"os"
	"slices"

	"github.com/KotonBads/mosaic/player"
	"github.com/charmbracelet/log"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

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

	library.Player.SortAlphabetically()
	library.Player.CurrentIdx = 0

	library.Player.MPV = &player.Client{}
	err = library.Player.MPV.Init()
	if err != nil {
		logger.Error("Failed to init MPV client", "err", err)
	}

	app := NewWindow(func(win *adw.ApplicationWindow) {
		logger.Info("Building Libadwaita OverlaySplitView responsive layout")

		split_view := adw.NewOverlaySplitView()
		split_view.SetHExpand(true)
		split_view.SetVExpand(true)
		split_view.SetMinSidebarWidth(260)
		split_view.SetMaxSidebarWidth(450)
		split_view.SetSidebarWidthFraction(0.35)

		player_refresh := func(track player.Track) {
			library.Player.CurrentIdx = slices.IndexFunc(library.Player.Queue, func(found player.Track) bool {
				if found.ID == track.ID {
					return true
				}
				return false
			})
			player_area := gtk.NewBox(gtk.OrientationVertical, 40)
			player_area.SetVAlign(gtk.AlignCenter)
			player_area.SetMarginStart(24)
			player_area.SetMarginEnd(24)
			player_area.SetMarginTop(24)
			player_area.SetMarginBottom(24)

			album_art := gtk.NewAspectFrame(0.5, 0.5, 1.0, false)
			album_art.SetOverflow(gtk.OverflowHidden)
			album_art.AddCSSClass("album-art")
			album_art.SetSizeRequest(120, 120)

			picture, err := AlbumArt(track)
			if err != nil {
				return
			}
			album_art.SetChild(picture)

			album_clamped := adw.NewClamp()
			album_clamped.SetChild(album_art)
			album_clamped.SetMaximumSize(320)
			player_area.Append(album_clamped)

			song_info := gtk.NewBox(gtk.OrientationVertical, 0)
			song_info.SetHExpand(true)
			song_info.SetVExpand(true)
			song_info.SetHAlign(gtk.AlignCenter)

			song_title := gtk.NewLabel(track.Title)
			song_title.AddCSSClass("title-4")
			song_info.Append(song_title)

			song_artist := gtk.NewLabel(track.Artists[0].Name)
			song_artist.AddCSSClass("body")
			song_artist.AddCSSClass("dim-label")
			song_info.Append(song_artist)

			player_area.Append(song_info)

			controls := PlayerControls(library.Player)
			controls_clamped := adw.NewClamp()
			controls_clamped.SetChild(controls)
			controls_clamped.SetMaximumSize(420)
			player_area.Append(controls_clamped)

			split_view.SetContent(player_area)
		}

		queue_refresh := func() {
			queue := Queue(library.Player, func(track player.Track) {
				library.Player.PlayTrack(library.Player.CurrentIdx)
				player_refresh(track)
			})
			split_view.SetSidebar(queue)
		}

		queue_refresh()
		logger.Info("Setting player screen to song index: ", "index", library.Player.CurrentIdx)
		player_refresh(library.Player.Queue[library.Player.CurrentIdx])

		library.Player.MPV.OnTrackEnd = func(reason string) {
			if reason == "stop" {
				return
			}
			library.Player.Next()
		}
		library.Player.Subscribe(player.OnTrackChange, player_refresh)

		win.SetContent(split_view)
	})

	exitCode := app.Run(os.Args)
	if exitCode != 0 {
		logger.Warn("Application exited with non-zero status", "code", exitCode)
	} else {
		logger.Info("Mosaic shutdown gracefully")
	}
	library.Player.MPV.Close()
	os.Exit(exitCode)
}
