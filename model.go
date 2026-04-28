package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages sent into the Bubbletea loop from goroutines.
type tickMsg time.Time
type finishedMsg struct{}
type lyricsMsg struct {
	current string
	next    string
}

var (
	styleTime          = lipgloss.NewStyle().Underline(true)
	styleCurrentLyrics = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")) // cyan
	styleNextLyrcs     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))          // gray
)

type model struct {
	ap          *audioPlayer
	ls          *lyrcsSyncer
	width       int
	currentLine string
	nextLine    string
	bar         progress.Model
	quitting    bool
}

func newModel(ap *audioPlayer, ls *lyrcsSyncer) model {
	bar := progress.New(
		progress.WithSolidFill("3"), // yellow
		progress.WithoutPercentage(),
	)
	return model{ap: ap, ls: ls, bar: bar}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.bar.Width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			m.ap.togglePause()
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "k":
			m.ap.changeValume(1)
		case "j":
			m.ap.changeValume(-1)
		case "h":
			m.ap.seek(-10)
		case "l":
			m.ap.seek(+10)
		}

	case lyricsMsg:
		m.currentLine = msg.current
		m.nextLine = msg.next

	case finishedMsg:
		m.quitting = true
		return m, tea.Quit

	case tickMsg:
		return m, tickCmd()
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting || m.width == 0 {
		return ""
	}

	// Progress bar
	pct := float64(m.ap.Position()) / float64(m.ap.Len())
	bar := m.bar.ViewAs(pct)

	// Time / volume line
	pos := styleTime.Render(formatTime(m.ap.D(m.ap.Position())))
	end := styleTime.Render(formatTime(m.ap.D(m.ap.Len())))
	volStr := fmt.Sprintf("Volume %02d", m.ap.volume())
	timeStr := fmt.Sprintf("%s/%s", pos, end)
	gap := max(m.width-lipgloss.Width(volStr)-lipgloss.Width(timeStr), 0)
	line2 := volStr + strings.Repeat(" ", gap) + timeStr

	// Lyrics line (centered)
	var line3 string
	if m.currentLine != "" {
		pad := max((m.width-lipgloss.Width(m.currentLine))/2, 0)
		line3 = styleCurrentLyrics.Render(strings.Repeat(" ", pad) + m.currentLine)
	}

	// after line3:
	var line4 string
	if m.nextLine != "" {
		pad := max((m.width-lipgloss.Width(m.nextLine))/2, 0)
		line4 = styleNextLyrcs.Render(strings.Repeat(" ", pad) + m.nextLine)
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s", bar, line2, line3, line4)
}

func formatTime(d time.Duration) string {
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}
