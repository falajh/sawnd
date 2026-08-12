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
	program    *tea.Program

	mu        sync.Mutex
	position  time.Duration
	length    time.Duration
	paused    bool
	volumePct int

	status *StatusBroadcaster
}

func newAudioPlayer(filePath string, loop int) (*audioPlayer, error) {
	if _, err := exec.LookPath("mpv"); err != nil {
		return nil, fmt.Errorf("mpv not found in PATH: install it (e.g. `sudo apt install mpv` / `brew install mpv`)")
	}

	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("sawnd-%d.sock", os.Getpid()))

	args := []string{
		"--no-load-scripts",
		"--no-video",
		"--idle=yes",
		"--pause=yes",
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
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	conn := mpvipc.NewConnection(socketPath)

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
		status:     NewStatusBroadcaster(),
	}, nil
}

// Subscribe lets any module receive a live stream of playback status
// updates without audioPlayer needing to know who's listening.
func (ap *audioPlayer) Subscribe() <-chan PlayerStatus {
	return ap.status.Subscribe()
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
				ap.mu.Lock()
				s := PlayerStatus{
					State:     StateStopped,
					Position:  ap.position,
					Length:    ap.length,
					VolumePct: ap.volumePct,
				}
				ap.mu.Unlock()
				ap.status.publish(s)

				if ap.program != nil {
					ap.program.Send(finishedMsg{})
				}
				stop <- struct{}{}
				return
			}
		}
	}()
}

// pollPosition is the single source of truth for mpv's live state — the
// only goroutine that talks to mpv for time-pos/duration/pause/volume. It
// caches the values (for the TUI's synchronous getters) and publishes them
// to status (for MPRIS, lyrics sync, or anything else) on every tick.
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
		volume, volErr := ap.conn.Get("volume")

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
		if volErr == nil {
			if v, ok := volume.(float64); ok {
				ap.volumePct = int(v)
			}
		}

		state := StatePlaying
		if ap.paused {
			state = StatePaused
		}
		s := PlayerStatus{
			State:     state,
			Position:  ap.position,
			Length:    ap.length,
			VolumePct: ap.volumePct,
		}
		ap.mu.Unlock()

		ap.status.publish(s)
	}
}

func (ap *audioPlayer) close() {
	if ap.status != nil {
		ap.status.closeAll()
	}
	if ap.conn != nil && !ap.conn.IsClosed() {
		ap.conn.Call("quit")
		ap.conn.Close()
	}
	if ap.cmd != nil && ap.cmd.Process != nil {
		ap.cmd.Process.Kill()
		ap.cmd.Wait()
	}
	os.Remove(ap.socketPath)
}

func (ap *audioPlayer) Position() time.Duration {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.position
}

func (ap *audioPlayer) Length() time.Duration {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.length
}

func (ap *audioPlayer) volume() int {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.volumePct
}

func (ap *audioPlayer) seek(seconds int) {
	ap.conn.Call("seek", float64(seconds), "relative")
}

func (ap *audioPlayer) togglePause() {
	ap.conn.Call("cycle", "pause")
}

func (ap *audioPlayer) changeValume(factor int) {
	ap.mu.Lock()
	newVol := clamp(ap.volumePct+factor*5, 0, 100)
	ap.volumePct = newVol
	ap.mu.Unlock()

	ap.conn.Set("volume", newVol)
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
