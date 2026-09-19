package ui

import (
	"strings"

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

func PlayerControls() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 12)

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
	if err != nil {
		logger.Warn("Failed to load album thumbnail", "track", track.Title, "err", err)
	}

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
