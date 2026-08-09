package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dexterlb/mpvipc"
)

type audioPlayer struct {
	conn       *mpvipc.Connection
	cmd        *exec.Cmd
	socketPath string
	program    *tea.Program // set after program is created

	mu        sync.Mutex
	position  time.Duration
	length    time.Duration
	paused    bool
	volumePct int
}

func newAudioPlayer(filePath string, loop int) (*audioPlayer, error) {
	if _, err := exec.LookPath("mpv"); err != nil {
		return nil, fmt.Errorf("mpv not found in PATH: install it (e.g. `sudo apt install mpv` / `brew install mpv`)")
	}

	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("sawnd-%d.sock", os.Getpid()))

	args := []string{
		"--no-video",
		"--idle=yes",
		"--pause=yes", // start paused; play() unpauses once the TUI is wired up
		"--input-ipc-server=" + socketPath,
	}
	switch {
	case loop == -1:
		args = append(args, "--loop-file=inf")
	case loop > 1:
		args = append(args, fmt.Sprintf("--loop-file=%d", loop-1))
	}
	args = append(args, filePath)

	cmd := exec.Command("mpv", args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	conn := mpvipc.NewConnection(socketPath)

	// mpv creates the IPC socket asynchronously, so retry briefly.
	var connErr error
	for range 100 {
		if connErr = conn.Open(); connErr == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if connErr != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("failed to connect to mpv over IPC: %w", connErr)
	}

	volumePct := 100
	if v, err := conn.Get("volume"); err == nil {
		if f, ok := v.(float64); ok {
			volumePct = int(f)
		}
	}

	return &audioPlayer{
		conn:       conn,
		cmd:        cmd,
		socketPath: socketPath,
		paused:     true,
		volumePct:  volumePct,
	}, nil
}

func (ap *audioPlayer) play() {
	ap.conn.Set("pause", false)
	ap.mu.Lock()
	ap.paused = false
	ap.mu.Unlock()

	go ap.pollPosition()

	events, stop := ap.conn.NewEventListener()
	go func() {
		for event := range events {
			if event.Name == "end-file" {
				if ap.program != nil {
					ap.program.Send(finishedMsg{})
				}
				stop <- struct{}{}
				return
			}
		}
	}()
}

// close stops mpv and cleans up the IPC socket. Call this after the
// Bubble Tea program exits.
func (ap *audioPlayer) close() {
	if ap.conn != nil && !ap.conn.IsClosed() {
		_, _ = ap.conn.Call("quit")
		_ = ap.conn.Close()
	}
	if ap.cmd != nil && ap.cmd.Process != nil {
		_ = ap.cmd.Process.Kill()
		_ = ap.cmd.Wait()
	}
	_ = os.Remove(ap.socketPath)
}

func (ap *audioPlayer) Position() time.Duration {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.position
}

// pollPosition is the ONLY goroutine that talks to mpv for time-pos/duration.
// Everything else reads the cached values via Position()/Length().
func (ap *audioPlayer) pollPosition() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if ap.conn.IsClosed() {
			return
		}

		pos, posErr := ap.conn.Get("time-pos")
		length, lenErr := ap.conn.Get("duration")
		paused, pauseErr := ap.conn.Get("pause")

		ap.mu.Lock()
		if posErr == nil {
			if v, ok := pos.(float64); ok {
				ap.position = time.Duration(v * float64(time.Second))
			}
		}
		if lenErr == nil {
			if v, ok := length.(float64); ok {
				ap.length = time.Duration(v * float64(time.Second))
			}
		}
		if pauseErr == nil {
			if v, ok := paused.(bool); ok {
				ap.paused = v
			}
		}
		ap.mu.Unlock()
	}
}

func (ap *audioPlayer) Length() time.Duration {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.length
}

func (ap *audioPlayer) volume() int {
	return ap.volumePct
}

func (ap *audioPlayer) seek(seconds int) {
	_, _ = ap.conn.Call("seek", float64(seconds), "relative")
}

func (ap *audioPlayer) togglePause() {
	_, _ = ap.conn.Call("cycle", "pause")
}

func (ap *audioPlayer) changeValume(factor int) {
	ap.volumePct = clamp(ap.volumePct+factor*5, 0, 100)
	_ = ap.conn.Set("volume", ap.volumePct)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
