package swapbtc

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

// View renders the bordered panel for the current state.
func (m Model) View() string {
	body := m.bodyForState()
	help := theme.Dim().Render(m.helpForState())
	titled := lipgloss.JoinVertical(lipgloss.Left,
		theme.Text().Bold(true).Padding(0, 1).Render("Swap BTC"),
		"",
		body,
		"",
		help,
	)
	w, h := m.width-2, m.height-2
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	box := theme.Border(m.focused).Width(w).Height(h).Render(titled)
	// Hard-cap to the allotted footprint so a tall form can never overflow and
	// push the tab header / wallet panel off-screen.
	return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(box)
}

func (m Model) bodyForState() string {
	if len(m.providers) == 0 {
		return "  No active BTC swap providers."
	}
	switch m.state {
	case stateForm:
		return m.formBody()
	case stateQuotes:
		return m.quotesBody(time.Now())
	case stateAddress:
		return m.addressBody(time.Now())
	case stateReview:
		return m.reviewBody(time.Now())
	case stateExecuting:
		return theme.Text().Render("  Submitting transaction…")
	case stateDone:
		if m.execRes != nil {
			return swap.FormatSwapResult(*m.execRes)
		}
	}
	return ""
}

func (m Model) formBody() string {
	amountLabel := "  Amount (USD $):"
	if m.field == fieldAmount {
		amountLabel = theme.Text().Bold(true).Render("> Amount (USD $):")
	}
	rows := []string{m.tokens.View(), "", amountLabel, "  " + m.amount.View()}
	return m.formWithError(rows)
}

func (m Model) addressBody(at time.Time) string {
	rows := []string{theme.Text().Bold(true).Render("> Destination BTC address:"), "  " + m.addr.View()}
	if route := m.selectedQuote(); route.Provider != "" {
		rows = append(rows, "", quoteRow(route, false, false, at))
	}
	return m.formWithError(rows)
}

func (m Model) formWithError(rows []string) string {
	if m.quoteErr != "" {
		rows = append(rows, "", theme.Error().Render("  "+m.quoteErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) helpForState() string {
	if len(m.providers) == 0 {
		return ""
	}
	switch m.state {
	case stateForm:
		switch m.field {
		case fieldToken:
			return "↑/↓ select token  ·  enter: next  ·  esc: back"
		default:
			return "Enter get quotes · Esc back"
		}
	case stateQuotes:
		action := "Enter review"
		if m.destination == "" {
			action = "Enter add address"
		}
		if m.width >= tableWidth {
			return "j/k · " + action + " · a address · r refresh · Esc edit"
		}
		return "j/k · Space details · r refresh\n" + action + " · a address · Esc edit"
	case stateAddress:
		return "Enter get quotes · Esc back"
	case stateReview:
		return "y confirm swap · a address · r refresh · Esc"
	case stateExecuting:
		return "waiting for receipt…"
	case stateDone:
		return "any key to start another"
	}
	return ""
}

// formatSats renders a sats amount with thousands separators, e.g. 72145 →
// "72,145".
func formatSats(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
