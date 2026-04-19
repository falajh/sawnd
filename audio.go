package main

import (
	"os"
	"time"

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
	volumer  *effects.Volume
	fd       *os.File
	done     float64
	finished bool
}

func (ap *audioPlayer) play() {
	speaker.Play(beep.Seq(ap, beep.Callback(func() { ap.finished = true; ap.fd.Close() })))
}

func (ap *audioPlayer) Stream(samples [][2]float64) (n int, ok bool) {
	return ap.Ctrl.Stream(samples)
}

func (ap *audioPlayer) Err() error {
	return ap.Ctrl.Err()
}

func (ap *audioPlayer) volume() int {
	return int(ap.volumer.Volume * 10)
}

func (ap *audioPlayer) seek(factor int) {
	speaker.Lock()
	newPos := ap.Position()
	if factor < 1 {
		factor *= -1
		newPos -= ap.N(time.Duration(factor) * time.Second)
	} else {
		newPos += ap.N(time.Duration(factor) * time.Second)
	}

	if newPos < 0 {
		newPos = 0
	}

	if newPos >= ap.Len() {
		newPos = ap.Len() - 1
	}

	if err := ap.Seek(newPos); err != nil {
		panic("Seek: " + err.Error())
	}

	speaker.Unlock()
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
