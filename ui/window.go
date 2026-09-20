package ui

import (
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
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

func NewWindow(onActivate func(win *adw.ApplicationWindow)) *adw.Application {
	app := adw.NewApplication("com.KotonBads.Mosaic", gio.ApplicationDefaultFlags)
	app.ConnectActivate(func() {
		logger.Info("Activating Adwaita application window", "appID", "com.KotonBads.Mosaic")

		load_css()
		win := adw.NewApplicationWindow(&app.Application)
		win.SetTitle("Mosaic")
		win.SetDefaultSize(1280, 720)

		onActivate(win)

		win.Present()
		logger.Debug("Main application window presented")
	})
	return app
}
