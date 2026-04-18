package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type lyrcsSyncer struct {
	qiue    []lrc
	current lrc
}

type lrc struct {
	d    time.Duration
	line string
}

func (ls *lyrcsSyncer) sync(ap *audioPlayer) {
	go func() {
		var i int
		for {
			nextLrc := ls.qiue[i]
			for {
				if ls.current.d > ap.positionD() {
					i = 0
					nextLrc = ls.qiue[i]
				}
				if nextLrc.d <= ap.positionD() {
					ls.current = nextLrc
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

func newLyrcsSyncer(path string) (*lyrcsSyncer, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(f), "\n")
	qiue, err := formatLrcs(lines)
	return &lyrcsSyncer{qiue: qiue}, err
}

func formatLrcs(lines []string) ([]lrc, error) {
	qiue := make([]lrc, len(lines))
	for i, line := range lines {
		sublines := strings.SplitN(line[1:], "] ", 2)
		if len(sublines) < 2 {
			return nil, fmt.Errorf("Coudn't parse lrc line format")
		}
		times := strings.Split(sublines[0][1:], ":")

		sec, err := strconv.ParseFloat(times[1], 32)
		if err != nil {
			return nil, fmt.Errorf("Coudn't parse secend format")
		}
		m, err := strconv.Atoi(times[0])
		if err != nil {
			return nil, fmt.Errorf("Coudn't parse minutes format")
		}

		d := time.Duration(float64(m)*float64(time.Minute) + sec*float64(time.Second))
		qiue[i] = lrc{d: d, line: sublines[1]}
	}
	return qiue, nil
}
