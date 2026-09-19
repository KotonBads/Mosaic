package ui

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/KotonBads/mosaic/player"
	"github.com/charmbracelet/log"
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

func scrolled_list(items []player.Track, onSelect func(track player.Track)) gtk.Widgetter {
	list := gtk.NewListBox()
	list.SetVExpand(true)
	list.AddCSSClass("sidebar")

	for _, item := range items {
		row := gtk.NewListBoxRow()
		row.SetChild(QueueElement(item))
		list.Append(row)
	}

	list.ConnectRowSelected(func(row *gtk.ListBoxRow) {
		if row == nil {
			return
		}
		idx := row.Index()
		if idx >= 0 && idx < len(items) {
			onSelect(items[idx])
		}
	})

	logger.Debug("Initialized queue list", "itemCount", len(items))

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scrolled.SetChild(list)
	return scrolled
}

func header() gtk.Widgetter {
	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(false)

	return header
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

	app := NewWindow(func(win *gtk.ApplicationWindow) {
		logger.Info("Prefetching album art")
		go func() {
			for _, track := range library.Tracks {
				GetAlbumArt(track)
				GetAlbumThumb(track)
			}
			logger.Info("Prefetching complete")
		}()

		logger.Info("Building main UI layout")
		pane := gtk.NewPaned(gtk.OrientationHorizontal)
		pane.SetPosition(360)
		pane.SetResizeStartChild(false)
		pane.SetResizeEndChild(true)

		// Initial right pane placeholder
		placeholder := gtk.NewBox(gtk.OrientationVertical, 8)
		placeholder.SetHExpand(true)
		placeholder.SetVExpand(true)
		placeholder.SetHAlign(gtk.AlignCenter)
		placeholder.SetVAlign(gtk.AlignCenter)

		lblPrompt := gtk.NewLabel("Select a track from the queue to view album art")
		lblPrompt.AddCSSClass("dim-label")
		placeholder.Append(lblPrompt)

		pane.SetEndChild(placeholder)

		sortedTracks := slices.SortedStableFunc(slices.Values(library.Tracks), func(a, b player.Track) int {
			return strings.Compare(a.Title, b.Title)
		})

		list := scrolled_list(sortedTracks, func(track player.Track) {
			pic, err := GetAlbumArt(track)
			if err != nil {
				logger.Warn("Failed to load album art", "track", track.Title, "err", err)
				return
			}

			pic.SetCanShrink(true)
			pic.SetContentFit(gtk.ContentFitCover)

			frame := gtk.NewFrame("")
			frame.AddCSSClass("album-art")
			frame.SetOverflow(gtk.OverflowHidden)
			frame.SetChild(pic)

			af := gtk.NewAspectFrame(0.5, 0.5, 1.0, false)
			af.SetHExpand(true)
			af.SetVExpand(true)
			af.SetChild(frame)

			detailBox := gtk.NewBox(gtk.OrientationVertical, 14)
			detailBox.SetHExpand(true)
			detailBox.SetVExpand(true)
			detailBox.SetMarginTop(24)
			detailBox.SetMarginBottom(24)
			detailBox.SetMarginStart(24)
			detailBox.SetMarginEnd(24)

			titleLabel := gtk.NewLabel(track.Title)
			titleLabel.AddCSSClass("title-1")
			titleLabel.SetHAlign(gtk.AlignCenter)

			artistLabel := gtk.NewLabel(join_artist_name(track.Artists))
			artistLabel.AddCSSClass("dim-label")
			artistLabel.AddCSSClass("title-4")
			artistLabel.SetHAlign(gtk.AlignCenter)

			detailBox.Append(af)
			detailBox.Append(titleLabel)
			detailBox.Append(artistLabel)

			pane.SetEndChild(detailBox)
		})

		box := gtk.NewBox(gtk.OrientationVertical, 4)
		box.SetVExpand(true)
		box.SetHExpand(true)
		box.Append(header())
		box.Append(list)

		pane.SetStartChild(box)
		win.SetChild(pane)
	})

	exitCode := app.Run(os.Args)
	if exitCode != 0 {
		logger.Warn("Application exited with non-zero status", "code", exitCode)
	} else {
		logger.Info("Mosaic shutdown gracefully")
	}
	os.Exit(exitCode)
}
