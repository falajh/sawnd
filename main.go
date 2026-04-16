package main

import (
	"fmt"
	"github/MJ-NMR/sawnd/audio"
	"github/MJ-NMR/sawnd/lrc"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

type module struct {
	ls           *lrc.LyrcsSyncer
	ap           *audio.Player
	termWidth    int
	termOldState *term.State
}

func (m *module) start() {
	m.ap.Play()
	m.ls.Sync(m.ap)
}

func (m *module) setupTerm() (keyCh chan byte) {
	go func() {
		widthCh := make(chan os.Signal, 1)
		signal.Notify(widthCh, syscall.SIGWINCH)

		widthCh <- nil
		for range widthCh {
			width, _, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				panic("term.GetSize: " + err.Error())
			}
			width -= 20
			m.termWidth = width
		}
	}()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic("term.MakeRaw: " + err.Error())
	}
	m.termOldState = oldState

	keyCh = make(chan byte)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			keyCh <- buf[0]
		}
	}()

	return keyCh
}

func (m *module) resetTerm() {
	term.Restore(int(os.Stdin.Fd()), m.termOldState)

}
func formatTime(d time.Duration) string {
	m, s := int(d.Minutes())%60, int(d.Seconds())%60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func (m *module) update() {
	m.ap.Update()
	positionFormat := formatTime(m.ap.Position)
	totalFormat := formatTime(m.ap.Total)
	hashtag := strings.Repeat("#", int(m.ap.Done*float64(m.termWidth)))
	space := strings.Repeat(" ", m.termWidth-int(m.ap.Done*float64(m.termWidth)))
	fmt.Printf("\r\r(%02d)[%s%s] %s/%s ", m.ap.V, hashtag, space, positionFormat, totalFormat)
}
func exitWithHelp() {
	println("sawnd <file.mp3> [ --loop <looptimes | -1 infinitely> ]")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		exitWithHelp()
	}

	fd, err := os.Open(os.Args[1])
	if err != nil {
		panic("os.Open: " + err.Error())
	}
	defer fd.Close()

	loops := 1
	lrcsPath := ""
	switch len(os.Args) {
	case 4:
		switch os.Args[2] {
		case "--loop":
			loops, err = strconv.Atoi(os.Args[3])
			if err != nil {
				exitWithHelp()
			}
		case "--lrc":
			lrcsPath = os.Args[3]
		}
	}

	ls, err := lrc.NewLyrcsSyncer(lrcsPath)
	if err != nil {
		fmt.Println(err)
		exitWithHelp()
	}

	ap, err := audio.NewPlayer(fd, loops)
	if err != nil {
		fmt.Println(err)
		exitWithHelp()
	}

	m := module{
		ap: ap,
		ls: ls,
	}

	keyCh := m.setupTerm()
	defer m.resetTerm()

	m.start()
	m.update()
	round := time.Tick(100 * time.Microsecond)
	for {
		select {
		case key := <-keyCh:
			switch key {
			case ' ':
				m.ap.TogglePause()
			case 'q', 3: // 3 = Ctrl+C
				return
			case 'k':
				m.ap.ChangeValume(1)
			case 'j':
				m.ap.ChangeValume(-1)
			case 'h':
				m.ap.Seek(-10)
			case 'l':
				m.ap.Seek(+10)
			}
		case l := <-m.ls.ReadyQuie:
			fmt.Printf("\n\r%s", l.Line)
		case <-round:
			if m.ap.P {
				continue
			}

			if m.ap.Finished {
				return
			}

			m.update()
		}
	}
}
