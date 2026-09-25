package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

var SOCKET_DIR = os.Getenv("XDG_RUNTIME_DIR")

func (c *Client) Init() error {
	logger.SetLevel(log.InfoLevel)

	if SOCKET_DIR == "" {
		c.socketPath = filepath.Join("/tmp", "mosaic.sock")
	}
	c.socketPath = filepath.Join(SOCKET_DIR, "mosaic.sock")

	_ = os.Remove(c.socketPath)

	cmd := exec.Command("mpv",
		"--idle=yes",
		"--no-video",
		"--no-config",
		"--load-scripts=no",
		"--input-default-bindings=no",
		"--input-terminal=no",
		"--terminal=no",
		"--input-ipc-server="+c.socketPath,
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start mpv: %w", err)
	}

	for i := range 20 {
		conn, err := net.Dial("unix", c.socketPath)
		if err != nil {
			logger.Warn("Failed to connect, retrying...", "attempt", i+1, "err", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		c.conn = conn
		c.reader = bufio.NewReader(conn)
		logger.Info("Connected to mpv", "attempt", i+1)
		break
	}

	go c.listenLoop()
	c.ObserveProperty(1, "time-pos")
	c.ObserveProperty(2, "pause")

	return nil
}

func (c *Client) SendCommand(args ...any) error {
	id := c.reqID.Add(1)
	req := map[string]any{
		"command":    args,
		"request_id": id,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = c.conn.Write(data)
	if err != nil {
		logger.Error("Failed writing command to mpv", "err", err, "args", args)
	}
	return err
}

func (c *Client) PlayFile(path string) error {
	logger.Debug("Playing track", "path", path)
	return c.SendCommand("loadfile", path, "replace")
}

func (c *Client) Play() error {
	logger.Debug("Play")
	return c.SendCommand("set-property", "pause", true)
}

func (c *Client) Pause() error {
	logger.Debug("Pause")
	return c.SendCommand("set-property", "pause", false)
}

func (c *Client) TogglePause() error {
	logger.Debug("Toggling playback pause")
	return c.SendCommand("cycle", "pause")
}

func (c *Client) Seek(seconds float64) error {
	logger.Debug("Seeking track", "seconds", seconds)
	return c.SendCommand("seek", seconds, "absolute")
}

func (c *Client) SetVolume(volume float64) error {
	logger.Debug("Setting volume", "volume", volume)
	return c.SendCommand("set_property", "volume", volume)
}

func (c *Client) ObserveProperty(id int, name string) error {
	logger.Debug("Subscribing to mpv property", "id", id, "property", name)
	return c.SendCommand("observe_property", id, name)
}

func (c *Client) listenLoop() {
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			logger.Warn("MPV socket connection closed", "err", err)
			return
		}

		var event struct {
			Event  string `json:"event"`
			Name   string `json:"name"`
			Data   any    `json:"data"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if event.Event == "property-change" {
			switch event.Name {
			case "time-pos":
				if sec, ok := event.Data.(float64); ok && c.OnTimePos != nil {
					coreglib.IdleAdd(func() {
						c.OnTimePos(sec)
					})
				}
			case "pause":
				if paused, ok := event.Data.(bool); ok && c.OnPause != nil {
					logger.Debug("Playback pause state changed", "paused", paused)
					coreglib.IdleAdd(func() {
						c.OnPause(paused)
					})
				}
			}
		} else if event.Event == "end-file" && c.OnTrackEnd != nil {
			logger.Info("Track finished playing")
			coreglib.IdleAdd(func() {
				c.OnTrackEnd(event.Reason)
			})
		}
	}
}

// Close gracefully closes the socket connection and cleans up
func (c *Client) Close() {
	logger.Info("Shutting down MPV client")
	_ = c.SendCommand("quit")
	if c.conn != nil {
		_ = c.conn.Close()
	}
	_ = os.Remove(c.socketPath)
}
