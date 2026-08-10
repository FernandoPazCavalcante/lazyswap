package swapoverlay

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/FernandoPazCavalcante/lazyswap/internal/safety"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

// View renders the overlay centred over the available area.
func (m Model) View() string {
	body := m.bodyForStep()
	help := theme.Dim().Render(m.helpForStep())

	box := theme.Border(true).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			theme.Text().Bold(true).Render(m.titleForStep()),
			"",
			body,
			"",
			help,
		))

	if m.width == 0 || m.height == 0 {
		return box
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) titleForStep() string {
	switch m.step {
	case stepFrom:
		return "Swap — choose source token"
	case stepTo:
		return fmt.Sprintf("Swap — choose destination (from %s)", m.selectedFrom.Symbol)
	case stepAmount:
		return fmt.Sprintf("Swap %s → %s — USD amount", m.selectedFrom.Symbol, m.selectedTo.Symbol)
	case stepPreview:
		return fmt.Sprintf("Swap %s → %s — preview", m.selectedFrom.Symbol, m.selectedTo.Symbol)
	case stepExecuting:
		return "Swap — executing"
	case stepDone:
		return "Swap — result"
	}
	return ""
}

func (m Model) bodyForStep() string {
	switch m.step {
	case stepFrom:
		return m.from.View()
	case stepTo:
		return m.to.View()
	case stepAmount:
		return m.amount.View()
	case stepPreview:
		return m.previewBody()
	case stepExecuting:
		return theme.Text().Render(m.execMsg)
	case stepDone:
		return m.doneBody()
	}
	return ""
}

func (m Model) previewBody() string {
	if m.quoteErr != "" {
		return theme.Error().Render("Quote failed: " + m.quoteErr)
	}
	if m.quote == nil {
		return theme.Dim().Render("Loading quote…")
	}
	q := m.quote
	route := "direct — on-chain V2 router, no fee"
	if q.Mode == "api" {
		route = "api — OpenOcean best rate, MEV protected"
	}
	lines := []string{
		fmt.Sprintf("Route          %s", route),
		fmt.Sprintf("USD            %s", q.USDAmountFormatted),
		fmt.Sprintf("From           %s %s   %s", q.FromTokenAmount, q.FromToken.Symbol, q.FromTokenPriceLine),
		fmt.Sprintf("Fee (%.2f%%)    %s %s", q.FeePercent, q.FeeAmount, q.FromToken.Symbol),
		fmt.Sprintf("Net swap input %s %s", q.NetFromTokenAmount, q.FromToken.Symbol),
		fmt.Sprintf("Estimated      %s %s", q.EstimatedOutput, q.ToToken.Symbol),
		fmt.Sprintf("Min received   %s %s   (slippage %.2f%%)", q.MinOutput, q.ToToken.Symbol, q.Slippage),
	}
	if q.PriceImpact != "" {
		lines = append(lines, fmt.Sprintf("Price impact   %s", q.PriceImpact))
	}
	if q.NeedsApproval {
		lines = append(lines, theme.Dim().Render("Note: ERC-20 approval required — will be sent automatically."))
	}
	if q.Mode != "api" {
		lines = append(lines, theme.Dim().Render("t: try API route — best rates across 500+ DEXs (needs LazySwap Pass)"))
	}
	if line := m.riskLine(); line != "" {
		lines = append(lines, "", line)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// riskLine renders the safety verdict for the preview: empty when no check
// applies, dim while pending or unknown, error-styled on HIGH risk.
func (m Model) riskLine() string {
	if m.riskPending {
		return theme.Dim().Render("safety: checking token risk…")
	}
	if m.risk == nil {
		return ""
	}
	s := safety.FormatReport(*m.risk)
	if m.risk.Unknown {
		return theme.Dim().Render(s)
	}
	if m.risk.Level == safety.LevelHigh {
		return theme.Error().Render(s)
	}
	return theme.Text().Render(s)
}

func (m Model) doneBody() string {
	if m.execRes == nil {
		return ""
	}
	return swap.FormatSwapResult(*m.execRes)
}

func (m Model) helpForStep() string {
	switch m.step {
	case stepFrom, stepTo:
		return "↑/↓ navigate  ·  enter: select  ·  esc: back"
	case stepAmount:
		return "enter: get quote  ·  esc: back"
	case stepPreview:
		return "y / enter: execute  ·  t: toggle route  ·  n / esc: back"
	case stepExecuting:
		return "waiting for receipt…"
	case stepDone:
		return "any key to close"
	}
	return ""
}
