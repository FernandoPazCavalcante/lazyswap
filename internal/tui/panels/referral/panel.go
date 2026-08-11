// Package referral implements the right-panel "Referral" tab (tab 2).
//
// Display-only: it shows the wallet's referral code and earnings, pushed by
// the parent screen after an async API fetch. Claiming happens via the CLI
// (`lazyswap referral claim`) — this panel never moves money.
package referral

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FernandoPazCavalcante/lazyswap/internal/api"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

// Model owns the referral-tab state. All inputs are pushed by the parent.
type Model struct {
	loading bool
	loaded  bool
	stats   api.ReferralStats
	errMsg  string

	focused       bool
	width, height int
}

// New builds an empty referral panel (parent populates it on activation).
func New() Model { return Model{} }

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// SetSize stores the outer dimensions.
func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

// SetFocused toggles the focused border.
func (m *Model) SetFocused(b bool) { m.focused = b }

// SetLoading marks a fetch as in-flight.
func (m *Model) SetLoading() {
	m.loading = true
	m.errMsg = ""
}

// SetStats records freshly fetched referral stats.
func (m *Model) SetStats(s api.ReferralStats) {
	m.stats = s
	m.loaded = true
	m.loading = false
	m.errMsg = ""
}

// SetError records a failed fetch.
func (m *Model) SetError(s string) {
	m.errMsg = s
	m.loading = false
}

// Update is a no-op: the panel is display-only.
func (m Model) Update(_ tea.Msg) (Model, tea.Cmd) { return m, nil }

// View renders the referral dashboard.
func (m Model) View() string {
	body := m.body()
	return theme.Border(m.focused).
		Width(maxInt(m.width-2, 20)).
		Height(maxInt(m.height-2, 3)).
		Padding(0, 1).
		Render(body)
}

func (m Model) body() string {
	switch {
	case m.errMsg != "":
		return theme.Error().Render("referral: " + m.errMsg)
	case m.loading:
		return theme.Dim().Render("Loading referral stats…")
	case !m.loaded:
		return theme.Dim().Render("Referral stats load when the API is reachable\n(needs a LazySwap Pass to authenticate).")
	}

	s := m.stats
	code := s.Code
	if code == "" {
		code = "(none yet — run `lazyswap referral code`)"
	}
	rows := []string{
		theme.Text().Render("Referral program"),
		"",
		fmt.Sprintf("Code        %s", code),
		fmt.Sprintf("Referred    %d wallet(s), %d swap(s)", s.TotalReferred, s.TotalSwaps),
		fmt.Sprintf("Earned      $%.2f   (claimed $%.2f)", s.TotalEarned, s.TotalClaimed),
		fmt.Sprintf("Claimable   $%.2f   (minimum $%.0f)", s.Claimable, s.MinClaim),
		"",
	}
	if s.CanClaim {
		rows = append(rows, theme.Text().Render("Claim ready — run `lazyswap referral claim`"))
	} else {
		rows = append(rows, theme.Dim().Render("Earnings accrue from settled swaps by referred wallets."))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
