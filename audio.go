package main

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

func newAudioPlayer(mp3Path string, loop int) (*audioPlayer, error) {
	fd, err := os.Open(mp3Path)
	if err != nil {
		return nil, err
	}
	seeker, format, err := mp3.Decode(fd)
	if err != nil {
		return nil, err
	}
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/30))
	volumeChanger := &effects.Volume{Streamer: beep.Loop(loop, seeker), Base: 2}
	ctrl := &beep.Ctrl{Streamer: volumeChanger}
	return &audioPlayer{
		fd:               fd,
		SampleRate:       format.SampleRate,
		StreamSeekCloser: seeker,
		Ctrl:             ctrl,
		volumer:          volumeChanger,
	}, nil
}

type audioPlayer struct {
	beep.StreamSeekCloser
	*beep.Ctrl
	beep.SampleRate
	volumer *effects.Volume
	fd      *os.File
	program *tea.Program // set after program is created
}

func (ap *audioPlayer) play() {
	speaker.Play(beep.Seq(ap.Ctrl, beep.Callback(func() {
		ap.fd.Close()
		if ap.program != nil {
			ap.program.Send(finishedMsg{})
		}
	})))
}

func (ap *audioPlayer) positionD() time.Duration {
	return ap.SampleRate.D(ap.Position())
}

func (ap *audioPlayer) volume() int {
	return int(ap.volumer.Volume * 10)
}

func (ap *audioPlayer) seek(factor int) {
	speaker.Lock()
	defer speaker.Unlock()
	newPos := ap.Position()
	secs := time.Duration(abs(factor)) * time.Second
	if factor < 0 {
		newPos -= ap.N(secs)
	} else {
		newPos += ap.N(secs)
	}
	newPos = clamp(newPos, 0, ap.Len()-1)
	if err := ap.Seek(newPos); err != nil {
		panic("Seek: " + err.Error())
	}
}

func (ap *audioPlayer) togglePause() {
	speaker.Lock()
	ap.Paused = !ap.Paused
	speaker.Unlock()
}

func (ap *audioPlayer) changeValume(factor int) {
	speaker.Lock()
	ap.volumer.Volume += float64(factor) / 10
	speaker.Unlock()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
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

