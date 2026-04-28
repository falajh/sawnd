package main

import (
	"flag"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
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

	p := tea.NewProgram(newModel(ap, ls))

	// Give audio player and lyrics syncer a reference to the program
	// so they can send messages into the Bubbletea loop.
	ap.program = p
	if ls != nil {
		ls.program = p
	}

	ap.play()
	if ls != nil {
		ls.sync(ap)
	}

	if _, err := p.Run(); err != nil {
		fmt.Println("error running program:", err)
		os.Exit(1)
	}
}

