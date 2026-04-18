package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	loops := ""
	lrcsPath := ""
	if len(os.Args) >= 4 {
		switch os.Args[2] {
		case "--loop":
			loops = os.Args[3]
		case "--lrc":
			lrcsPath = os.Args[3]
		default:
			exitWithHelp()
		}
	}
	if len(os.Args) == 6 {
		if os.Args[4] == "--loop" && loops == "" {
			loops = os.Args[5]
		} else if os.Args[4] == "--lrc" && lrcsPath == "" {
			lrcsPath = os.Args[5]
		} else {
			exitWithHelp()
		}
	}
	if len(os.Args) < 2 {
		exitWithHelp()
	}

	ls, err := newLyrcsSyncer(lrcsPath)
	if err != nil {
		fmt.Println("lrc.NewLyrcsSyncer: ", err)
		exitWithHelp()
	}

	ap, err := newAudioPlayer(os.Args[1], loops)
	if err != nil {
		fmt.Println("audio.NewPlayer: ", err)
		exitWithHelp()
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

func exitWithHelp() {
	println("sawnd <file.mp3> [ --loop <looptimes | -1 infinitely> ]")
	os.Exit(1)
}
