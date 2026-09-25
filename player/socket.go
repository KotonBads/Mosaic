package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// StartMPV launches an isolated mpv process with an IPC socket server
func StartMPV(socketPath string) (*Client, error) {
	logger.Debug("Preparing MPV socket", "socketPath", socketPath)
	_ = os.Remove(socketPath)

	cmd := exec.Command("mpv",
		"--idle=yes",
		"--no-video",
		"--no-config",
		"--load-scripts=no",
		"--input-default-bindings=no",
		"--input-terminal=no",
		"--terminal=no",
		"--input-ipc-server="+socketPath,
	)

	if err := cmd.Start(); err != nil {
		logger.Error("Failed to spawn mpv process", "err", err)
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	var conn net.Conn
	var err error
	for i := 0; i < 20; i++ {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		logger.Error("Cannot connect to mpv socket", "socketPath", socketPath, "err", err)
		return nil, fmt.Errorf("cannot connect to mpv socket: %w", err)
	}

	client := &Client{
		conn:       conn,
		reader:     bufio.NewReader(conn),
		socketPath: socketPath,
	}

	logger.Info("Connected to MPV IPC socket", "socketPath", socketPath)

	go client.listenLoop()

	// Observe fundamental playback properties
	_ = client.ObserveProperty(1, "time-pos")
	_ = client.ObserveProperty(2, "pause")
	_ = client.ObserveProperty(3, "eof-reached")

	return client, nil
}

// SendCommand sends a JSON array command to mpv over the socket
func (c *Client) SendCommand(args ...interface{}) error {
	id := c.reqID.Add(1)
	req := map[string]interface{}{
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
	logger.Info("Playing track", "path", path)
	return c.SendCommand("loadfile", path, "replace")
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
			Event string      `json:"event"`
			Name  string      `json:"name"`
			Data  interface{} `json:"data"`
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
				c.OnTrackEnd()
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
