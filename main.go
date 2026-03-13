package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
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
		panic("oto.NewContext" + err.Error())
	}
	<-ready
	c := countWraper{r: decodedMp3}
	player := otoCtx.NewPlayer(&c)

	total := decodedMp3.Length()
	player.Play()
	for player.IsPlaying() {
		current := float64(c.n) / float64(total) * 100
		fmt.Print("\033[H\033[2J")
		fmt.Printf("[%s]%d%%\n", strings.Repeat("#", int(current))+strings.Repeat(" ", 100-int(current)), int(current))
		time.Sleep(time.Second)
	}
}
