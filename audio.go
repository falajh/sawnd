package main

import (
	"os"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

func newAudioPlayer(mp3Path string, loop string) (*audioPlayer, error) {
	fd, err := os.Open(mp3Path)
	if err != nil {
		return nil, err
	}

	seeker, format, err := mp3.Decode(fd)
	if err != nil {
		return nil, err
	}

	if loop == "" {
		loop = "1"
	}

	loopN, err := strconv.Atoi(loop)
	if err != nil {
		return nil, err
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/30))
	volumeChanger := &effects.Volume{Streamer: beep.Loop(loopN, seeker), Base: 2}
	ctrl := &beep.Ctrl{Streamer: volumeChanger}

	return &audioPlayer{
		fd:         fd,
		sampleRate: format.SampleRate,
		seeker:     seeker,
		streamer:   ctrl,
		volumer:    volumeChanger,
	}, nil

}

type audioPlayer struct {
	seeker     beep.StreamSeekCloser
	sampleRate beep.SampleRate
	streamer   *beep.Ctrl
	volumer    *effects.Volume
	fd         *os.File
	done       float64
	finished   bool
}

func (ap *audioPlayer) play() {
	speaker.Play(beep.Seq(ap, beep.Callback(func() { ap.finished = true; ap.fd.Close() })))
}

func (ap *audioPlayer) Stream(samples [][2]float64) (n int, ok bool) {
	return ap.streamer.Stream(samples)
}

func (ap *audioPlayer) Err() error {
	return ap.streamer.Err()
}

func (ap *audioPlayer) position() int {
	return ap.seeker.Position()
}

func (ap *audioPlayer) positionD() time.Duration {
	return ap.sampleRate.D(ap.position())
}

func (ap *audioPlayer) len() int {
	return ap.seeker.Len()
}

func (ap *audioPlayer) lenD() time.Duration {
	return ap.sampleRate.D(ap.seeker.Len())
}

func (ap *audioPlayer) paused() bool {
	return ap.streamer.Paused
}

func (ap *audioPlayer) volume() int {
	return int(ap.volumer.Volume * 10)
}

func (ap *audioPlayer) seek(factor int) {
	speaker.Lock()
	newPos := ap.position()
	if factor < 1 {
		factor *= -1
		newPos -= ap.sampleRate.N(time.Duration(factor) * time.Second)
	} else {
		newPos += ap.sampleRate.N(time.Duration(factor) * time.Second)
	}

	if newPos < 0 {
		newPos = 0
	}

	if newPos >= ap.len() {
		newPos = ap.len() - 1
	}

	if err := ap.seeker.Seek(newPos); err != nil {
		panic("Seek: " + err.Error())
	}

	speaker.Unlock()
}

func (ap *audioPlayer) togglePause() {
	speaker.Lock()
	ap.streamer.Paused = !ap.streamer.Paused
	speaker.Unlock()
}

func (ap *audioPlayer) changeValume(factor int) {
	speaker.Lock()
	ap.volumer.Volume += float64(factor) / 10
	speaker.Unlock()
}
