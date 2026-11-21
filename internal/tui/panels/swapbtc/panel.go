// Package swapbtc implements the EVM-to-BTC form, quote comparison and review.
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

// refreshSeconds caps the validity of quotes that omit an expiry.
const refreshSeconds = 10

// minBTCAddrLen is the shortest plausible Bitcoin address (matches swap-tab.ts).
const minBTCAddrLen = 25

// ─── State ───────────────────────────────────────────────────────────────────

type state int

const (
	stateForm state = iota
	stateQuotes
	stateAddress
	stateReview
	stateExecuting
	stateDone
)

type field int

const (
	fieldToken field = iota
	fieldAmount
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

	from        swap.TokenInfo
	network     string
	destination string
	requestID   uint64
	providers   []Provider
	quotes      []RouteQuote
	selected    Provider
	expanded    bool
	ticking     bool
	quoteErr    string
	execRes     *swap.FlowResult

	focused       bool
	width, height int
}

// New builds an empty panel. SetBalances seeds the source token list.
func New(providers ...Provider) Model {
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
		providers: providers,
		tokens:    newList("Swap to BTC — choose source"),
		amount:    amt,
		addr:      addr,
		state:     stateForm,
		field:     fieldToken,
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
	// Leave room for the border, title, amount, hint, and validation feedback.
	listH := h - 11
	if listH < 3 {
		listH = 3
	}
	m.tokens.SetSize(listW, listH)
	m.amount.Width = clampInt(w-8, 10, 24)
	m.addr.Width = clampInt(w-8, 12, 48)
}

// SetFocused toggles the focused border color.
func (m *Model) SetFocused(b bool) { m.focused = b }

func (m *Model) SetNetwork(network string) { m.network = network }

// Reset clears the form session when the source wallet or network changes.
func (m *Model) Reset() {
	m.reset()
	m.SetBalances(nil)
}

func (m Model) ShowingQuotes() bool {
	return m.state == stateQuotes || m.state == stateAddress || m.state == stateReview
}

func (m Model) IsExecuting(requestID uint64) bool {
	return m.state == stateExecuting && m.requestID == requestID
}

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
	return m.state == stateAddress || (m.state == stateForm && m.field == fieldAmount)
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
