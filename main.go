package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"golang.org/x/term"
)

type countWraper struct {
	r io.Reader
	n int64
}

func (c *countWraper) Read(p []byte) (n int, err error) {
	n, err = c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func main() {

	if len(os.Args) < 2 {
		println("sawnd <file.mp3>")
		os.Exit(1)
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		panic("opening my-file.mp3 failed: " + err.Error())
	}

	decodedMp3, err := mp3.NewDecoder(file)
	if err != nil {
		panic("mp3.NewDecoder failed: " + err.Error())
	}

	opt := oto.NewContextOptions{
		Format:       oto.FormatSignedInt16LE,
		ChannelCount: 2,
		SampleRate:   44100,
	}
	otoCtx, ready, err := oto.NewContext(&opt)
	if err != nil {
		panic("oto.NewContext " + err.Error())
	}

	<-ready
	c := countWraper{r: decodedMp3}
	player := otoCtx.NewPlayer(&c)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	width := 100
	total := decodedMp3.Length()

	sigCh <- nil
	player.Play()
	for {
		select {
		case <-sigCh:
			width, _, err = term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				panic("Cannot get the terminal width " + err.Error())
			}
			width -= 10
		default:
			if !player.IsPlaying() {
				return
			}
			percent := float64(c.n) / float64(total) * float64(width)
			fmt.Printf("\r\r[%s] %.2f%% ", strings.Repeat("#", int(percent))+strings.Repeat(" ", width-int(percent)), percent)
			time.Sleep(time.Second)
		}
	}
}
