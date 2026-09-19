package main

import (
	"os"
	"time"

	"github.com/KotonBads/mosaic/ui"
	"github.com/charmbracelet/log"
)

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
		Prefix:          "mosaic",
		Level:           log.DebugLevel,
	})
	log.SetDefault(logger)

	log.Info("Starting Mosaic Music Player", "version", "0.1.0")

	ui.App()
}
