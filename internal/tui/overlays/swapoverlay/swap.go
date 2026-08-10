// Package swapoverlay implements the modal swap flow.
//
// State machine: stepFrom → stepTo → stepAmount → stepPreview → stepExecuting
// → stepDone. ESC steps back; from stepFrom, ESC closes the overlay.
//
// Mirrors src/tui/panels/swap-panel.ts but greatly simplified: no THORchain,
// no custom-address entry (recommended + owned only). The parent screen owns
// the quoting / execution commands; this overlay only renders state and emits
// intent messages.
package swapoverlay

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/safety"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

// ─── Public messages ─────────────────────────────────────────────────────────

// CancelMsg signals the parent to close the overlay.
type CancelMsg struct{}

// QuoteRequestMsg is emitted when the user finalises from/to/amount.
// The parent should run a Flow.Quote and send back QuoteResultMsg.
type QuoteRequestMsg struct {
	From      swap.TokenInfo
	To        swap.TokenInfo
	USDAmount string
}

// ExecuteRequestMsg is emitted when the user confirms the preview.
// The parent should run Flow.Execute and send back ExecutionResultMsg.
type ExecuteRequestMsg struct {
	From      swap.TokenInfo
	To        swap.TokenInfo
	USDAmount string
}

// ModeToggleMsg asks the parent to flip the swap route (direct ↔ api) and
// requote the same intent.
type ModeToggleMsg struct {
	From      swap.TokenInfo
	To        swap.TokenInfo
	USDAmount string
}

// ─── Input messages (sent by parent) ─────────────────────────────────────────

// QuoteResultMsg carries the result of a QuoteRequestMsg. SafetyPending tells
// the overlay a SafetyResultMsg will follow (render "checking…" meanwhile).
type QuoteResultMsg struct {
	Quote         swap.FlowQuote
	Err           error
	SafetyPending bool
}

// SafetyResultMsg carries the async token risk report for the preview step.
type SafetyResultMsg struct {
	Report safety.Report
}

// ExecutionResultMsg carries the result of an ExecuteRequestMsg.
type ExecutionResultMsg struct {
	Result swap.FlowResult
}

// ─── State ───────────────────────────────────────────────────────────────────

type step int

const (
	stepFrom step = iota
	stepTo
	stepAmount
	stepPreview
	stepExecuting
	stepDone
)

// Model is the overlay state.
type Model struct {
	from, to      list.Model
	amount        textinput.Model
	step          step
	width, height int

	selectedFrom Item
	selectedTo   Item
	usdAmount    string

	quote    *swap.FlowQuote
	quoteErr string
	execMsg  string // status banner during stepExecuting
	execRes  *swap.FlowResult

	risk        *safety.Report
	riskPending bool
}

// New builds the overlay. balances should be the current wallet's tokens;
// c provides the recommended-token list + native metadata for the to-side.
func New(balances []balance.TokenBalance, c chain.Config) Model {
	fromItems := buildFromItems(balances)
	toItems := buildToItems(balances, c)

	ti := textinput.New()
	ti.Placeholder = "USD amount (e.g. 50)"
	ti.Prompt = "$ "
	ti.CharLimit = 16
	ti.Width = 24

	return Model{
		from:   newList("Swap from", fromItems),
		to:     newList("Swap to", toItems),
		amount: ti,
		step:   stepFrom,
	}
}

// Init kicks off the text-input cursor when the amount step is reached.
func (m Model) Init() tea.Cmd { return nil }

// SetSize lays out the inner widgets.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	listW := clampInt(w-8, 30, 80)
	listH := clampInt(h-12, 6, 18)
	m.from.SetSize(listW, listH)
	m.to.SetSize(listW, listH)
	m.amount.Width = clampInt(w-12, 12, 32)
}

// Update advances the state machine.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case QuoteResultMsg:
		if msg.Err != nil {
			m.quoteErr = msg.Err.Error()
			m.quote = nil
		} else {
			m.quote = &msg.Quote
			m.quoteErr = ""
		}
		m.risk = nil
		m.riskPending = msg.Err == nil && msg.SafetyPending
		m.step = stepPreview
		return m, nil

	case SafetyResultMsg:
		r := msg.Report
		m.risk = &r
		m.riskPending = false
		return m, nil

	case ExecutionResultMsg:
		r := msg.Result
		m.execRes = &r
		m.step = stepDone
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey routes a keypress to the current step's handler.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Global ESC: step back, or cancel from the first step.
	if msg.Type == tea.KeyEsc {
		return m.stepBack()
	}

	switch m.step {
	case stepFrom:
		return m.handleFromStep(msg)
	case stepTo:
		return m.handleToStep(msg)
	case stepAmount:
		return m.handleAmountStep(msg)
	case stepPreview:
		return m.handlePreviewStep(msg)
	case stepExecuting:
		// Block all input while a tx is in flight; only ESC exits via stepBack.
		return m, nil
	case stepDone:
		// Any key dismisses.
		return m, func() tea.Msg { return CancelMsg{} }
	}
	return m, nil
}

func (m Model) handleFromStep(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		if it, ok := m.from.SelectedItem().(Item); ok {
			m.selectedFrom = it
			m.step = stepTo
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.from, cmd = m.from.Update(msg)
	return m, cmd
}

func (m Model) handleToStep(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		if it, ok := m.to.SelectedItem().(Item); ok {
			if strings.EqualFold(it.Address, m.selectedFrom.Address) {
				// Refuse same-token swap; keep cursor on the to list.
				return m, nil
			}
			m.selectedTo = it
			m.step = stepAmount
			m.amount.Focus()
			return m, textinput.Blink
		}
	}
	var cmd tea.Cmd
	m.to, cmd = m.to.Update(msg)
	return m, cmd
}

func (m Model) handleAmountStep(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		v := strings.TrimSpace(m.amount.Value())
		if v == "" {
			return m, nil
		}
		m.usdAmount = v
		m.amount.Blur()
		return m, func() tea.Msg {
			return QuoteRequestMsg{
				From:      m.selectedFrom.toTokenInfo(),
				To:        m.selectedTo.toTokenInfo(),
				USDAmount: v,
			}
		}
	}
	var cmd tea.Cmd
	m.amount, cmd = m.amount.Update(msg)
	return m, cmd
}

func (m Model) handlePreviewStep(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y", "enter":
		if m.quoteErr != "" {
			return m, nil
		}
		m.step = stepExecuting
		m.execMsg = "Submitting transaction…"
		return m, func() tea.Msg {
			return ExecuteRequestMsg{
				From:      m.selectedFrom.toTokenInfo(),
				To:        m.selectedTo.toTokenInfo(),
				USDAmount: m.usdAmount,
			}
		}
	case "n":
		return m.stepBack()
	case "t":
		// Requote via the other route; show the loading state meanwhile.
		m.quote = nil
		m.quoteErr = ""
		m.risk = nil
		m.riskPending = false
		return m, func() tea.Msg {
			return ModeToggleMsg{
				From:      m.selectedFrom.toTokenInfo(),
				To:        m.selectedTo.toTokenInfo(),
				USDAmount: m.usdAmount,
			}
		}
	}
	return m, nil
}

// stepBack rewinds one step, or emits CancelMsg if we're already at stepFrom.
func (m Model) stepBack() (Model, tea.Cmd) {
	switch m.step {
	case stepFrom, stepExecuting:
		return m, func() tea.Msg { return CancelMsg{} }
	case stepTo:
		m.step = stepFrom
	case stepAmount:
		m.amount.Blur()
		m.step = stepTo
	case stepPreview:
		m.step = stepAmount
		m.amount.Focus()
		m.quote = nil
		m.quoteErr = ""
		return m, textinput.Blink
	case stepDone:
		return m, func() tea.Msg { return CancelMsg{} }
	}
	return m, nil
}
