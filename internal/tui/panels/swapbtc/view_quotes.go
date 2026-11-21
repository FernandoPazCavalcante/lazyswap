package swapbtc

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

const tableWidth = 66

func (m Model) quotesBody(at time.Time) string {
	quotes := m.orderedQuotes(at)
	lines := []string{fmt.Sprintf("  %s USD · %s (%s) → Bitcoin", m.amount.Value(), m.from.Symbol, m.network), ""}
	quote := m.selectedQuote()
	wide := m.width >= tableWidth
	if wide {
		lines = append(lines, theme.Dim().Render(quoteColumns("Provider", "Est. sats", "Min. sats", "Fee sats", "ETA")))
	}
	for _, route := range quotes {
		lines = append(lines, quoteRow(route, m.selected == route.Provider, wide, at))
		if !wide && m.expanded && route.Provider == m.selected && route.valid(at) {
			lines = append(lines, quoteDetails(route, at, true)...)
		}
	}
	if wide && quote.valid(at) {
		lines = append(lines, "")
		lines = append(lines, quoteDetails(quote, at, false)...)
	}
	if quote.Provider == "" && len(quotes) > 0 {
		quote = quotes[0]
	}
	if quote.Err != nil {
		lines = append(lines, lipgloss.NewStyle().Width(max(1, m.width-4)).Render(quote.Err.Error()))
	}
	return strings.Join(lines, "\n")
}

func quoteColumns(provider, output, minimum, fee, eta string) string {
	return fmt.Sprintf("  %-17s %11s %11s %11s %8s", provider, output, minimum, fee, eta)
}

func quoteRow(route RouteQuote, selected, wide bool, at time.Time) string {
	provider := string(route.Provider)
	var line string
	switch {
	case !route.valid(at):
		line = fmt.Sprintf("  %s · %s", provider, route.status(at))
	case wide:
		line = quoteColumns(provider, formatSats(route.Quote.EstimatedOutputSats),
			btcToSats(route.Quote.MinOutput), btcToSats(route.FeeBTC), quoteETA(route.ETASeconds))
	default:
		line = fmt.Sprintf("  %s · %s sats · %s", provider, formatSats(route.Quote.EstimatedOutputSats), quoteETA(route.ETASeconds))
	}
	if selected {
		return theme.Text().Bold(true).Render("▸" + line[1:])
	}
	return line
}

func quoteDetails(route RouteQuote, at time.Time, full bool) []string {
	q := route.Quote
	var lines []string
	if full {
		lines = append(lines, "  Min. receive (sats): "+btcToSats(q.MinOutput),
			"  Route fee (sats): "+btcToSats(route.FeeBTC)+" · ETA "+quoteETA(route.ETASeconds))
	}
	gas := q.SourceGasBudget
	if gas == "" {
		gas = "unavailable"
	}
	lines = append(lines, "  Est. source gas budget: "+gas)
	if q.NeedsApproval {
		lines = append(lines, "  Token approval gas is additional")
	}
	return append(lines, fmt.Sprintf("  Quote expires in %s", max(0, q.ExpiresAt.Sub(at).Round(time.Second))))
}

func (m Model) reviewBody(at time.Time) string {
	route := m.selectedQuote()
	if !route.valid(at) {
		return "  Quote expired · r: refresh"
	}
	q := route.Quote
	lines := []string{
		"  Review · " + string(route.Provider),
		fmt.Sprintf("  Send: %s %s · %s", q.FromTokenAmount, q.FromToken.Symbol, m.network),
		fmt.Sprintf("  Est. receive: %s sats · Bitcoin", formatSats(q.EstimatedOutputSats)),
		"  Destination:",
	}
	address := q.BTCAddress
	width := max(1, m.width-4)
	for len(address) > width {
		lines = append(lines, "  "+address[:width])
		address = address[width:]
	}
	lines = append(lines, "  "+address)
	return strings.Join(append(lines, quoteDetails(route, at, true)...), "\n")
}

func quoteETA(seconds int) string {
	if seconds <= 0 {
		return "—"
	}
	return fmt.Sprintf("~%dm", (seconds+59)/60)
}

func btcToSats(value string) string {
	if value == "" {
		return "—"
	}
	amount, err := balance.ParseUnits(value, 8)
	if err != nil || !amount.IsInt64() {
		return "—"
	}
	return formatSats(amount.Int64())
}
