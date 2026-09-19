package ui

import (
	"github.com/charmbracelet/log"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	_ "embed"
)

//go:embed app.css
var css string

func load_css() {
	provider := gtk.NewCSSProvider()
	provider.LoadFromBytes(glib.NewBytes([]byte(css)))

	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(),
		provider,
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)
}

func NewPaned() *gtk.Paned {
	return gtk.NewPaned(gtk.OrientationHorizontal)
}

func NewWindow(onActivate func(win *gtk.ApplicationWindow)) *gtk.Application {
	app := gtk.NewApplication("com.KotonBads.Mosaic", gio.ApplicationDefaultFlags)
	app.ConnectActivate(func() {
		log.Info("Activating GTK application window", "appID", "com.KotonBads.Mosaic")

		load_css()
		win := gtk.NewApplicationWindow(app)
		win.SetTitle("Mosaic")
		win.SetDefaultSize(1280, 720)

		onActivate(win)

		win.Present()
		log.Debug("Main application window presented")
	})
	return app
}
