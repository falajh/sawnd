package audio

import (
	"os"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type Player struct {
	V          int
	Done       float64
	Position   time.Duration
	Finished   bool
	Total      time.Duration
	P          bool
	sampleRate beep.SampleRate
	streamer   beep.StreamSeeker
	ctrl       *beep.Ctrl
	volume     *effects.Volume
	fd         *os.File
}

func (p *Player) Play() {
	speaker.Play(beep.Seq(p.volume, beep.Callback(func() { p.Finished = true; p.fd.Close() })))
}

func (p *Player) Seek(factor int) {
	speaker.Lock()
	newPos := p.streamer.Position()
	if factor < 1 {
		factor *= -1
		newPos -= p.sampleRate.N(time.Duration(factor) * time.Second)
	}
	if factor > 1 {
		newPos += p.sampleRate.N(time.Duration(factor) * time.Second)
	}
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= p.streamer.Len() {
		newPos = p.streamer.Len() - 1
	}
	if err := p.streamer.Seek(newPos); err != nil {
		panic("Seek: " + err.Error())
	}
	speaker.Unlock()
}

func (p *Player) TogglePause() {
	speaker.Lock()
	p.ctrl.Paused = !p.ctrl.Paused
	p.P = p.ctrl.Paused
	speaker.Unlock()
}

func (p *Player) ChangeValume(factor int) {
	speaker.Lock()
	p.volume.Volume += float64(factor) / 10
	speaker.Unlock()
}

func NewPlayer(mp3Path string, loop string) (*Player, error) {
	fd, err := os.Open(mp3Path)
	if err != nil {
		return nil, err
	}

	streamer, format, err := mp3.Decode(fd)
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
	ctrl := &beep.Ctrl{Streamer: beep.Loop(loopN, streamer)}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}
	total := format.SampleRate.D(streamer.Len())

	return &Player{
		fd:         fd,
		sampleRate: format.SampleRate,
		streamer:   streamer,
		ctrl:       ctrl,
		volume:     volume,
		Total:      total,
	}, nil

}

func (p *Player) Update() {
	speaker.Lock()
	position := p.sampleRate.D(p.streamer.Position())
	length := p.sampleRate.D(p.streamer.Len())
	v := int(10 * p.volume.Volume)
	// speed := ap.resampler.Ratio()
	speaker.Unlock()
	p.Position = position
	p.Done = position.Seconds() / length.Seconds()
	p.V = v
}
