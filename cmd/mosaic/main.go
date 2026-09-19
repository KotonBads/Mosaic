package main

import (
	"time"

	"github.com/KotonBads/mosaic/ui"
	"github.com/charmbracelet/log"
)

func main() {
	log.SetTimeFormat(time.Kitchen)
	log.SetReportCaller(false)
	log.Info("Starting Mosaic Music Player", "version", "0.1.0")

	ui.App()
}
