package swapbtc

import (
	"fmt"
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
	switch m.state {
	case stateForm:
		return m.formBody()
	case stateQuotes:
		return m.quotesBody(time.Now())
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
	addrLabel := "  Destination BTC address:"
	if m.field == fieldAmount {
		amountLabel = theme.Text().Bold(true).Render("> Amount (USD $):")
	}
	if m.field == fieldAddr {
		addrLabel = theme.Text().Bold(true).Render("> Destination BTC address:")
	}
	rows := []string{
		m.tokens.View(),
		"",
		amountLabel,
		"  " + m.amount.View(),
		m.estimateLine(),
		addrLabel,
		"  " + m.addr.View(),
	}
	if m.quoteErr != "" {
		rows = append(rows, "", theme.Error().Render("  "+m.quoteErr))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// estimateLine renders the live BTC/sats price preview under the amount field,
// or the THORChain minimum disclaimer when the amount is too low. Always returns
// one row so the form height stays stable.
func (m Model) estimateLine() string {
	switch {
	case m.estimating:
		return theme.Dim().Render("  estimating…")
	case m.estimate != nil && m.estimate.ThorBelowMin:
		return theme.Error().Render(fmt.Sprintf("  ⚠ Below THORChain minimum — need ≥ %s %s (≈ %s)",
			m.estimate.ThorMinTokenAmount, m.estimate.FromToken.Symbol, m.estimate.ThorMinUSD))
	case m.estimateErr != "":
		return theme.Dim().Render("  (estimate unavailable)")
	case m.estimate != nil:
		s := theme.Text().Render(fmt.Sprintf("  THORChain ≈ %s BTC (%s sats)",
			m.estimate.EstimatedOutput, formatSats(m.estimate.EstimatedOutputSats)))
		if m.estimate.ThorMinTokenAmount != "" {
			s += theme.Dim().Render(fmt.Sprintf("   · min %s %s",
				m.estimate.ThorMinTokenAmount, m.estimate.FromToken.Symbol))
		}
		return s
	default:
		return ""
	}
}

func (m Model) helpForState() string {
	switch m.state {
	case stateForm:
		switch m.field {
		case fieldToken:
			return "↑/↓ select token  ·  enter: next  ·  esc: back"
		default:
			return "type value  ·  enter: next/quote  ·  esc: back"
		}
	case stateQuotes:
		if m.width >= tableWidth {
			return "j/k select · Enter review · r refresh · Esc edit"
		}
		return "j/k · Space details · Enter review · r refresh · Esc"
	case stateReview:
		return "y confirm swap · r refresh · Esc quotes"
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
