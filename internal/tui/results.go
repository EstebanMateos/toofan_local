package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vyrx-dev/toofan/internal/game"
	"github.com/vyrx-dev/toofan/internal/theme"
)

func (m model) handleResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if time.Since(m.finishedAt) < 500*time.Millisecond {
		return m, nil
	}

	switch msg.String() {
	case "e":
		if len(m.game.ErrorWords()) > 0 {
			m.showingErrors = !m.showingErrors
			return m, nil
		}
	case "enter":
		if m.raceState == onlineResults && m.raceClient != nil {
			if !m.isRaceHost {
				m.message = "waiting for host to configure next race"
				m.msgTime = time.Now()
				return m, nil
			}
			m.pickingOnline = true
			m.raceState = onlineConfigPick
			m.onlineConfigCur = 0
			m.showingErrors = false
			m.active = screenTyping
			return m, nil
		}
	case "tab":
		// restart immediately after a finished test
	case "ctrl+t":
		theme.Next()
		m.save()
	case "esc":
		if m.raceState == onlineResults && m.raceClient != nil {
			m.disconnectRace()
			m.game = game.New(m.duration, m.mode, m.lang, m.difficulty)
			m.showingErrors = false
			m.active = screenTyping
			return m, nil
		}
		if m.showingErrors {
			m.showingErrors = false
			return m, nil
		}
		m.pickingDur = true
		m.durCur = 0
		for i, d := range durations {
			if d == m.duration {
				m.durCur = i
			}
		}
		m.game = game.New(m.duration, m.mode, m.lang, m.difficulty)
		if m.activeRace != nil {
			m.game.SetText(m.activeRace.Text)
		}
		m.showingErrors = false
		m.active = screenTyping
		return m, nil
	}

	if m.showingErrors {
		m.showingErrors = false
		return m, nil
	}

	if m.raceState == onlineResults && m.raceClient != nil {
		m.disconnectRace()
	}

	m.game = game.New(m.duration, m.mode, m.lang, m.difficulty)
	if m.activeRace != nil {
		m.game.SetText(m.activeRace.Text)
	}
	m.showingErrors = false
	m.bots = nil
	m.botLastTick = time.Time{}
	m.active = screenTyping
	return m, nil
}

func getSassyLine(wpm float64) string {
	switch {
	case wpm < 30:
		return "you type like my grandma... and she's dead."
	case wpm < 50:
		return "are you using just your index fingers?"
	case wpm < 70:
		return "not bad, but keep it off your resume."
	case wpm < 90:
		return "fast enough to look busy when the boss walks by."
	case wpm < 120:
		return "calm down turbo, leave some keys for the rest of us."
	default:
		return "what kind of gaming chair do you have?!"
	}
}

func (m model) viewResults(p theme.Palette) string {
	if m.showingErrors {
		return m.viewErrors(p)
	}

	dim := lipgloss.NewStyle().Foreground(p.Foreground)
	val := lipgloss.NewStyle().Foreground(p.Typed)
	hi := lipgloss.NewStyle().Foreground(p.Accent).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(p.Error)
	italic := lipgloss.NewStyle().Foreground(p.Foreground).Italic(true)
	success := lipgloss.NewStyle().Foreground(p.Success)

	r := m.result

	timeStr := fmt.Sprintf("%ds", m.duration)
	if m.duration == 0 {
		if r.WPM > 0 {
			elapsed := float64(r.Chars) / 5.0 / r.WPM * 60.0
			timeStr = fmt.Sprintf("%ds", int(math.Round(elapsed)))
		} else {
			timeStr = "0s"
		}
	}

	errStr := val.Render("0")
	if r.Mistakes > 0 {
		errStr = errStyle.Render(fmt.Sprintf("%d", r.Mistakes))
	}

	cw := 10
	statBlock := func(label, value string) string {
		return lipgloss.NewStyle().Width(cw).Align(lipgloss.Center).Render(
			lipgloss.JoinVertical(lipgloss.Center, dim.Render(label), value),
		)
	}

	blocks := []string{
		statBlock("wpm", hi.Render(fmt.Sprintf("%.0f", r.WPM))),
	}
	if m.activeRace != nil {
		blocks = append(blocks, statBlock("old wpm", val.Render(fmt.Sprintf("%.0f", m.activeRace.Stats.WPM))))
	}
	blocks = append(blocks,
		statBlock("acc", val.Render(fmt.Sprintf("%.0f%%", r.Accuracy))),
		statBlock("raw", val.Render(fmt.Sprintf("%.0f", r.Raw))),
		statBlock("typos", errStr),
		statBlock("time", val.Render(timeStr)),
	)
	stats := lipgloss.JoinHorizontal(lipgloss.Top, blocks...)

	var out []string
	out = append(out, "", stats, "", "")

	sassy := getSassyLine(r.WPM)
	if sassy != "" {
		out = append(out, italic.Render(sassy), "")
	}

	if m.gotNewPB {
		pbLine := success.Bold(true).Render(fmt.Sprintf("★ NEW PB!  %.0f → %.0f", m.pb, r.WPM))
		out = append(out, pbLine)
	} else if m.pb > 0 {
		out = append(out, dim.Render(fmt.Sprintf("pb: %.0f", m.pb)))
	}

	if len(m.bots) > 0 {
		userProg := 1.0
		placements := game.BotPlacements(m.bots, userProg)
		// Inject user WPM into placements for informative display
		for i := range placements {
			if placements[i].IsUser {
				placements[i].WPM = r.WPM
			}
		}
		out = append(out, "", viewBotResults(p, placements))
	} else if m.raceState == onlineResults {
		out = append(out, "", m.viewOnlineResults(p))
		if m.isRaceHost {
			out = append(out, "", dim.Render("enter configure next race · esc leave room"))
		} else {
			out = append(out, "", dim.Render("waiting for host · esc leave room"))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Center, out...)
}

func (m model) viewErrors(p theme.Palette) string {
	hi := lipgloss.NewStyle().Foreground(p.Accent)
	errStyle := lipgloss.NewStyle().Foreground(p.Error)

	errWords := m.game.ErrorWords()

	var out []string
	out = append(out, hi.Render("words to practice"))
	out = append(out, "")

	for i := 0; i < len(errWords); i += 5 {
		end := i + 5
		if end > len(errWords) {
			end = len(errWords)
		}
		row := errStyle.Render(strings.Join(errWords[i:end], "  "))
		out = append(out, row)
	}

	out = append(out, "", lipgloss.NewStyle().Foreground(p.Foreground).Render("press any key to return"))

	return lipgloss.JoinVertical(lipgloss.Center, out...)
}
