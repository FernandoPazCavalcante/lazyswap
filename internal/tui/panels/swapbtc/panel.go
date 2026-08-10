// Package swapbtc implements the right-panel "Swap BTC" tab (tab 5): a
// cross-chain EVM → BTC swap form routed through THORchain.
//
// Mirrors src/tui/panels/swap-tab.ts. The form has three sub-fields — source
// token (list), USD amount, destination BTC address. Submitting emits a
// QuoteRequestMsg; the parent runs Flow.GetThorchainQuote and feeds back a
// QuoteResultMsg. A live 10s countdown re-quotes while the preview is shown.
// Confirming the preview emits ExecuteRequestMsg.
package swapbtc

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

// refreshSeconds is the quote-refresh countdown while the preview is shown.
const refreshSeconds = 10

// minBTCAddrLen is the shortest plausible Bitcoin address (matches swap-tab.ts).
const minBTCAddrLen = 25

// ─── State ───────────────────────────────────────────────────────────────────

type state int

const (
	stateForm state = iota
	stateQuoting
	statePreview
	stateExecuting
	stateDone
)

type field int

const (
	fieldToken field = iota
	fieldAmount
	fieldAddr
)

// Item wraps an owned token balance for the source list.
type Item struct {
	Symbol   string
	Address  string
	Decimals uint8
	Balance  string
	USDValue string
}

func (i Item) FilterValue() string { return i.Symbol }
func (i Item) Title() string {
	usd := i.USDValue
	if usd == "" {
		usd = "—"
	}
	return fmt.Sprintf("%-8s %14s   %s", i.Symbol, i.Balance, usd)
}
func (i Item) Description() string { return "" }

func (i Item) toTokenInfo() swap.TokenInfo {
	return swap.TokenInfo{Symbol: i.Symbol, Address: i.Address, Decimals: i.Decimals}
}

// Model owns the Swap BTC tab state.
type Model struct {
	tokens list.Model
	amount textinput.Model
	addr   textinput.Model

	state state
	field field

	from      swap.TokenInfo
	quote     *swap.FlowQuote
	quoteErr  string
	countdown int
	execRes   *swap.FlowResult

	// Price preview shown under the amount field (destination-less estimate).
	estimate    *swap.FlowQuote
	estimateErr string
	estimating  bool

	focused       bool
	width, height int
}

// New builds an empty panel. SetBalances seeds the source token list.
func New() Model {
	amt := textinput.New()
	amt.Placeholder = "e.g. 50"
	amt.Prompt = "$ "
	amt.CharLimit = 16
	amt.Width = 20

	addr := textinput.New()
	addr.Placeholder = "bc1q… or 1… or 3…"
	addr.Prompt = ""
	addr.CharLimit = 62
	addr.Width = 44

	return Model{
		tokens: newList("Swap to BTC — choose source"),
		amount: amt,
		addr:   addr,
		state:  stateForm,
		field:  fieldToken,
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// SetSize lays out the inner widgets within the outer dimensions.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	listW := w - 4
	if listW < 10 {
		listW = 10
	}
	// Reserve rows for the chrome the form always draws around the list:
	// border (2) + title + 2 blanks + help + amount label/input + estimate
	// line + addr label/input + an optional error line. Keeps it within height.
	listH := h - 14
	if listH < 3 {
		listH = 3
	}
	m.tokens.SetSize(listW, listH)
	m.amount.Width = clampInt(w-8, 10, 24)
	m.addr.Width = clampInt(w-8, 12, 48)
}

// SetFocused toggles the focused border color.
func (m *Model) SetFocused(b bool) { m.focused = b }

// SetBalances rebuilds the source token list from the wallet's balances.
func (m *Model) SetBalances(balances []balance.TokenBalance) {
	items := make([]list.Item, 0, len(balances))
	for _, b := range balances {
		items = append(items, Item{
			Symbol:   b.Symbol,
			Address:  b.Address,
			Decimals: b.Decimals,
			Balance:  b.Balance,
			USDValue: b.USDValue,
		})
	}
	m.tokens.SetItems(items)
}

// Capturing reports whether a text field is currently focused (so digits and
// letters are typed rather than treated as global shortcuts).
func (m Model) Capturing() bool {
	return m.state == stateForm && (m.field == fieldAmount || m.field == fieldAddr)
}

// ─── list helpers ──────────────────────────────────────────────────────────────

func newList(title string) list.Model {
	l := list.New(nil, theme.ListDelegate{}, 40, 6)
	l.Title = title
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().Foreground(theme.Yellow).Bold(true).Padding(0, 1)
	return l
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
