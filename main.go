package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
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
	finished   bool
}

func newPlayer(sampleRate beep.SampleRate, streamer beep.StreamSeeker, loop int) *player {
	ctrl := &beep.Ctrl{Streamer: beep.Loop(loop, streamer)}
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
	speaker.Play(beep.Seq(p.volume, beep.Callback(func() { p.finished = true })))
}

type display struct {
	width    int
	oldState *term.State
}

func (d *display) init() (keyCh chan byte) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic("term.MakeRaw: " + err.Error())
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
				panic("term.GetSize: " + err.Error())
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
		println("sawnd <file.mp3> [ --loop <looptimes | -1 infinitely> ]")
		os.Exit(1)
	}

	fd, err := os.Open(os.Args[1])
	if err != nil {
		panic("os.Open: " + err.Error())
	}
	defer fd.Close()
	loop := 1
	if len(os.Args) == 4 && os.Args[2] == "--loop" {
		loop, err = strconv.Atoi(os.Args[3])
		if err != nil {
			println("sawnd <file.mp3> [ --loop <looptimes | -1 infinitely> ]")
			os.Exit(1)
		}
	}

	streamer, format, err := mp3.Decode(fd)
	if err != nil {
		panic("mp3.decode: " + err.Error())
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/30))

	p := newPlayer(format.SampleRate, streamer, loop)

	d := display{}
	keyCh := d.init()
	defer d.reset()

	p.play()
	d.update(p)
	round := time.Tick(100 * time.Microsecond)
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

			if p.finished {
				return
			}

			d.update(p)
		}
	}
}
