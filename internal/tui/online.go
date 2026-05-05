package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vyrx-dev/toofan/internal/game"
	"github.com/vyrx-dev/toofan/internal/theme"
)

const (
	onlineOff        = 0
	onlineSizePick   = 1
	onlineUsername   = 2
	onlineConnecting = 3
	onlineLobby      = 4
	onlineCountdown  = 5
	onlineRacing     = 6
	onlineResults    = 7
)

var onlineSizes = []int{2, 3, 4, 5, 6}

func (m model) handleOnline(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.raceState {
	case onlineOff, onlineSizePick:
		return m.handleSizePicker(msg)
	case onlineUsername:
		return m.handleUsernameInput(msg)
	case onlineLobby, onlineCountdown:
		if msg.String() == "esc" {
			m.disconnectRace()
			return m, nil
		}
	case onlineResults:
		if msg.String() == "esc" || msg.String() == "enter" {
			m.disconnectRace()
			return m, nil
		}
	}
	return m, nil
}

func (m model) handleSizePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.onlineSizeCur > 0 {
			m.onlineSizeCur--
		}
	case "down", "j":
		if m.onlineSizeCur < len(onlineSizes)-1 {
			m.onlineSizeCur++
		}
	case "enter":
		m.onlineSize = onlineSizes[m.onlineSizeCur]
		m.raceState = onlineUsername
	case "esc":
		m.pickingOnline = false
		m.raceState = onlineOff
	}
	return m, nil
}

func (m model) handleUsernameInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.pickingOnline = false
		m.raceState = onlineOff
		m.usernameBuf = ""
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.usernameBuf)
		if name == "" && m.username != "" {
			name = m.username
		}
		if len(name) < 2 || len(name) > 16 {
			return m, nil
		}
		m.username = name
		m.usernameBuf = ""
		m.save()
		m.raceState = onlineConnecting

		serverURL := m.serverURL
		if serverURL == "" {
			serverURL = game.DefaultServerURL
		}
		m.raceClient = game.NewRaceClient(serverURL, m.username)
		if err := m.raceClient.Join(m.onlineSize); err != nil {
			m.message = "connect failed: " + err.Error()
			m.msgTime = time.Now()
			m.pickingOnline = false
			m.raceState = onlineOff
			m.raceClient = nil
			return m, nil
		}
		m.raceState = onlineLobby
		return m, m.listenRaceMsg()
	case "backspace":
		if len(m.usernameBuf) > 0 {
			m.usernameBuf = m.usernameBuf[:len(m.usernameBuf)-1]
		}
	default:
		for _, r := range msg.Runes {
			if len(m.usernameBuf) < 16 {
				m.usernameBuf += string(r)
			}
		}
	}
	return m, nil
}

func (m *model) disconnectRace() {
	if m.raceClient != nil {
		m.raceClient.Close()
		m.raceClient = nil
	}
	m.pickingOnline = false
	m.raceState = onlineOff
	m.racePlayers = nil
	m.raceText = ""
}

func (m model) listenRaceMsg() tea.Cmd {
	if m.raceClient == nil {
		return nil
	}
	rc := m.raceClient
	return func() tea.Msg {
		msg, ok := <-rc.Messages()
		if !ok {
			return raceServerMsg{Msg: game.ServerMsg{Type: "disconnected"}}
		}
		return raceServerMsg{Msg: msg}
	}
}

func (m model) handleRaceServerMsg(msg game.ServerMsg) (model, tea.Cmd) {
	if m.raceClient == nil {
		return m, nil
	}

	switch msg.Type {
	case "joined":
		var payload game.LobbyPayload
		json.Unmarshal(msg.Payload, &payload)
		m.raceClient.SetRoom(payload.Room)
		m.onlineCount = payload.Online
		m.raceState = onlineLobby

	case "countdown":
		var payload game.CountdownPayload
		json.Unmarshal(msg.Payload, &payload)
		m.raceState = onlineCountdown

	case "start":
		var payload game.StartPayload
		json.Unmarshal(msg.Payload, &payload)
		m.raceText = payload.Text
		m.game.Reset(m.mode, m.lang, m.difficulty)
		m.game.SetText(payload.Text)
		m.raceState = onlineRacing
		m.pickingOnline = false

	case "progress":
		var payload game.ProgressPayload
		json.Unmarshal(msg.Payload, &payload)
		for i := range payload.Players {
			if m.raceClient != nil && payload.Players[i].Name == m.raceClient.Name() {
				payload.Players[i].IsUser = true
			}
		}
		m.racePlayers = payload.Players

	case "finish":
		var payload game.FinishPayload
		json.Unmarshal(msg.Payload, &payload)
		for i := range payload.Placements {
			if m.raceClient != nil && payload.Placements[i].Name == m.raceClient.Name() {
				payload.Placements[i].IsUser = true
			}
		}
		m.racePlayers = payload.Placements
		m.raceState = onlineResults

	case "online":
		var payload game.OnlinePayload
		json.Unmarshal(msg.Payload, &payload)
		m.onlineCount = payload.Count

	case "disconnected":
		m.disconnectRace()
		m.message = "disconnected from server"
		return m, nil
	}

	return m, m.listenRaceMsg()
}

func (m model) viewOnline(p theme.Palette) string {
	switch m.raceState {
	case onlineOff, onlineSizePick:
		return m.viewSizePicker(p)
	case onlineUsername:
		return m.viewUsernamePrompt(p)
	case onlineConnecting:
		return m.viewConnecting(p)
	case onlineLobby:
		return m.viewLobby(p)
	case onlineCountdown:
		return m.viewRaceCountdown(p)
	case onlineResults:
		return m.viewOnlineResults(p)
	}
	return ""
}

func (m model) viewSizePicker(p theme.Palette) string {
	labels := make([]string, len(onlineSizes))
	for i, size := range onlineSizes {
		labels[i] = fmt.Sprintf("%d players", size)
	}
	return renderList(p, "multiplayer room size", labels, nil, m.onlineSizeCur)
}

func (m model) viewUsernamePrompt(p theme.Palette) string {
	dim := lipgloss.NewStyle().Foreground(p.Foreground)
	hi := lipgloss.NewStyle().Foreground(p.Accent)
	val := lipgloss.NewStyle().Foreground(p.Typed)
	cur := lipgloss.NewStyle().Foreground(p.Background).Background(p.Cursor)

	display := m.usernameBuf
	if m.username != "" && display == "" {
		display = m.username
	}

	var inputLine string
	if display == "" {
		inputLine = cur.Render(" ")
	} else {
		inputLine = val.Render(display) + cur.Render(" ")
	}

	lines := []string{
		hi.Render("multiplayer"),
		"",
		dim.Render("enter a username (2-16 chars)"),
		"",
		inputLine,
		"",
		dim.Render("enter to join · esc to cancel"),
	}

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

func (m model) viewConnecting(p theme.Palette) string {
	dim := lipgloss.NewStyle().Foreground(p.Foreground)
	hi := lipgloss.NewStyle().Foreground(p.Accent)

	return lipgloss.JoinVertical(lipgloss.Center,
		hi.Render("multiplayer"),
		"",
		dim.Render("connecting..."),
	)
}

func (m model) viewLobby(p theme.Palette) string {
	dim := lipgloss.NewStyle().Foreground(p.Foreground)
	hi := lipgloss.NewStyle().Foreground(p.Accent)
	val := lipgloss.NewStyle().Foreground(p.Typed)

	lines := []string{
		hi.Render("lobby"),
		"",
		dim.Render("waiting for players..."),
		"",
	}

	if m.onlineCount > 0 {
		lines = append(lines, val.Render(fmt.Sprintf("%d", m.onlineCount))+" "+dim.Render("online"))
	}

	if len(m.racePlayers) > 0 {
		lines = append(lines, "")
		for _, pl := range m.racePlayers {
			prefix := "  "
			if pl.IsUser {
				prefix = hi.Render("> ")
			}
			lines = append(lines, prefix+val.Render(pl.Name))
		}
	}

	lines = append(lines, "", dim.Render("esc to leave"))

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

func (m model) viewRaceCountdown(p theme.Palette) string {
	hi := lipgloss.NewStyle().Foreground(p.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(p.Foreground)

	return lipgloss.JoinVertical(lipgloss.Center,
		hi.Render("get ready"),
		"",
		dim.Render("race starting..."),
	)
}

func (m model) viewOnlineResults(p theme.Palette) string {
	dim := lipgloss.NewStyle().Foreground(p.Foreground)
	hi := lipgloss.NewStyle().Foreground(p.Accent).Bold(true)
	val := lipgloss.NewStyle().Foreground(p.Typed)

	ordinals := []string{"", "1st", "2nd", "3rd", "4th", "5th", "6th"}

	lines := []string{
		hi.Render("race results"),
		"",
	}

	for i, pl := range m.racePlayers {
		ord := ""
		rank := i + 1
		if rank < len(ordinals) {
			ord = ordinals[rank]
		} else {
			ord = fmt.Sprintf("%dth", rank)
		}

		wpmStr := fmt.Sprintf("%.0f wpm", pl.WPM)
		var row string
		if pl.IsUser {
			row = hi.Render(fmt.Sprintf("  %-4s  %-12s  %s", ord, pl.Name, wpmStr)) + hi.Render(" <")
		} else {
			row = dim.Render(fmt.Sprintf("  %-4s  ", ord)) + val.Render(fmt.Sprintf("%-12s", pl.Name)) + dim.Render(fmt.Sprintf("  %s", wpmStr))
		}
		lines = append(lines, row)
	}

	lines = append(lines, "", dim.Render("press any key to continue"))

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

func viewOnlineRaceBar(p theme.Palette, players []game.RacePlayer, barWidth int) string {
	if barWidth < 20 {
		barWidth = 20
	}

	hi := lipgloss.NewStyle().Foreground(p.Accent)
	dim := lipgloss.NewStyle().Foreground(p.Foreground)
	val := lipgloss.NewStyle().Foreground(p.Typed)

	nameWidth := 12
	trackWidth := barWidth - nameWidth - 8
	if trackWidth < 10 {
		trackWidth = 10
	}

	var rows []string
	for _, pl := range players {
		filled := int(pl.Progress * float64(trackWidth))
		if filled > trackWidth {
			filled = trackWidth
		}
		empty := trackWidth - filled
		pct := fmt.Sprintf("%3.0f%%", pl.Progress*100)

		var nameStr, barStr, pctStr string
		if pl.IsUser {
			nameStr = hi.Render(fmt.Sprintf("%-*s", nameWidth, pl.Name))
			barStr = hi.Render(strings.Repeat("━", filled)) + dim.Render(strings.Repeat("─", empty))
			pctStr = val.Render(pct)
		} else {
			nameStr = dim.Render(fmt.Sprintf("%-*s", nameWidth, pl.Name))
			barStr = dim.Render(strings.Repeat("━", filled)) + dim.Render(strings.Repeat("─", empty))
			pctStr = dim.Render(pct)
		}

		rows = append(rows, nameStr+" "+barStr+" "+pctStr)
	}

	return strings.Join(rows, "\n")
}
