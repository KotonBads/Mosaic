package ui

import (
	"strings"
	"time"

	"github.com/KotonBads/mosaic/player"
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
	box := gtk.NewBox(gtk.OrientationHorizontal, 12)

	play_pause := gtk.NewButtonFromIconName("media-playback-pause-symbolic")
	previous := gtk.NewButtonFromIconName("media-skip-backward-symbolic")
	next := gtk.NewButtonFromIconName("media-skip-forward-symbolic")
	shuffle := gtk.NewButtonFromIconName("media-playlist-shuffle-symbolic")
	repeat := gtk.NewButtonFromIconName("media-playlist-repeat-symbolic")
	seek_bar := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 100, 1)

	seek_bar.SetHExpand(true)
	seek_bar.SetDrawValue(false)

	box.Append(shuffle)
	box.Append(previous)
	box.Append(play_pause)
	box.Append(next)
	box.Append(repeat)
	box.Append(seek_bar)

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

		if len(p.Queue) > 0 && p.CurrentIdx >= 0 && p.CurrentIdx < len(p.Queue) {
			duration := p.Queue[p.CurrentIdx].Duration
			if duration > 0 {
				updatingUI = true
				percent := (p.Pos.Seconds() / duration.Seconds()) * 100.0
				seek_bar.SetValue(percent)
				updatingUI = false
			}
		}
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

		p.OnChange = refresh
	}

	refresh()

	return box
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

func Queue(p player.Player) gtk.Widgetter {
	
	
	return nil
}
