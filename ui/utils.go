package ui

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"context"

	"github.com/KotonBads/mosaic/player"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"go.senan.xyz/taglib"
)

func PictureFromBytes(data []byte) (*gtk.Picture, error) {
	tex, err := gdk.NewTextureFromBytes(glib.NewBytes(data))
	if err != nil {
		return nil, err
	}

	picture := gtk.NewPictureForPaintable(tex)
	picture.SetCanShrink(true)
	return picture, nil
}

func ThumbnailFromBytes(data []byte) (*gtk.Picture, error) {
	pixbuf, err := gdkpixbuf.NewPixbufFromStreamAtScale(
		context.Background(),
		gio.NewMemoryInputStreamFromBytes(glib.NewBytes(data)),
		42,
		42,
		false,
	)
	if err != nil {
		return nil, err
	}

	tex := gdk.NewTextureForPixbuf(pixbuf)
	picture := gtk.NewPictureForPaintable(tex)
	picture.SetCanShrink(true)
	picture.SetContentFit(gtk.ContentFitContain)
	return picture, nil
}

func read_image_from_tag(track player.Track) ([]byte, error) {
	img_bytes, err := taglib.ReadImage(track.Path)
	if err != nil {
		return nil, err
	}
	return img_bytes, nil
}

func join_artist_name(artists []player.Artist) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		names = append(names, artist.Name)
	}
	return strings.Join(names, ", ")
}

func sanitize_name(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "/", "_"))
}

func format_time(t time.Duration) string {
	totalSecs := int64(t.Round(time.Second).Seconds())
	if totalSecs < 0 {
		totalSecs = 0
	}

	h := totalSecs / 3600
	m := (totalSecs % 3600) / 60
	s := totalSecs % 60

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func GetAlbumArt(track player.Track) (*gtk.Picture, error) {
	cache_dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}

	art_dir := cache_dir + "/mosaic/art"
	err = os.MkdirAll(art_dir, 0755)
	if err != nil {
		return nil, err
	}

	// sanitize everything
	// thanks fall out boy
	artists := sanitize_name(join_artist_name(track.Artists))
	art_path := fmt.Sprintf("%s/%s - %s.png", art_dir, artists, sanitize_name(track.Album.Title))

	_, err = os.Stat(art_path)
	if errors.Is(err, os.ErrNotExist) {
		img_bytes, err := read_image_from_tag(track)
		if err != nil {
			logger.Warn("Could not read image from tag", "err", err)
			return nil, err
		}
		picture, err := PictureFromBytes(img_bytes)
		if err != nil {
			logger.Warn("Could not create album art", "err", err)
			return nil, err
		}

		texture, ok := picture.Paintable().Cast().(gdk.Texturer)
		if ok {
			logger.Debug("Saving album art", "art_path", art_path)
			gdk.BaseTexture(texture).SaveToPNG(art_path)
		}

		return picture, nil
	}

	picture := gtk.NewPictureForFilename(art_path)
	picture.SetCanShrink(true)
	return picture, nil
}

func GetAlbumThumb(track player.Track) (*gtk.Picture, error) {
	cache_dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}

	thumb_dir := cache_dir + "/mosaic/thumb"
	err = os.MkdirAll(thumb_dir, 0755)
	if err != nil {
		return nil, err
	}

	artists := sanitize_name(join_artist_name(track.Artists))
	thumb_path := fmt.Sprintf("%s/%s - %s.png", thumb_dir, artists, sanitize_name(track.Album.Title))

	_, err = os.Stat(thumb_path)
	if errors.Is(err, os.ErrNotExist) {
		img_bytes, err := read_image_from_tag(track)
		if err != nil {
			return nil, err
		}
		picture, err := ThumbnailFromBytes(img_bytes)
		if err != nil {
			return nil, err
		}

		texture, ok := picture.Paintable().Cast().(gdk.Texturer)
		if ok {
			gdk.BaseTexture(texture).SaveToPNG(thumb_path)
		}

		return picture, nil
	}

	// trust that the cached thumbnails are already to scale
	// if they're not, well it's in cache so it will be refreshed
	// at some point
	picture := gtk.NewPictureForFilename(thumb_path)
	picture.SetCanShrink(true)
	picture.SetContentFit(gtk.ContentFitContain)
	return picture, nil
}

// GetBlurredAmbientArt implements dual-filtering / pyramid downsampling and upsampling
// (Maki/Kawase blur approximation) using intermediate mip buffers.
// Successively downscaling and upscaling with bilinear filtering eliminates blocky pixelation
// and banding while generating a beautifully smooth, high-radius diffuse blur.
func GetBlurredAmbientArt(track player.Track) (*gdk.Texture, error) {
	cache_dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}

	artists := sanitize_name(join_artist_name(track.Artists))
	art_path := fmt.Sprintf("%s/mosaic/art/%s - %s.png", cache_dir, artists, sanitize_name(track.Album.Title))

	// Ensure full art exists in cache
	if _, err := os.Stat(art_path); errors.Is(err, os.ErrNotExist) {
		if _, err := GetAlbumArt(track); err != nil {
			return nil, err
		}
	}

	current, err := gdkpixbuf.NewPixbufFromFile(art_path)
	if err != nil {
		return nil, err
	}

	// Downsample pyramid: progressively halve the resolution to smoothly filter high frequencies
	downSteps := []int{256, 128, 64, 32, 16}
	for _, size := range downSteps {
		if next := current.ScaleSimple(size, size, gdkpixbuf.InterpBilinear); next != nil {
			current = next
		}
	}

	// Upsample pyramid: step back up through intermediate resolutions with bilinear filtering
	// to smoothly diffuse the low-frequency color gradients without nearest-neighbor grid artifacts
	upSteps := []int{32, 64, 128, 256}
	for _, size := range upSteps {
		if next := current.ScaleSimple(size, size, gdkpixbuf.InterpBilinear); next != nil {
			current = next
		}
	}

	return gdk.NewTextureForPixbuf(current), nil
}
