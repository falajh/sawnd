package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"golang.org/x/term"
)

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

	mr, err := newMp3Reader(fd)
	if err != nil {
		panic("cannot create mp3Reader: " + err.Error())
	}

	opt := oto.NewContextOptions{
		Format:       oto.FormatSignedInt16LE,
		ChannelCount: 2,
		SampleRate:   44100,
	}
	otoCtx, ready, err := oto.NewContext(&opt)
	if err != nil {
		panic("cannot create oto.NewContext: " + err.Error())
	}

	<-ready
	player := otoCtx.NewPlayer(mr)

	// Switch terminal to raw mode so we can read keypresses instantly
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic("term.MakeRaw failed: " + err.Error())
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	var width int
	total := mr.Length

	keyCh := make(chan byte)
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

	paused := false
	sigCh <- nil
	player.Play()
	for {
		select {
		case <-sigCh:
			width, _, err = term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				panic("Cannot get the terminal width " + err.Error())
			}
			width -= 11

		case key := <-keyCh:
			switch key {
			case ' ':
				if paused {
					player.Play()
					paused = false
				} else {
					player.Pause()
					paused = true
				}
			case 'q', 'Q', 3: // 3 = Ctrl+C
				return
			}

		default:
			if !player.IsPlaying() && !paused {
				return
			}
			percent := float64(mr.count) / float64(total)
			done := int(percent * float64(width))
			fill := width - int(done)
			fmt.Printf("\r\r[%s%s] %.2f%% ", strings.Repeat("#", done), strings.Repeat(" ", fill), percent*100)
			time.Sleep(100 * time.Microsecond)
		}
	}
}

func newMp3Reader(fd *os.File) (*mp3Reader, error) {
	decodedMp3, err := mp3.NewDecoder(fd)
	if err != nil {
		return nil, err
	}

	return &mp3Reader{
		decoder: decodedMp3,
		Length:  decodedMp3.Length(),
	}, nil
}

type mp3Reader struct {
	decoder *mp3.Decoder
	count   int64
	Length  int64
}

func (mr *mp3Reader) Read(p []byte) (n int, err error) {
	n, err = mr.decoder.Read(p)
	mr.count += int64(n)
	return n, err
}
