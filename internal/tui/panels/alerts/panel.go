// Package alerts implements the right-panel "Alerts" tab (tab 3): the token
// price monitor watchlist. The panel is a thin view over the backend monitor
// API — the parent screen runs every request and pushes results back in.
//
// Keys (when focused): j/k move, x remove, t Telegram link, R register.
// Tokens are added from the Tokens tab with 'a'.
package alerts

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FernandoPazCavalcante/lazyswap/internal/api"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

// RegisterRequestMsg asks the parent to create a monitor account.
type RegisterRequestMsg struct{}

// RemoveRequestMsg asks the parent to delete a watchlist item.
type RemoveRequestMsg struct{ ID string }

// TelegramRequestMsg asks the parent to start Telegram linking.
type TelegramRequestMsg struct{}

// Model owns the alerts-tab state. All data is pushed by the parent.
type Model struct {
	registered bool
	loading    bool
	loaded     bool
	items      []api.WatchItem
	cursor     int
	link       *api.TelegramLink
	errMsg     string
	notice     string

	focused       bool
	width, height int
}

// New builds an empty alerts panel (parent populates it on activation).
func New() Model { return Model{} }

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// SetSize stores the outer dimensions.
func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

// SetFocused toggles the focused border.
func (m *Model) SetFocused(b bool) { m.focused = b }

// SetRegistered records whether a monitor API key is held.
func (m *Model) SetRegistered(b bool) { m.registered = b }

// SetLoading marks a fetch as in-flight.
func (m *Model) SetLoading() {
	m.loading = true
	m.errMsg = ""
}

// SetItems records a freshly fetched watchlist.
func (m *Model) SetItems(items []api.WatchItem) {
	m.items = items
	m.loaded = true
	m.loading = false
	m.errMsg = ""
	if m.cursor >= len(items) {
		m.cursor = maxInt(len(items)-1, 0)
	}
}

// SetLink records a Telegram linking code to display.
func (m *Model) SetLink(l api.TelegramLink) {
	m.link = &l
	m.loading = false
	m.errMsg = ""
}

// SetNotice shows a transient confirmation line.
func (m *Model) SetNotice(s string) {
	m.notice = s
	m.loading = false
}

// SetError records a failed request.
func (m *Model) SetError(s string) {
	m.errMsg = s
	m.loading = false
}

// Update handles navigation and emits request messages for the parent.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "R":
		if !m.registered {
			return m, func() tea.Msg { return RegisterRequestMsg{} }
		}
	case "x":
		if m.registered && m.cursor < len(m.items) {
			id := m.items[m.cursor].ID
			return m, func() tea.Msg { return RemoveRequestMsg{ID: id} }
		}
	case "t":
		if m.registered {
			return m, func() tea.Msg { return TelegramRequestMsg{} }
		}
	}
	return m, nil
}

// View renders the watchlist.
func (m Model) View() string {
	return theme.Border(m.focused).
		Width(maxInt(m.width-2, 20)).
		Height(maxInt(m.height-2, 3)).
		Padding(0, 1).
		Render(m.body())
}

func (m Model) body() string {
	switch {
	case !m.registered:
		return lipgloss.JoinVertical(lipgloss.Left,
			theme.Text().Render("Price alerts"),
			"",
			theme.Dim().Render("Not registered."),
			theme.Dim().Render("Press R to create a (free) monitor account."),
		)
	case m.loading:
		return theme.Dim().Render("Loading watchlist…")
	case m.errMsg != "":
		return theme.Error().Render("alerts: " + m.errMsg)
	}

	rows := []string{theme.Text().Render("Price alerts"), ""}
	if len(m.items) == 0 && m.loaded {
		rows = append(rows, theme.Dim().Render("Watchlist empty — press 'a' on a token in the Tokens tab."))
	}
	for i, it := range m.items {
		line := fmt.Sprintf("%s  ±%.1f%%  %s", it.TokenSymbol, it.ThresholdPct, it.ChainKey)
		if i == m.cursor {
			rows = append(rows, theme.Text().Render("▸ "+line))
		} else {
			rows = append(rows, theme.Dim().Render("  "+line))
		}
	}
	rows = append(rows, "")
	if m.link != nil {
		rows = append(rows,
			theme.Text().Render(fmt.Sprintf("Telegram code: %s", m.link.Code)),
			theme.Dim().Render(m.link.DeepLink),
		)
	}
	if m.notice != "" {
		rows = append(rows, theme.Text().Render(m.notice))
	}
	rows = append(rows, theme.Dim().Render("x remove · t link Telegram"))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
