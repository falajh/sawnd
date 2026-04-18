package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type module struct {
	ls           *lyrcsSyncer
	ap           *audioPlayer
	termWidth    int
	termOldState *term.State
}

func (m *module) start() {
	m.ap.play()
	if m.ls != nil {
		m.ls.sync(m.ap)
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
	fmt.Println("\r")
}

func (m *module) update() {
	if m.termWidth <= 0 {
		return
	}

	done := m.ap.position() * 100 / m.ap.len()
	fill := (done * (m.termWidth - 2)) / 100

	hashtag := ansi.NewStyle(ansi.AttrYellowBackgroundColor).Styled(strings.Repeat(" ", fill))
	gap := m.termWidth - fill - 2
	space := ansi.NewStyle(ansi.AttrBrightBlackBackgroundColor).Styled(strings.Repeat(" ", gap))
	line1 := fmt.Sprintf("\r\r %s%s \r\n", hashtag, space)

	positionFormat := ansi.NewStyle(ansi.AttrUnderline).Styled(formatTime(m.ap.positionD()))
	lenghtFormat := ansi.NewStyle(ansi.AttrUnderline).Styled(formatTime(m.ap.lenD()))
	gap = m.termWidth - len(positionFormat+lenghtFormat) + 2
	space = strings.Repeat(" ", gap)
	line2 := fmt.Sprintf("\r\r Volume %02d%s%s/%s\r\n", m.ap.volume(), space, positionFormat, lenghtFormat)

	var line3 string
	if m.ls != nil {
		gap = int((float64(m.termWidth) - float64(len(m.ls.current.line))) / 2.0)
		space = ""
		if gap > 0 {
			space = strings.Repeat(" ", gap)
		}
		line3 = ansi.EraseLineRight + space + m.ls.current.line
		line3 = ansi.NewStyle(ansi.AttrBold, ansi.AttrCyanForegroundColor).Styled(line3)
	}

	fmt.Print(ansi.CursorUp(2))
	fmt.Print(line1, line2, line3)

}

func formatTime(d time.Duration) string {
	m, s := int(d.Minutes())%60, int(d.Seconds())%60
	return fmt.Sprintf("%02d:%02d", m, s)
}
