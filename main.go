package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"golang.org/x/term"
)

type player struct {
	sampleRate beep.SampleRate
	streamer   beep.StreamSeeker
	ctrl       *beep.Ctrl
	volume     *effects.Volume
}

func newPlayer(sampleRate beep.SampleRate, streamer beep.StreamSeeker) *player {
	ctrl := &beep.Ctrl{Streamer: beep.Loop(-1, streamer)}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}
	return &player{
		sampleRate: sampleRate,
		streamer:   streamer,
		ctrl:       ctrl,
		volume:     volume,
	}
}

func (p *player) play() {
	speaker.Play(p.volume)
}

type display struct {
	width    int
	oldState *term.State
}

func (d *display) init() (keyCh chan byte) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic("term.MakeRaw failed: " + err.Error())
	}
	d.oldState = oldState

	keyCh = make(chan byte)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			keyCh <- buf[0]
		}
	}()

	go func() {
		widthCh := make(chan os.Signal, 1)
		signal.Notify(widthCh, syscall.SIGWINCH)

		widthCh <- nil
		for range widthCh {
			width, _, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				panic("Cannot get the terminal width " + err.Error())
			}
			width -= 20
			d.width = width
		}
	}()

	return keyCh
}

func (d *display) reset() {
	term.Restore(int(os.Stdin.Fd()), d.oldState)

}

func (d *display) update(p *player) {
	speaker.Lock()
	position := p.sampleRate.D(p.streamer.Position())
	length := p.sampleRate.D(p.streamer.Len())
	pres := position.Seconds() / length.Seconds()
	done := int(pres * float64(d.width))
	hashtag := strings.Repeat("#", done)
	space := strings.Repeat(" ", d.width-done)
	volume := p.volume.Volume
	// speed := ap.resampler.Ratio()
	speaker.Unlock()
	positionFormat := fmt.Sprintf("%02d:%02d", int(position.Minutes())%60, int(position.Seconds())%60)
	totalFormat := fmt.Sprintf("%02d:%02d", int(length.Minutes())%60, int(length.Seconds())%60)
	fmt.Printf("\r\r(%02d)[%s%s] %s/%s ", int(10*volume), hashtag, space, positionFormat, totalFormat)
}

func main() {
	if len(os.Args) < 2 {
		println("sawnd <file.mp3>")
		os.Exit(1)
	}

	fd, err := os.Open(os.Args[1])
	if err != nil {
		panic("opening my-file.mp3 failed: " + err.Error())
	}
	defer fd.Close()

	streamer, format, err := mp3.Decode(fd)
	// d, err := newMp3Decoder(fd)
	if err != nil {
		panic("cannot create mp3Reader: " + err.Error())
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/30))

	p := newPlayer(format.SampleRate, streamer)

	// Switch terminal to raw mode so we can read keypresses instantly
	d := display{}
	keyCh := d.init()
	defer d.reset()

	round := time.Tick(100 * time.Microsecond)
	p.play()
	d.update(p)
	for {
		select {
		case key := <-keyCh:
			switch key {
			case ' ':
				speaker.Lock()
				p.ctrl.Paused = !p.ctrl.Paused
				speaker.Unlock()
			case 'q', 3: // 3 = Ctrl+C
				return
			case 'j', 'k':
				speaker.Lock()
				v := 0.1
				if key == 'j' {
					v = -0.1
				}
				p.volume.Volume += v
				speaker.Unlock()
			case 'h', 'l':
				speaker.Lock()
				newPos := p.streamer.Position()
				if key == 'h' {
					newPos -= p.sampleRate.N(10 * time.Second)
				}
				if key == 'l' {
					newPos += p.sampleRate.N(10 * time.Second)
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

		case <-round:
			if p.ctrl.Paused {
				continue
			}
			d.update(p)
		}
	}
}
