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

	app := NewWindow(func(win *adw.ApplicationWindow) {
		logger.Info("Prefetching album art")
		go func() {
			for _, track := range library.Tracks {
				GetAlbumArt(track)
				GetAlbumThumb(track)
			}
			logger.Info("Prefetching complete")
		}()

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

			// glib.IdleAdd(library.Player.ControlChange)
			split_view.SetContent(player_area)
		}

		queue_refresh := func() {
			queue := Queue(library.Player, player_refresh)
			split_view.SetSidebar(queue)
		}

		queue_refresh()

		library.Player.Subscribe(player.QueueChange, queue_refresh)
		library.Player.Subscribe(player.OnTrackChange, player_refresh)

		win.SetContent(split_view)
		// 		// Detail container holding either the placeholder or track details
		// 		detailContainer := gtk.NewBox(gtk.OrientationVertical, 0)
		// 		detailContainer.SetHExpand(true)
		// 		detailContainer.SetVExpand(true)
		//
		// 		setDetail := func(w gtk.Widgetter) {
		// 			for child := detailContainer.FirstChild(); child != nil; {
		// 				next := gtk.BaseWidget(child).NextSibling()
		// 				detailContainer.Remove(child)
		// 				child = next
		// 			}
		// 			if w != nil {
		// 				detailContainer.Append(w)
		// 			}
		// 		}
		//
		// 		// Initial right pane placeholder
		// 		placeholder := gtk.NewBox(gtk.OrientationVertical, 8)
		// 		placeholder.SetHExpand(true)
		// 		placeholder.SetVExpand(true)
		// 		placeholder.SetHAlign(gtk.AlignCenter)
		// 		placeholder.SetVAlign(gtk.AlignCenter)
		//
		// 		lblPrompt := gtk.NewLabel("Select a track from the queue to view album art")
		// 		lblPrompt.AddCSSClass("dim-label")
		// 		placeholder.Append(lblPrompt)
		//
		// 		setDetail(placeholder)
		//
		// 		// Overlay split view setup
		// 		splitView := adw.NewOverlaySplitView()
		// 		splitView.SetSidebarWidthFraction(0.32)
		// 		splitView.SetMinSidebarWidth(260)
		// 		splitView.SetMaxSidebarWidth(420)
		// 		splitView.SetEnableShowGesture(true)
		// 		splitView.SetEnableHideGesture(true)
		//
		// 		// Header Bar toggle button for small screens / collapsed mode
		// 		toggleQueueBtn := gtk.NewButtonFromIconName("view-list-symbolic")
		// 		toggleQueueBtn.SetTooltipText("Toggle Queue")
		// 		toggleQueueBtn.SetVisible(false)
		//
		// 		toggleQueueBtn.ConnectClicked(func() {
		// 			splitView.SetShowSidebar(!splitView.ShowSidebar())
		// 		})
		//
		// 		var (
		// 			sidebarList *gtk.ListBox
		// 			showTrack   func(idx int)
		// 		)
		//
		// 		showTrack = func(idx int) {
		// 			if idx < 0 || idx >= len(library.Player.Queue) {
		// 				return
		// 			}
		//
		// 			// Highlight the track row in the sidebar
		// 			if sidebarList != nil {
		// 				if row := sidebarList.RowAtIndex(idx); row != nil {
		// 					sidebarList.SelectRow(row)
		// 				}
		// 			}
		//
		// 			track := library.Player.Queue[idx]
		//
		// 			pic, err := GetAlbumArt(track)
		// 			if err != nil {
		// 				logger.Warn("Failed to load album art", "track", track.Title, "err", err)
		// 				return
		// 			}
		//
		// 			pic.SetCanShrink(true)
		// 			pic.SetContentFit(gtk.ContentFitCover)
		//
		// 			frame := gtk.NewFrame("")
		// 			frame.AddCSSClass("album-art")
		// 			frame.SetOverflow(gtk.OverflowHidden)
		// 			frame.SetChild(pic)
		//
		// 			af := gtk.NewAspectFrame(0.5, 0.5, 1.0, false)
		// 			af.SetHExpand(true)
		// 			af.SetVExpand(true)
		// 			af.SetChild(frame)
		//
		// 			detailBox := gtk.NewBox(gtk.OrientationVertical, 14)
		// 			detailBox.SetHExpand(true)
		// 			detailBox.SetVExpand(true)
		// 			detailBox.SetMarginTop(24)
		// 			detailBox.SetMarginBottom(24)
		// 			detailBox.SetMarginStart(24)
		// 			detailBox.SetMarginEnd(24)
		//
		// 			titleLabel := gtk.NewLabel(track.Title)
		// 			titleLabel.AddCSSClass("title-1")
		// 			titleLabel.SetHAlign(gtk.AlignCenter)
		//
		// 			artistLabel := gtk.NewLabel(join_artist_name(track.Artists))
		// 			artistLabel.AddCSSClass("dim-label")
		// 			artistLabel.AddCSSClass("title-4")
		// 			artistLabel.SetHAlign(gtk.AlignCenter)
		//
		// 			detailBox.Append(af)
		// 			detailBox.Append(titleLabel)
		// 			detailBox.Append(artistLabel)
		// 			detailBox.Append(PlayerControls(library.Player))
		//
		// 			setDetail(detailBox)
		// 		}
		//
		// 		// When track changes via Next/Prev/click, propagate to UI
		// 		library.Player.OnTrackChange = func(idx int) {
		// 			showTrack(idx)
		// 		}
		//
		// 		list, listWidget := scrolled_list(library.Player, func(idx int) {
		// 			// When collapsed, auto-hide drawer after track selection to focus on player
		// 			if splitView.Collapsed() {
		// 				splitView.SetShowSidebar(false)
		// 			}
		// 		})
		// 		sidebarList = list
		//
		// 		queueBox := gtk.NewBox(gtk.OrientationVertical, 0)
		// 		queueBox.SetVExpand(true)
		// 		queueBox.SetHExpand(true)
		// 		queueBox.AddCSSClass("background")
		// 		queueBox.Append(listWidget)
		//
		// 		splitView.SetSidebar(queueBox)
		// 		splitView.SetContent(detailContainer)
		//
		// 		// Responsive Libadwaita Breakpoint (<= 760px)
		// 		bp := adw.NewBreakpoint(adw.BreakpointConditionParse("max-width: 760px"))
		// 		bp.ConnectApply(func() {
		// 			logger.Debug("Applying narrow breakpoint: collapsing split view")
		// 			splitView.SetCollapsed(true)
		// 			toggleQueueBtn.SetVisible(true)
		// 		})
		// 		bp.ConnectUnapply(func() {
		// 			logger.Debug("Unapplying narrow breakpoint: uncollapsing split view")
		// 			splitView.SetCollapsed(false)
		// 			toggleQueueBtn.SetVisible(false)
		// 		})
		//
		// 		win.AddBreakpoint(bp)
		//
		// 		// Adwaita ToolbarView and HeaderBar
		// 		headerBar := adw.NewHeaderBar()
		// 		headerBar.PackStart(toggleQueueBtn)
		//
		// 		toolbar := adw.NewToolbarView()
		// 		toolbar.AddTopBar(headerBar)
		// 		toolbar.SetContent(splitView)
		//
		// 		win.SetContent(toolbar)
	})

	exitCode := app.Run(os.Args)
	if exitCode != 0 {
		logger.Warn("Application exited with non-zero status", "code", exitCode)
	} else {
		logger.Info("Mosaic shutdown gracefully")
	}
	os.Exit(exitCode)
}
