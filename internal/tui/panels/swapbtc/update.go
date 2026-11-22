package swapbtc

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Update advances the form, quote comparison, and review.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
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
	if len(m.providers) == 0 {
		return m, nil
	}
	if k.Type == tea.KeyEsc {
		return m.handleEsc()
	}

	switch m.state {
	case stateForm:
		return m.handleFormKey(k)

	case stateQuotes:
		return m.handleQuoteKey(k.String(), time.Now())
	case stateAddress:
		return m.handleAddressKey(k)
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
			m.field = fieldToken
			m.quoteErr = ""
		}
	case stateQuotes:
		return m.editQuote()
	case stateAddress:
		m.addr.Blur()
		m.quoteErr = ""
		m.state = stateQuotes
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
			amount, err := strconv.ParseFloat(strings.TrimSpace(m.amount.Value()), 64)
			if err != nil || !(amount > 0 && amount <= math.MaxFloat64) {
				m.quoteErr = "Enter a positive USD amount."
				return m, nil
			}
			m.amount.Blur()
			return m.beginQuotes()
		}
		m.quoteErr = ""
		var cmd tea.Cmd
		m.amount, cmd = m.amount.Update(k)
		return m, cmd
	}
	return m, nil
}

func (m Model) editAddress() (Model, tea.Cmd) {
	m.state = stateAddress
	m.addr.SetValue(m.destination)
	cmd := m.addr.Focus()
	return m, cmd
}

func (m Model) handleAddressKey(k tea.KeyMsg) (Model, tea.Cmd) {
	m.quoteErr = ""
	if k.Type != tea.KeyEnter {
		var cmd tea.Cmd
		m.addr, cmd = m.addr.Update(k)
		return m, cmd
	}
	address := strings.TrimSpace(m.addr.Value())
	if address != "" && len(address) < minBTCAddrLen {
		m.quoteErr = "Enter a valid BTC address."
		return m, nil
	}
	m.addr.Blur()
	if address == m.destination {
		m.state = stateQuotes
		return m, nil
	}
	m.destination = address
	return m.beginQuotes()
}

// ─── intent helpers ────────────────────────────────────────────────────────────

func (m Model) beginQuotes() (Model, tea.Cmd) {
	if m.state == stateForm {
		it, ok := m.tokens.SelectedItem().(Item)
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
		USDAmount: strings.TrimSpace(m.amount.Value()), BTCAddress: m.destination}
	return m, func() tea.Msg { return request }
}

func (m *Model) reset() {
	m.state = stateForm
	m.field = fieldToken
	m.requestID++
	m.destination = ""
	m.quotes = nil
	m.selected = ""
	m.expanded = false
	m.ticking = false
	m.quoteErr = ""
	m.execRes = nil
	m.amount.Blur()
	m.addr.Blur()
}

func (m Model) editQuote() (Model, tea.Cmd) {
	destination := m.destination
	m.reset()
	m.destination = destination
	m.field = fieldAmount
	m.amount.Focus()
	return m, textinput.Blink
}
