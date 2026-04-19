package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	flagParser := flag.NewFlagSet("", flag.ExitOnError)
	flagParser.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.mp3> [OPTION]...\n\r OPTIONS:\n", os.Args[0])
		flagParser.PrintDefaults()
	}
	loops := flagParser.Int("loop", 1, "How many loops, -1 for infinitely.")
	lrcsPath := flagParser.String("lrc", "", "Lrcs file path.")
	if len(os.Args) < 2 {
		flagParser.Usage()
		os.Exit(2)
	}
	flagParser.Parse(os.Args[2:])

	ls, err := newLyrcsSyncer(*lrcsPath)
	if err != nil {
		fmt.Printf("lrc.NewLyrcsSyncer: %v\n\n", err)
		flagParser.Usage()
		os.Exit(2)
	}

	ap, err := newAudioPlayer(os.Args[1], *loops)
	if err != nil {
		fmt.Printf("audio.NewPlayer: %v\n\n", err)
		flagParser.Usage()
		os.Exit(2)
	}

	m := module{
		ap: ap,
		ls: ls,
	}

	keyCh := m.setupTerm()
	defer m.resetTerm()

	m.start()
	// fmt.Print(ansi.CursorDown(5))
	m.update()
	round := time.Tick(100 * time.Microsecond)
	for {
		select {
		case key := <-keyCh:
			switch key {
			case ' ':
				m.ap.togglePause()
			case 'q', 3: // 3 = Ctrl+C
				return
			case 'k':
				m.ap.changeValume(1)
			case 'j':
				m.ap.changeValume(-1)
			case 'h':
				m.ap.seek(-10)
			case 'l':
				m.ap.seek(+10)
			}
		case <-round:
			if m.ap.paused() {
				continue
			}

			if m.ap.finished {
				return
			}

			m.update()
		}
	}
}
