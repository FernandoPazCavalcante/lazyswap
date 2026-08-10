package swapoverlay

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

// Item wraps a token row (either an owned balance or a recommended seed).
type Item struct {
	Symbol   string
	Address  string // NativeSentinel or 0x…
	Decimals uint8
	Balance  string // human, may be "0.00"
	USDValue string // may be ""
	Owned    bool
}

func (i Item) FilterValue() string { return i.Symbol }
func (i Item) Title() string {
	usd := i.USDValue
	if usd == "" {
		usd = "—"
	}
	return fmt.Sprintf("%-8s %14s   %s", i.Symbol, i.Balance, usd)
}
func (i Item) Description() string {
	if i.Owned {
		return ""
	}
	return "★ Recommended"
}

// toTokenInfo flattens an Item into the swap-layer struct.
func (i Item) toTokenInfo() swap.TokenInfo {
	return swap.TokenInfo{Symbol: i.Symbol, Address: i.Address, Decimals: i.Decimals}
}

// ─── Item building ───────────────────────────────────────────────────────────

func buildFromItems(balances []balance.TokenBalance) []list.Item {
	out := make([]list.Item, 0, len(balances))
	for _, b := range balances {
		// Native uses the sentinel address; ERC-20s carry their on-chain addr.
		out = append(out, list.Item(Item{
			Symbol:   b.Symbol,
			Address:  b.Address,
			Decimals: b.Decimals,
			Balance:  b.Balance,
			USDValue: b.USDValue,
			Owned:    true,
		}))
	}
	return out
}

func buildToItems(balances []balance.TokenBalance, c chain.Config) []list.Item {
	// Index owned balances by lowercase address for fast lookup.
	owned := make(map[string]balance.TokenBalance, len(balances))
	for _, b := range balances {
		owned[strings.ToLower(b.Address)] = b
	}

	// Native first.
	out := []list.Item{
		Item{
			Symbol:   c.NativeSymbol,
			Address:  balance.NativeAddress,
			Decimals: c.NativeDecimals,
			Balance:  pickBalance(owned[balance.NativeAddress]),
			USDValue: owned[balance.NativeAddress].USDValue,
			Owned:    contains(owned, balance.NativeAddress),
		},
	}

	// Recommended tokens (filter out native sentinel collisions).
	seen := map[string]bool{strings.ToLower(balance.NativeAddress): true}
	for _, r := range c.RecommendedTokens {
		key := strings.ToLower(r.Address)
		if seen[key] {
			continue
		}
		seen[key] = true
		b, isOwned := owned[key]
		out = append(out, Item{
			Symbol:   r.Symbol,
			Address:  r.Address,
			Decimals: r.Decimals,
			Balance:  pickBalance(b),
			USDValue: b.USDValue,
			Owned:    isOwned,
		})
	}

	// Then any owned tokens not already listed.
	for _, b := range balances {
		key := strings.ToLower(b.Address)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Item{
			Symbol:   b.Symbol,
			Address:  b.Address,
			Decimals: b.Decimals,
			Balance:  b.Balance,
			USDValue: b.USDValue,
			Owned:    true,
		})
	}
	return out
}

func pickBalance(b balance.TokenBalance) string {
	if b.Balance == "" {
		return "0.00"
	}
	return b.Balance
}

func contains(owned map[string]balance.TokenBalance, addr string) bool {
	_, ok := owned[strings.ToLower(addr)]
	return ok
}

// ─── list helpers ────────────────────────────────────────────────────────────

func newList(title string, items []list.Item) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)

	dim := lipgloss.NewStyle().Foreground(theme.YellowDim).Padding(0, 0, 0, 2)
	// Selected rows use the shared YellowSel highlight block (matches the
	// Settings / Wallets / Tokens selection style).
	sel := lipgloss.NewStyle().Foreground(theme.Yellow).Background(theme.YellowSel).Padding(0, 0, 0, 2).Bold(true)
	delegate.Styles.NormalTitle = dim
	delegate.Styles.SelectedTitle = sel
	delegate.Styles.NormalDesc = lipgloss.NewStyle().Foreground(theme.YellowDim).Padding(0, 0, 0, 2)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().Foreground(theme.YellowDim).Background(theme.YellowSel).Padding(0, 0, 0, 2)

	l := list.New(items, delegate, 60, 10)
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
