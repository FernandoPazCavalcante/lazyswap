package swapbtc

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Update advances the form, quote comparison, and review.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case EstimateResultMsg:
		if msg.ID != m.estimateID || m.state != stateForm || m.field != fieldAddr {
			return m, nil
		}
		m.estimating = false
		if msg.Err != nil {
			m.estimate = nil
			m.estimateErr = msg.Err.Error()
			return m, nil
		}
		q := msg.Quote
		m.estimate = &q
		m.estimateErr = ""
		return m, nil

	case QuoteResultMsg:
		return m.onQuote(msg, time.Now())

	case ExecutionResultMsg:
		if !m.IsExecuting(msg.ID) {
			return m, nil
		}
		r := msg.Result
		m.execRes = &r
		m.state = stateDone
		return m, nil

	case TickMsg:
		return m.onQuoteTick(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(k tea.KeyMsg) (Model, tea.Cmd) {
	if k.Type == tea.KeyEsc {
		return m.handleEsc()
	}

	switch m.state {
	case stateForm:
		return m.handleFormKey(k)

	case stateQuotes:
		return m.handleQuoteKey(k.String(), time.Now())
	case stateReview:
		return m.handleReviewKey(strings.ToLower(k.String()), time.Now())

	case stateExecuting:
		// Block input while the tx is in flight.

	case stateDone:
		// Any key resets the form for another swap.
		m.reset()
		return m, nil
	}
	return m, nil
}

func (m Model) handleEsc() (Model, tea.Cmd) {
	switch m.state {
	case stateForm:
		if m.field != fieldToken {
			m.amount.Blur()
			m.addr.Blur()
			m.field = fieldToken
		}
	case stateQuotes:
		return m.editQuote()
	case stateReview:
		m.state = stateQuotes
	case stateDone:
		m.reset()
	}
	return m, nil
}

func (m Model) handleFormKey(k tea.KeyMsg) (Model, tea.Cmd) {
	switch m.field {
	case fieldToken:
		if k.Type == tea.KeyEnter {
			if _, ok := m.tokens.SelectedItem().(Item); ok {
				m.field = fieldAmount
				m.amount.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.tokens, cmd = m.tokens.Update(k)
		return m, cmd

	case fieldAmount:
		if k.Type == tea.KeyEnter {
			m.amount.Blur()
			m.field = fieldAddr
			m.addr.Focus()
			// Fire a destination-less estimate so the BTC/sats preview shows
			// before the user types an address.
			return m, tea.Batch(textinput.Blink, m.requestEstimate())
		}
		// A keystroke changes the amount — drop the now-stale estimate.
		m.estimate = nil
		m.estimateErr = ""
		var cmd tea.Cmd
		m.amount, cmd = m.amount.Update(k)
		return m, cmd

	case fieldAddr:
		if k.Type == tea.KeyEnter {
			if !m.inputsValid() {
				m.quoteErr = "Enter a positive USD amount and a valid BTC address."
				return m, nil
			}
			m.addr.Blur()
			return m.beginQuotes()
		}
		var cmd tea.Cmd
		m.addr, cmd = m.addr.Update(k)
		return m, cmd
	}
	return m, nil
}

// ─── intent helpers ────────────────────────────────────────────────────────────

func (m Model) beginQuotes() (Model, tea.Cmd) {
	it, ok := m.tokens.SelectedItem().(Item)
	if !m.ShowingQuotes() {
		if !ok {
			return m, nil
		}
		m.from = it.toTokenInfo()
	}
	m.state = stateQuotes
	m.quoteErr = ""
	m.requestID++
	m.ticking = false
	m.quotes = make([]RouteQuote, 0, len(m.providers))
	for _, provider := range m.providers {
		m.quotes = append(m.quotes, RouteQuote{Provider: provider})
	}
	request := QuoteRequestMsg{ID: m.requestID, From: m.from,
		USDAmount: strings.TrimSpace(m.amount.Value()), BTCAddress: strings.TrimSpace(m.addr.Value())}
	return m, func() tea.Msg { return request }
}

func (m *Model) requestEstimate() tea.Cmd {
	it, ok := m.tokens.SelectedItem().(Item)
	if !ok {
		return nil
	}
	usd := strings.TrimSpace(m.amount.Value())
	if v, err := strconv.ParseFloat(usd, 64); err != nil || v <= 0 {
		return nil
	}
	m.from = it.toTokenInfo()
	from := m.from
	m.estimateID++
	id := m.estimateID
	m.estimating = true
	m.estimateErr = ""
	return func() tea.Msg {
		return EstimateRequestMsg{ID: id, From: from, USDAmount: usd}
	}
}

func (m Model) inputsValid() bool {
	usd, err := strconv.ParseFloat(strings.TrimSpace(m.amount.Value()), 64)
	if err != nil || usd <= 0 {
		return false
	}
	return len(strings.TrimSpace(m.addr.Value())) >= minBTCAddrLen
}

func (m *Model) reset() {
	m.state = stateForm
	m.field = fieldToken
	m.requestID++
	m.estimateID++
	m.quotes = nil
	m.selected = ""
	m.expanded = false
	m.ticking = false
	m.quoteErr = ""
	m.execRes = nil
	m.estimate = nil
	m.estimateErr = ""
	m.estimating = false
	m.amount.Blur()
	m.addr.Blur()
}

func (m Model) editQuote() (Model, tea.Cmd) {
	m.reset()
	m.field = fieldAmount
	m.amount.Focus()
	return m, textinput.Blink
}
