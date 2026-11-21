package swapbtc

import (
	"cmp"
	"errors"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (r RouteQuote) valid(at time.Time) bool {
	return r.Err == nil && r.Quote.EstimatedOutputSats > 0 && at.Before(r.Quote.ExpiresAt)
}

func (r RouteQuote) pending() bool {
	return r.Err == nil && r.Quote.ExpiresAt.IsZero()
}

func (r RouteQuote) status(at time.Time) string {
	switch {
	case r.Err != nil:
		return "Unavailable"
	case r.pending():
		return "Quoting…"
	case !at.Before(r.Quote.ExpiresAt):
		return "Expired"
	default:
		return ""
	}
}

func (m Model) orderedQuotes(at time.Time) []RouteQuote {
	quotes := slices.Clone(m.quotes)
	slices.SortStableFunc(quotes, func(a, b RouteQuote) int {
		switch {
		case a.valid(at) && b.valid(at):
			return cmp.Compare(b.Quote.EstimatedOutputSats, a.Quote.EstimatedOutputSats)
		case a.valid(at):
			return -1
		case b.valid(at):
			return 1
		default:
			return 0
		}
	})
	return quotes
}

func (m Model) selectedQuote() RouteQuote {
	for _, quote := range m.quotes {
		if quote.Provider == m.selected {
			return quote
		}
	}
	return RouteQuote{}
}

func (m Model) onQuote(msg QuoteResultMsg, at time.Time) (Model, tea.Cmd) {
	if !m.ShowingQuotes() || m.requestID != msg.ID {
		return m, nil
	}
	route := msg.Route
	if route.Err == nil && route.Quote.EstimatedOutputSats <= 0 {
		route.Err = errors.New("provider returned no BTC amount")
	}
	if route.Err == nil && route.Quote.ExpiresAt.IsZero() {
		route.Quote.ExpiresAt = at.Add(refreshSeconds * time.Second)
	}
	index := slices.IndexFunc(m.quotes, func(q RouteQuote) bool { return q.Provider == route.Provider })
	if index < 0 || !m.quotes[index].pending() {
		return m, nil
	}
	m.quotes = slices.Clone(m.quotes)
	m.quotes[index] = route
	if m.selected == "" && route.valid(at) {
		m.selected, m.expanded = route.Provider, true
	}
	if !m.ticking && route.valid(at) {
		m.ticking = true
		return m, quoteTick(m.requestID)
	}
	return m, nil
}

func (m Model) onQuoteTick(msg TickMsg) (Model, tea.Cmd) {
	if !m.ShowingQuotes() || m.requestID != msg.ID {
		return m, nil
	}
	if m.state == stateReview && !m.selectedQuote().valid(msg.At) {
		m.state = stateQuotes
	}
	for _, route := range m.quotes {
		if route.valid(msg.At) {
			return m, quoteTick(m.requestID)
		}
	}
	m.ticking = false
	return m, nil
}

func quoteTick(id uint64) tea.Cmd {
	return tea.Tick(time.Second, func(at time.Time) tea.Msg { return TickMsg{ID: id, At: at} })
}

func (m Model) handleQuoteKey(key string, at time.Time) (Model, tea.Cmd) {
	switch key {
	case "r":
		for _, route := range m.quotes {
			if route.pending() {
				return m, nil
			}
		}
		return m.beginQuotes()
	case "up", "k":
		return m.moveQuote(-1, at), nil
	case "down", "j":
		return m.moveQuote(1, at), nil
	case " ":
		m.expanded = !m.expanded
	case "enter":
		if m.selectedQuote().valid(at) {
			m.state = stateReview
		}
	}
	return m, nil
}

func (m Model) moveQuote(delta int, at time.Time) Model {
	quotes := m.orderedQuotes(at)
	if len(quotes) == 0 {
		return m
	}
	index := slices.IndexFunc(quotes, func(q RouteQuote) bool { return q.Provider == m.selected })
	m.selected = quotes[clampInt(index+delta, 0, len(quotes)-1)].Provider
	m.expanded = true
	return m
}

func (m Model) handleReviewKey(key string, at time.Time) (Model, tea.Cmd) {
	route := m.selectedQuote()
	if key == "r" {
		return m.beginQuotes()
	}
	if key == "n" || !route.valid(at) {
		m.state = stateQuotes
		return m, nil
	}
	if key == "y" {
		m.state = stateExecuting
		msg := ExecuteRequestMsg{ID: m.requestID, Provider: route.Provider, Quote: route.Quote}
		return m, func() tea.Msg { return msg }
	}
	return m, nil
}
