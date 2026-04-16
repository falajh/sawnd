package lrc

import (
	"bufio"
	"fmt"
	"github/MJ-NMR/sawnd/audio"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type Lrxqiue chan lrc

type LyrcsSyncer struct {
	ReadyQuie Lrxqiue
	input     Lrxqiue
}

type lrc struct {
	d    time.Duration
	Line string
	Err  bool
}

func (ls *LyrcsSyncer) Sync(ap *audio.Player) {
	ls.ReadyQuie = make(Lrxqiue)
	go func() {
		for l := range ls.input {
			if l.Err {
				fmt.Println(l.Line)
				os.Exit(1)
			}
			for {
				if l.d <= ap.Position {
					ls.ReadyQuie <- l
					break
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()
}

func NewLyrcsSyncer(path string) (*LyrcsSyncer, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	br := bufio.NewReader(fd)
	initQiue := make(Lrxqiue)
	go formatLrcs(br, initQiue)
	return &LyrcsSyncer{input: initQiue}, nil
}

func formatLrcs(br *bufio.Reader, lrcCh chan lrc) {
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				lrcCh <- lrc{Err: true, Line: err.Error()}
			}
			return
		}
		sublines := strings.SplitN(line[1:], "] ", 2)
		if len(sublines) < 2 {
			lrcCh <- lrc{Err: true, Line: "Coudn't parse lrc line format"}
			return
		}
		times := strings.Split(sublines[0][1:], ":")

		sec, err := strconv.ParseFloat(times[1], 32)
		if err != nil {
			lrcCh <- lrc{Err: true, Line: "Coudn't parse secend format"}
			return
		}
		m, err := strconv.Atoi(times[0])
		if err != nil {
			lrcCh <- lrc{Err: true, Line: "Coudn't parse minutes format"}
			return
		}

		d := time.Duration(float64(m)*float64(time.Minute) + sec*float64(time.Second))
		lrcCh <- lrc{d: d, Line: sublines[1], Err: false}
	}
}
