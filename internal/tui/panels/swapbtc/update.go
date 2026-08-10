package swapbtc

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Update advances the form / preview state machine.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case EstimateResultMsg:
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
		if msg.Err != nil {
			m.quote = nil
			m.quoteErr = msg.Err.Error()
			m.state = stateForm
			m.field = fieldAddr
			m.addr.Focus()
			return m, textinput.Blink
		}
		q := msg.Quote
		m.quote = &q
		m.quoteErr = ""
		m.state = statePreview
		m.countdown = refreshSeconds
		return m, tick()

	case ExecutionResultMsg:
		r := msg.Result
		m.execRes = &r
		m.state = stateDone
		return m, nil

	case TickMsg:
		if m.state != statePreview {
			return m, nil
		}
		m.countdown--
		if m.countdown <= 0 {
			m.state = stateQuoting
			return m, m.requestQuote()
		}
		return m, tick()

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

	case statePreview:
		switch strings.ToLower(k.String()) {
		case "y", "enter":
			if m.quoteErr != "" || m.quote == nil {
				return m, nil
			}
			m.state = stateExecuting
			return m, m.requestExecute()
		case "n":
			m.state = stateForm
			m.field = fieldAddr
			m.addr.Focus()
			return m, textinput.Blink
		}

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
	case statePreview, stateDone:
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
			if m.estimate != nil && m.estimate.ThorBelowMin {
				m.quoteErr = fmt.Sprintf("Amount below THORChain minimum: need ≥ %s %s (≈ %s).",
					m.estimate.ThorMinTokenAmount, m.estimate.FromToken.Symbol, m.estimate.ThorMinUSD)
				return m, nil
			}
			if !m.inputsValid() {
				m.quoteErr = "Enter a positive USD amount and a valid BTC address."
				return m, nil
			}
			m.addr.Blur()
			m.state = stateQuoting
			m.quoteErr = ""
			return m, m.requestQuote()
		}
		var cmd tea.Cmd
		m.addr, cmd = m.addr.Update(k)
		return m, cmd
	}
	return m, nil
}

// ─── intent helpers ────────────────────────────────────────────────────────────

func (m *Model) requestQuote() tea.Cmd {
	it, ok := m.tokens.SelectedItem().(Item)
	if !ok {
		return nil
	}
	m.from = it.toTokenInfo()
	from := m.from
	usd := strings.TrimSpace(m.amount.Value())
	btc := strings.TrimSpace(m.addr.Value())
	return func() tea.Msg {
		return QuoteRequestMsg{From: from, USDAmount: usd, BTCAddress: btc}
	}
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
	m.estimating = true
	m.estimateErr = ""
	return func() tea.Msg {
		return EstimateRequestMsg{From: from, USDAmount: usd}
	}
}

func (m *Model) requestExecute() tea.Cmd {
	usd := strings.TrimSpace(m.amount.Value())
	btc := strings.TrimSpace(m.addr.Value())
	from := m.from
	return func() tea.Msg {
		return ExecuteRequestMsg{From: from, USDAmount: usd, BTCAddress: btc}
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
	m.quote = nil
	m.quoteErr = ""
	m.execRes = nil
	m.countdown = 0
	m.estimate = nil
	m.estimateErr = ""
	m.estimating = false
	m.amount.Blur()
	m.addr.Blur()
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return TickMsg{} })
}
