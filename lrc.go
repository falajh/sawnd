package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type lyrcsSyncer struct {
	qiue    []lrc
	current lrc
	program *tea.Program // set after program is created
}

type lrc struct {
	d    time.Duration
	line string
}

func (ls *lyrcsSyncer) sync(ap *audioPlayer) {
	if ls.program == nil {
		return
	}

	go func() {
		var (
			i       int
			current lrc
		)
		next := ls.qiue[i]
		length := len(ls.qiue)
		if length < 1 {
			return
		}

		for {
			pos := ap.positionD()
			// Handle seek backwards
			if current.d > pos {
				i = 0
				current = lrc{}
				next = ls.qiue[i]
			}

			if next.d <= pos {
				current = next
				i++

				if i < length {
					next = ls.qiue[i]
					ls.program.Send(lyricsMsg{current: current.line, next: next.line})
				} else {
					ls.program.Send(lyricsMsg{current: current.line, next: ""})
				}
				continue
			}

			time.Sleep(time.Millisecond)
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
	qiue := make([]lrc, 0, len(lines))
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		sublines := strings.SplitN(line[1:], "] ", 2)
		if len(sublines) < 2 {
			return nil, fmt.Errorf("couldn't parse lrc line format: %q", line)
		}
		times := strings.Split(sublines[0], ":")
		if len(times) < 2 {
			return nil, fmt.Errorf("couldn't parse time format: %q", sublines[0])
		}
		sec, err := strconv.ParseFloat(times[1], 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse seconds: %w", err)
		}
		m, err := strconv.Atoi(times[0])
		if err != nil {
			return nil, fmt.Errorf("couldn't parse minutes: %w", err)
		}
		d := time.Duration(float64(m)*float64(time.Minute) + sec*float64(time.Second))
		qiue = append(qiue, lrc{d: d, line: sublines[1]})
	}
	return qiue, nil
}
