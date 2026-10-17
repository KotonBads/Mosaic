package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/KotonBads/mosaic/player"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

func AlbumArt(track player.Track) (*gtk.Picture, error) {
	picture, err := GetAlbumArt(track)
	if err != nil {
		return nil, err
	}

	picture.SetHAlign(gtk.AlignCenter)
	picture.SetVAlign(gtk.AlignCenter)
	picture.AddCSSClass("album-art")

	return picture, nil
}

func volumeIconName(vol int) string {
	switch {
	case vol <= 0:
		return "audio-volume-muted-symbolic"
	case vol < 34:
		return "audio-volume-low-symbolic"
	case vol < 67:
		return "audio-volume-medium-symbolic"
	default:
		return "audio-volume-high-symbolic"
	}
}

func PlayerControls(p *player.Player) gtk.Widgetter {
	controls_box := gtk.NewBox(gtk.OrientationHorizontal, 16)
	controls_box.AddCSSClass("transport-row")
	controls_box.SetMarginTop(6)
	controls_box.SetMarginBottom(6)

	seek_box := gtk.NewBox(gtk.OrientationHorizontal, 10)
	seek_box.AddCSSClass("seek-row")

	vol_box := gtk.NewBox(gtk.OrientationHorizontal, 10)
	vol_box.AddCSSClass("volume-row")

	final_box := gtk.NewBox(gtk.OrientationVertical, 8)
	final_box.AddCSSClass("player-controls")

	is_scrubbing := false
	is_changing_vol := false

	play_pause := gtk.NewButtonFromIconName("media-playback-pause-symbolic")
	play_pause.AddCSSClass("play-pause-btn")
	play_pause.AddCSSClass("circular")
	play_pause.SetCursorFromName("pointer")

	previous := gtk.NewButtonFromIconName("media-skip-backward-symbolic")
	previous.AddCSSClass("transport-btn")
	previous.AddCSSClass("flat")
	previous.AddCSSClass("circular")
	previous.SetCursorFromName("pointer")

	next := gtk.NewButtonFromIconName("media-skip-forward-symbolic")
	next.AddCSSClass("transport-btn")
	next.AddCSSClass("flat")
	next.AddCSSClass("circular")
	next.SetCursorFromName("pointer")

	shuffle := gtk.NewButtonFromIconName("media-playlist-shuffle-symbolic")
	shuffle.AddCSSClass("mode-btn")
	shuffle.AddCSSClass("flat")
	shuffle.AddCSSClass("circular")
	shuffle.SetCursorFromName("pointer")

	repeat := gtk.NewButtonFromIconName("media-playlist-repeat-symbolic")
	repeat.AddCSSClass("mode-btn")
	repeat.AddCSSClass("flat")
	repeat.AddCSSClass("circular")
	repeat.SetCursorFromName("pointer")

	seek_bar := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 100, 1)
	seek_bar.AddCSSClass("seek-bar")
	seek_bar.SetCursorFromName("pointer")

	pos_cur := gtk.NewLabel(format_time(p.Pos))
	pos_cur.AddCSSClass("time-label")
	pos_cur.AddCSSClass("numeric")
	pos_cur.SetWidthChars(5)
	pos_cur.SetXAlign(0)

	pos_max := gtk.NewLabel(format_time(p.Queue[p.CurrentIdx].Duration))
	pos_max.AddCSSClass("time-label")
	pos_max.AddCSSClass("numeric")
	pos_max.SetWidthChars(5)
	pos_max.SetXAlign(1)

	vol_icon := gtk.NewButtonFromIconName(volumeIconName(p.Volume))
	vol_icon.AddCSSClass("volume-icon")
	vol_icon.AddCSSClass("flat")
	vol_icon.AddCSSClass("circular")
	vol_icon.SetCursorFromName("pointer")
	vol_icon.SetTooltipText("Toggle Mute")

	volume := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 100, 1)
	volume.AddCSSClass("volume-bar")
	volume.SetCursorFromName("pointer")

	vol_cur := gtk.NewLabel(fmt.Sprint(p.Volume))
	vol_cur.AddCSSClass("time-label")
	vol_cur.AddCSSClass("numeric")
	vol_cur.SetWidthChars(3)
	vol_cur.SetXAlign(1)

	controls_box.SetHAlign(gtk.AlignCenter)
	seek_box.SetHAlign(gtk.AlignFill)
	seek_box.SetHExpand(true)
	seek_bar.SetHExpand(true)
	seek_bar.SetDrawValue(false)
	vol_box.SetHAlign(gtk.AlignFill)
	vol_box.SetHExpand(true)
	volume.SetHExpand(true)
	volume.SetDrawValue(false)
	final_box.SetHAlign(gtk.AlignFill)
	final_box.SetHExpand(true)

	if p.Queue[p.CurrentIdx].Duration > 0 {
		seek_bar.SetValue(p.Pos.Seconds() * 100 / p.Queue[p.CurrentIdx].Duration.Seconds())
	}

	controls_box.Append(shuffle)
	controls_box.Append(previous)
	controls_box.Append(play_pause)
	controls_box.Append(next)
	controls_box.Append(repeat)
	seek_box.Append(pos_cur)
	seek_box.Append(seek_bar)
	seek_box.Append(pos_max)
	vol_box.Append(vol_icon)
	vol_box.Append(volume)
	vol_box.Append(vol_cur)
	final_box.Append(seek_box)
	final_box.Append(controls_box)
	final_box.Append(vol_box)

	refresh := func() {
		if p == nil {
			return
		}
		if p.Paused {
			play_pause.SetIconName("media-playback-start-symbolic")
		} else {
			play_pause.SetIconName("media-playback-pause-symbolic")
		}

		if p.Shuffle {
			shuffle.SetIconName("media-playlist-shuffle-symbolic")
			shuffle.RemoveCSSClass("dim-label")
			shuffle.AddCSSClass("mode-active")
		} else {
			shuffle.SetIconName("media-playlist-shuffle-symbolic")
			shuffle.AddCSSClass("dim-label")
			shuffle.RemoveCSSClass("mode-active")
		}

		if p.Repeat == player.RepeatOne {
			repeat.SetIconName("media-playlist-repeat-song-symbolic")
			repeat.RemoveCSSClass("dim-label")
			repeat.AddCSSClass("mode-active")
		} else if p.Repeat == player.RepeatQueue {
			repeat.SetIconName("media-playlist-repeat-symbolic")
			repeat.RemoveCSSClass("dim-label")
			repeat.AddCSSClass("mode-active")
		} else {
			repeat.SetIconName("media-playlist-repeat-symbolic")
			repeat.AddCSSClass("dim-label")
			repeat.RemoveCSSClass("mode-active")
		}

		if !is_changing_vol {
			vol_cur.SetText(fmt.Sprint(p.Volume))
			volume.SetValue(float64(p.Volume))
		}
		vol_icon.SetIconName(volumeIconName(p.Volume))
		pos_cur.SetText(format_time(p.Pos))
		pos_max.SetText(format_time(p.Queue[p.CurrentIdx].Duration))
	}

	if p != nil {
		seek_click := gtk.NewGestureClick()
		vol_click := gtk.NewGestureClick()

		// this is a workaround to have release events
		// for the seek bar
		// https://gitlab.gnome.org/GNOME/gtk/-/work_items/4939#note_1680234
		seek_controllers := seek_bar.ObserveControllers()
		for i := range seek_controllers.NItems() {
			g, ok := seek_controllers.Item(i).Cast().(*gtk.GestureClick)
			if ok && g != nil {
				seek_click = g
				break
			}
		}

		vol_controllers := volume.ObserveControllers()
		for i := range vol_controllers.NItems() {
			g, ok := vol_controllers.Item(i).Cast().(*gtk.GestureClick)
			if ok && g != nil {
				vol_click = g
				break
			}
		}

		play_pause.ConnectClicked(p.PlayPause)
		repeat.ConnectClicked(p.SetRepeat)
		shuffle.ConnectClicked(p.SetShuffle)
		next.ConnectClicked(p.Next)
		previous.ConnectClicked(p.Prev)
		vol_icon.ConnectClicked(p.ToggleMute)

		seek_bar.ConnectChangeValue(func(scroll gtk.ScrollType, value float64) (ok bool) {
			track := p.Queue[p.CurrentIdx]
			ratio := value / 100
			pos_cur.SetText(format_time(time.Duration(ratio * float64(track.Duration))))
			return false
		})
		seek_click.ConnectPressed(func(nPress int, x, y float64) {
			logger.Info("pressed, n press", "n", nPress)
			is_scrubbing = true
		})
		seek_click.ConnectReleased(func(nPress int, x, y float64) {
			logger.Info("released, n press", "n", nPress)

			track := p.Queue[p.CurrentIdx]
			ratio := seek_bar.Value() / 100
			p.Seek(time.Duration(ratio * float64(track.Duration)))
			is_scrubbing = false
		})

		volume.ConnectChangeValue(func(scroll gtk.ScrollType, value float64) (ok bool) {
			is_changing_vol = true
			int_val := max(0, min(100, int(value)))
			p.SetVolume(int_val)
			vol_cur.SetText(fmt.Sprint(p.Volume))
			vol_icon.SetIconName(volumeIconName(p.Volume))
			return false
		})

		vol_click.ConnectPressed(func(nPress int, x, y float64) {
			is_changing_vol = true
		})
		vol_click.ConnectReleased(func(nPress int, x, y float64) {
			is_changing_vol = false
			volume.SetValue(float64(p.Volume))
			vol_cur.SetText(fmt.Sprint(p.Volume))
			vol_icon.SetIconName(volumeIconName(p.Volume))
		})

		p.Subscribe(player.PosChange, func(pos time.Duration) {
			if is_scrubbing {
				return
			}
			track := p.Queue[p.CurrentIdx]
			if track.Duration > 0 {
				seek_bar.SetValue(pos.Seconds() * 100 / track.Duration.Seconds())
			}
			refresh()
		})

		p.Subscribe(player.ControlChange, refresh)
	}

	refresh()

	return final_box
}

func QueueElement(track player.Track) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 12)
	text_box := gtk.NewBox(gtk.OrientationVertical, 0)

	artists := make([]string, 0, len(track.Artists))
	for _, artist := range track.Artists {
		artists = append(artists, artist.Name)
	}

	picture, err := GetAlbumThumb(track)
	if err != nil || picture == nil {
		logger.Warn("Failed to load album thumbnail", "track", track.Title, "err", err)
		picture = gtk.NewPicture()
	}

	picture.SetCanShrink(true)
	picture.SetContentFit(gtk.ContentFitContain)
	picture.SetSizeRequest(42, 42)
	picture.SetHAlign(gtk.AlignCenter)
	picture.SetVAlign(gtk.AlignCenter)

	frame := gtk.NewFrame("")
	frame.SetSizeRequest(42, 42)
	frame.SetHAlign(gtk.AlignCenter)
	frame.SetVAlign(gtk.AlignCenter)
	frame.AddCSSClass("thumb-art")
	frame.SetOverflow(gtk.OverflowHidden)
	frame.SetChild(picture)

	title := gtk.NewLabel(track.Title)
	title.SetHAlign(gtk.AlignStart)
	title.SetEllipsize(pango.EllipsizeEnd)
	title.AddCSSClass("heading")

	artist := gtk.NewLabel(strings.Join(artists, ", "))
	artist.SetHAlign(gtk.AlignStart)
	artist.SetEllipsize(pango.EllipsizeEnd)
	artist.AddCSSClass("dim-label")

	text_box.Append(title)
	text_box.Append(artist)
	text_box.SetHExpand(true)

	box.SetMarginTop(4)
	box.SetMarginBottom(4)
	box.SetMarginStart(4)
	box.SetMarginEnd(4)
	box.AddCSSClass("queue")

	box.Append(frame)
	box.Append(text_box)

	return box
}

func Queue(p *player.Player, onChange func(track player.Track)) gtk.Widgetter {
	indices := make([]string, len(p.Queue))
	for i, t := range p.Queue {
		indices[i] = t.Title
	}
	model := gtk.NewStringList(indices)
	selection := gtk.NewSingleSelection(model)
	selection.SetAutoselect(false)
	selection.SetCanUnselect(false)
	if len(p.Queue) > 0 && p.CurrentIdx >= 0 && p.CurrentIdx < len(p.Queue) {
		selection.SetSelected(uint(p.CurrentIdx))
	}

	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(obj *glib.Object) {
		list_item := obj.Cast().(*gtk.ListItem)
		list_item.SetFocusable(false)
		item := QueueElement(p.Queue[p.CurrentIdx])

		list_item.SetChild(item)
	})

	factory.ConnectBind(func(obj *glib.Object) {
		list_item := obj.Cast().(*gtk.ListItem)
		item := QueueElement(p.Queue[list_item.Position()])
		list_item.SetChild(item)
	})

	list_view := gtk.NewListView(selection, &factory.ListItemFactory)
	list_view.SetSingleClickActivate(true)
	list_view.SetCanFocus(false)

	list_view.ConnectActivate(func(pos uint) {
		curr_track := p.Queue[pos]
		p.CurrentIdx = int(pos)
		selection.SetSelected(pos)
		onChange(curr_track)
	})

	select_refresh := func(_ player.Track) {
		if len(p.Queue) == 0 || p.CurrentIdx < 0 || p.CurrentIdx >= len(p.Queue) {
			return
		}
		selection.SetSelected(uint(p.CurrentIdx))
		list_view.ScrollTo(uint(p.CurrentIdx), gtk.ListScrollSelect, nil)
	}

	queue_refresh := func() {
		indices := make([]string, len(p.Queue))
		for i, t := range p.Queue {
			indices[i] = t.Title
		}
		model.Splice(0, uint(model.NItems()), indices)

		if len(p.Queue) > 0 && p.CurrentIdx >= 0 && p.CurrentIdx < len(p.Queue) {
			selection.SetSelected(uint(p.CurrentIdx))
			list_view.ScrollTo(uint(p.CurrentIdx), gtk.ListScrollSelect, nil)
		}
	}
	p.Subscribe(player.OnTrackChange, select_refresh)
	p.Subscribe(player.QueueChange, queue_refresh)

	scrollable := gtk.NewScrolledWindow()
	scrollable.SetChild(list_view)
	return scrollable
}
