package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AvengeMedia/dankgo/lyrics"
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
	vol_box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	final_box := gtk.NewBox(gtk.OrientationVertical, 4)
	is_scrubbing := false

	play_pause := gtk.NewButtonFromIconName("media-playback-pause-symbolic")
	previous := gtk.NewButtonFromIconName("media-skip-backward-symbolic")
	next := gtk.NewButtonFromIconName("media-skip-forward-symbolic")
	shuffle := gtk.NewButtonFromIconName("media-playlist-shuffle-symbolic")
	repeat := gtk.NewButtonFromIconName("media-playlist-repeat-symbolic")
	seek_bar := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 100, 1)
	pos_cur := gtk.NewLabel(format_time(p.Pos))
	pos_cur.AddCSSClass("numeric")
	pos_max := gtk.NewLabel(format_time(p.Queue[p.CurrentIdx].Duration))
	pos_max.AddCSSClass("numeric")
	volume := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 100, 1)
	vol_cur := gtk.NewLabel(fmt.Sprint(p.Volume))
	vol_cur.AddCSSClass("numeric")
	vol_cur.SetWidthChars(3)
	vol_max := gtk.NewLabel("100")
	vol_max.AddCSSClass("numeric")
	vol_max.SetWidthChars(3)

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

	controls_box.Append(shuffle)
	controls_box.Append(previous)
	controls_box.Append(play_pause)
	controls_box.Append(next)
	controls_box.Append(repeat)
	seek_box.Append(pos_cur)
	seek_box.Append(seek_bar)
	seek_box.Append(pos_max)
	vol_box.Append(vol_cur)
	vol_box.Append(volume)
	vol_box.Append(vol_max)
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

		vol_cur.SetText(fmt.Sprint(p.Volume))
		volume.SetValue(float64(p.Volume))
		pos_cur.SetText(format_time(p.Pos))
		pos_max.SetText(format_time(p.Queue[p.CurrentIdx].Duration))
	}

	if p != nil {
		click := gtk.NewGestureClick()

		// this is a workaround to have release events
		// for the seek bar
		// https://gitlab.gnome.org/GNOME/gtk/-/work_items/4939#note_1680234
		seek_controllers := seek_bar.ObserveControllers()
		for i := range seek_controllers.NItems() {
			g, ok := seek_controllers.Item(i).Cast().(*gtk.GestureClick)
			if ok && g != nil {
				click = g
				break
			}
		}

		play_pause.ConnectClicked(p.PlayPause)
		repeat.ConnectClicked(p.SetRepeat)
		shuffle.ConnectClicked(p.SetShuffle)
		next.ConnectClicked(p.Next)
		previous.ConnectClicked(p.Prev)

		seek_bar.ConnectChangeValue(func(scroll gtk.ScrollType, value float64) (ok bool) {
			track := p.Queue[p.CurrentIdx]
			ratio := value / 100
			pos_cur.SetText(format_time(time.Duration(ratio * float64(track.Duration))))
			return false
		})
		click.ConnectPressed(func(nPress int, x, y float64) {
			logger.Info("pressed, n press", "n", nPress)
			is_scrubbing = true
		})
		click.ConnectReleased(func(nPress int, x, y float64) {
			logger.Info("released, n press", "n", nPress)

			track := p.Queue[p.CurrentIdx]
			ratio := seek_bar.Value() / 100
			p.Seek(time.Duration(ratio * float64(track.Duration)))
			is_scrubbing = false
		})
		p.MPV.OnTimePos = func(pos float64) {
			if is_scrubbing {
				logger.Info("ignoring time pos", "pos", pos)
				return
			}
			track := p.Queue[p.CurrentIdx]
			p.Pos = time.Duration(pos * float64(time.Second))
			seek_bar.SetValue(float64(pos) * 100 / track.Duration.Seconds())
			glib.IdleAdd(refresh)
		}

		volume.ConnectValueChanged(func() {
			p.SetVolume(int(volume.Value()))
			vol_cur.SetText(fmt.Sprint(int(volume.Value())))
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
		p.CurrentIdx = int(pos)
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

func Lyrics(p *player.Player) gtk.Widgetter {
	cache_dir, _ := os.UserCacheDir()
	client := lyrics.New(lyrics.Options{
		CacheDir: filepath.Join(cache_dir, "mosaic", "lyrics"),
	})

	track := p.Queue[p.CurrentIdx]
	artists := make([]string, len(track.Artists))
	for i, a := range track.Artists {
		artists[i] = a.Name
	}
	request := lyrics.Request{
		Title:    track.Title,
		Artist:   strings.Join(artists, ", "),
		Album:    track.Album.Title,
		Duration: track.Duration,
	}
	result, err := client.Lookup(context.TODO(), request)
	if err != nil {
		return gtk.NewLabel("lyrics")
	}

	scrolled := gtk.NewScrolledWindow()
	scrolled.SetChild(gtk.NewLabel(result.Lyrics.Plain))
	return scrolled
}
