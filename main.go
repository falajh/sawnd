package main

import (
	"charm.land/bubbles/v2/viewport"
	"fmt"
	"github/MJ-NMR/sawnd/audio"
	"github/MJ-NMR/sawnd/lrc"
	"golang.org/x/term"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type module struct {
	vp           viewport.Model
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
	hideCurser()

	return keyCh
}

func (m *module) resetTerm() {
	showCurser()
	term.Restore(int(os.Stdin.Fd()), m.termOldState)
}

func formatTime(d time.Duration) string {
	m, s := int(d.Minutes())%60, int(d.Seconds())%60
	return fmt.Sprintf("%02d:%02d", m, s)
}

const EraseEntireLine = "\x1b[2K"

func hideCurser() {
	fmt.Print("\x1b[?25l")
}

func showCurser() {
	fmt.Print("\x1b[?25h")
}

func cursorDown(n int) {
	var s string
	if n > 1 {
		s = strconv.Itoa(n)
	}
	fmt.Print("\x1b[" + s + "B")
}

func cursorUp(n string) {
	fmt.Printf("\x1b[%sA", n)
}

func (m *module) update() {
	m.ap.Update()
	if m.termWidth <= 0 {
		return
	}
	positionFormat := formatTime(m.ap.Position)
	totalFormat := formatTime(m.ap.Total)
	fill := int(m.ap.Done * float64(m.termWidth-2.0))
	hashtag := strings.Repeat("#", fill)
	gap := m.termWidth - fill - 2
	space := strings.Repeat(" ", gap)
	line1 := EraseEntireLine + fmt.Sprintf("\r\r[%s%s]\r\n", hashtag, space)

	gap = m.termWidth - 2 - len(positionFormat) - len(totalFormat) - 5
	space = strings.Repeat(" ", gap)
	line2 := EraseEntireLine + fmt.Sprintf(" V %02d%s%s/%s \r\n", m.ap.V, space, positionFormat, totalFormat)

	gap = int((float64(m.termWidth) - float64(len(m.ls.Current.Line))) / 2.0)
	space = ""
	if gap > 0 {
		space = strings.Repeat(" ", gap)
	}
	line3 := EraseEntireLine + space + m.ls.Current.Line
	if m.ls.Current.Line == "" {
		line3 = EraseEntireLine + strings.Repeat(" ", m.termWidth)
	}

	cursorUp("2")
	fmt.Print(line1, line2, line3)

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
	cursorDown(2)
	m.update()
	round := time.Tick(100 * time.Microsecond)
	for {
		select {
		case key := <-keyCh:
			switch key {
			case ' ':
				m.ap.TogglePause()
			case 'q', 3: // 3 = Ctrl+C
				fmt.Println()
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
		case <-m.ls.ReadyQuie:
			continue
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
