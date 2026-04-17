package main

import (
	"fmt"
	"github/MJ-NMR/sawnd/audio"
	"github/MJ-NMR/sawnd/lrc"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
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

	ls, err := lrc.NewLyrcsSyncer(lrcsPath)
	if err != nil {
		fmt.Println("lrc.NewLyrcsSyncer: ", err)
		exitWithHelp()
	}

	ap, err := audio.NewPlayer(os.Args[1], loops)
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

type module struct {
	ls           *lrc.LyrcsSyncer
	ap           *audio.Player
	termWidth    int
	termOldState *term.State
}

func (m *module) start() {
	m.ap.Play()
	if m.ls != nil {
		m.ls.Sync(m.ap)
	}
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
	fmt.Print("\r\n\r\n")
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

	fill := int(m.ap.Done * float64(m.termWidth-2.0))
	hashtag := ansi.NewStyle(ansi.AttrYellowBackgroundColor).Styled(strings.Repeat(" ", fill))
	gap := m.termWidth - fill - 2
	space := ansi.NewStyle(ansi.AttrBrightBlackBackgroundColor).Styled(strings.Repeat(" ", gap))
	line1 := fmt.Sprintf("\r\r %s%s \r\n", hashtag, space)

	positionFormat := ansi.NewStyle(ansi.AttrUnderline).Styled(formatTime(m.ap.Position))
	totalFormat := ansi.NewStyle(ansi.AttrUnderline).Styled(formatTime(m.ap.Total))
	gap = m.termWidth - len(positionFormat+totalFormat) + 2
	space = strings.Repeat(" ", gap)
	line2 := fmt.Sprintf(" Volume %02d%s%s/%s\r\n", m.ap.V, space, positionFormat, totalFormat)

	var line3 string
	if m.ls != nil {
		gap = int((float64(m.termWidth) - float64(len(m.ls.Current.Line))) / 2.0)
		space = ""
		if gap > 0 {
			space = strings.Repeat(" ", gap)
		}
		line3 = ansi.EraseLineRight + space + m.ls.Current.Line
		line3 = ansi.NewStyle(ansi.AttrBold, ansi.AttrCyanForegroundColor).Styled(line3)
	}

	fmt.Print(ansi.CursorUp(2))
	fmt.Print(line1, line2, line3)

}

func exitWithHelp() {
	println("sawnd <file.mp3> [ --loop <looptimes | -1 infinitely> ]")
	os.Exit(1)
}
