package ui

import (
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

func PlayerControls(p *player.Player) gtk.Widgetter {
	controls_box := gtk.NewBox(gtk.OrientationHorizontal, 12)
	seek_box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	final_box := gtk.NewBox(gtk.OrientationVertical, 4)

	play_pause := gtk.NewButtonFromIconName("media-playback-pause-symbolic")
	previous := gtk.NewButtonFromIconName("media-skip-backward-symbolic")
	next := gtk.NewButtonFromIconName("media-skip-forward-symbolic")
	shuffle := gtk.NewButtonFromIconName("media-playlist-shuffle-symbolic")
	repeat := gtk.NewButtonFromIconName("media-playlist-repeat-symbolic")
	seek_bar := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 100, 1)
	pos_cur := gtk.NewLabel(format_time(p.Pos))
	pos_max := gtk.NewLabel(format_time(p.Queue[p.CurrentIdx].Duration))

	controls_box.SetHAlign(gtk.AlignCenter)
	seek_box.SetHAlign(gtk.AlignFill)
	seek_box.SetHExpand(true)
	seek_bar.SetHExpand(true)
	seek_bar.SetDrawValue(false)
	final_box.SetHAlign(gtk.AlignFill)
	final_box.SetHExpand(true)

	controls_box.Append(shuffle)
	controls_box.Append(previous)
	controls_box.Append(play_pause)
	controls_box.Append(next)
	controls_box.Append(repeat)
	seek_box.Append(pos_cur)
	seek_box.Append(seek_bar)
	seek_box.Append(pos_max)
	final_box.Append(seek_box)
	final_box.Append(controls_box)

	var updatingUI bool

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
		} else {
			shuffle.SetIconName("media-playlist-shuffle-symbolic")
			shuffle.AddCSSClass("dim-label")
		}

		if p.Repeat == player.RepeatOne {
			repeat.SetIconName("media-playlist-repeat-song-symbolic")
			repeat.RemoveCSSClass("dim-label")
		} else if p.Repeat == player.RepeatQueue {
			repeat.SetIconName("media-playlist-repeat-symbolic")
			repeat.RemoveCSSClass("dim-label")
		} else {
			repeat.SetIconName("media-playlist-repeat-symbolic")
			repeat.AddCSSClass("dim-label")
		}

		pos_cur.SetText(format_time(p.Pos))
		pos_max.SetText(format_time(p.Queue[p.CurrentIdx].Duration))
	}

	if p != nil {
		play_pause.ConnectClicked(p.PlayPause)
		repeat.ConnectClicked(p.SetRepeat)
		shuffle.ConnectClicked(p.SetShuffle)
		next.ConnectClicked(p.Next)
		previous.ConnectClicked(p.Prev)
		seek_bar.ConnectValueChanged(func() {
			if updatingUI {
				return
			}
			if len(p.Queue) == 0 || p.CurrentIdx < 0 || p.CurrentIdx >= len(p.Queue) {
				return
			}
			track := p.Queue[p.CurrentIdx]
			ratio := seek_bar.Value() / 100.0
			p.Seek(time.Duration(ratio * float64(track.Duration)))
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

	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(obj *glib.Object) {
		list_item := obj.Cast().(*gtk.ListItem)
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

	list_view.ConnectActivate(func(pos uint) {
		curr_track := p.Queue[pos]
		onChange(curr_track)
	})

	select_refresh := func(_ player.Track) {
		list_view.Model().SelectItem(uint(p.CurrentIdx), true)
		list_view.ScrollTo(uint(p.CurrentIdx), gtk.ListScrollFocus, nil)
	}

	queue_refresh := func() {
		indices := make([]string, len(p.Queue))
		for i, t := range p.Queue {
			indices[i] = t.Title
		}
		model.Splice(0, uint(model.NItems()), indices)

		list_view.Model().SelectItem(uint(p.CurrentIdx), true)
		list_view.ScrollTo(uint(p.CurrentIdx), gtk.ListScrollFocus, nil)
	}
	p.Subscribe(player.OnTrackChange, select_refresh)
	p.Subscribe(player.QueueChange, queue_refresh)

	scrollable := gtk.NewScrolledWindow()
	scrollable.SetChild(list_view)
	return scrollable
}
