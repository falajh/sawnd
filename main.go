package main

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
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

	fmt.Print(ansi.HideCursor)
	return keyCh
}

func (m *module) resetTerm() {
	fmt.Print(ansi.ShowCursor)
	term.Restore(int(os.Stdin.Fd()), m.termOldState)
}

func formatTime(d time.Duration) string {
	m, s := int(d.Minutes())%60, int(d.Seconds())%60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func (m *module) update() {
	m.ap.Update()
	if m.termWidth <= 0 {
		return
	}
	positionFormat := ansi.NewStyle(ansi.AttrUnderline).Styled(formatTime(m.ap.Position))
	totalFormat := ansi.NewStyle(ansi.AttrUnderline).Styled(formatTime(m.ap.Total))
	fill := int(m.ap.Done * float64(m.termWidth-2.0))
	hashtag := ansi.NewStyle(ansi.AttrYellowBackgroundColor).Styled(strings.Repeat(" ", fill))
	gap := m.termWidth - fill - 2
	space := ansi.NewStyle(ansi.AttrBrightBlackBackgroundColor).Styled(strings.Repeat(" ", gap))
	line1 := fmt.Sprintf("\r\r %s%s \r\n", hashtag, space)

	gap = m.termWidth - 2 - len(positionFormat) - len(totalFormat) - 5
	space = strings.Repeat(" ", gap)
	line2 := fmt.Sprintf(" V %02d%s%s/%s \r\n", m.ap.V, space, positionFormat, totalFormat)

	gap = int((float64(m.termWidth) - float64(len(m.ls.Current.Line))) / 2.0)
	space = ""
	if gap > 0 {
		space = strings.Repeat(" ", gap)
	}
	line3 := ansi.EraseLineRight + space + m.ls.Current.Line
	if m.ls.Current.Line == "" {
		line3 = ansi.EraseEntireLine
	}
	line3 = ansi.NewStyle(ansi.AttrBold, ansi.AttrCyanForegroundColor).Styled(line3)

	fmt.Print(ansi.CursorUp(2))
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
	fmt.Print(ansi.CursorDown(2))
	m.update()
	round := time.Tick(100 * time.Microsecond)
	for {
		select {
		case key := <-keyCh:
			switch key {
			case ' ':
				m.ap.TogglePause()
			case 'q', 3: // 3 = Ctrl+C
				fmt.Println("\r")
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
