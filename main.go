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
		fmt.Fprintf(os.Stderr, "Usage: %s <audio-file> [OPTION]...\n\r OPTIONS:\n", os.Args[0])
		flagParser.PrintDefaults()
	}
	loops := flagParser.Int("loop", 1, "How many loops, -1 for infinitely.")
	lrcsPath := flagParser.String("lrc", "", "Lrcs file path.")

	if len(os.Args) < 2 {
		flagParser.Usage()
		os.Exit(2)
	}
	flagParser.Parse(os.Args[2:])

	ls, err := newLyrcsSyncer(*lrcsPath, os.Args[1])
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
	defer ap.close()

	mprisSvc, err := startMPRIS(ap, os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mpris: %v\n", err)
	} else {
		defer mprisSvc.Close()
		go mprisSvc.run(ap.Subscribe())
	}

	p := tea.NewProgram(newModel(ap, ls))

	// Give audio player and lyrics syncer a reference to the program
	// so they can send messages into the Bubbletea loop.
	ap.program = p
	if ls != nil {
		ls.program = p
		ls.sync(ap.status.Subscribe())
	}

	ap.play()

	if _, err := p.Run(); err != nil {
		fmt.Println("error running program:", err)
		os.Exit(1)
	}
}
