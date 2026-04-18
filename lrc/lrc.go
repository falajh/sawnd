package lrc

import (
	"fmt"
	"github/MJ-NMR/sawnd/audio"
	"os"
	"strconv"
	"strings"
	"time"
)

type LyrcsSyncer struct {
	qiue    []lrc
	Current lrc
}

type lrc struct {
	d    time.Duration
	Line string
	Err  bool
}

func (ls *LyrcsSyncer) Sync(ap *audio.Player) {
	go func() {
		var i int
		for {
			nextLrc := ls.qiue[i]
			if nextLrc.Err {
				fmt.Println(nextLrc.Line)
				os.Exit(1)
			}
			for {
				if ls.Current.d > ap.Position {
					i = 0
					nextLrc = ls.qiue[i]
				}
				if nextLrc.d <= ap.Position {
					ls.Current = nextLrc
					i++
					if i < len(ls.qiue) {
						break
					}
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()
}

func NewLyrcsSyncer(path string) (*LyrcsSyncer, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(f), "\n")
	qiue := make([]lrc, len(lines))
	formatLrcs(lines, qiue)
	return &LyrcsSyncer{qiue: qiue}, nil
}

func formatLrcs(lines []string, qiue []lrc) {
	for i, line := range lines {
		sublines := strings.SplitN(line[1:], "] ", 2)
		if len(sublines) < 2 {
			qiue[i] = lrc{Err: true, Line: "Coudn't parse lrc line format"}
			return
		}
		times := strings.Split(sublines[0][1:], ":")

		sec, err := strconv.ParseFloat(times[1], 32)
		if err != nil {
			qiue[i] = lrc{Err: true, Line: "Coudn't parse secend format"}
			return
		}
		m, err := strconv.Atoi(times[0])
		if err != nil {
			qiue[i] = lrc{Err: true, Line: "Coudn't parse minutes format"}
			return
		}

		d := time.Duration(float64(m)*float64(time.Minute) + sec*float64(time.Second))
		qiue[i] = lrc{d: d, Line: sublines[1], Err: false}
	}
}
