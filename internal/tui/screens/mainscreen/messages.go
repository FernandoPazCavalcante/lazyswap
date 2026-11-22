package mainscreen

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/safety"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	swapbtcpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/swapbtc"
	walletpkg "github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// ─── Messages ────────────────────────────────────────────────────────────────

type walletsRefreshedMsg struct {
	wallets []walletpkg.Wallet
	err     error
}
type createdMsg struct {
	w   *walletpkg.Wallet
	err error
}
type importedMsg struct {
	w   *walletpkg.Wallet
	err error
}
type deletedMsg struct{ err error }
type balancesFetchedMsg struct {
	walletAddress string
	balances      []balance.TokenBalance
	err           error
}
type swapQuoteMsg struct {
	quote swap.FlowQuote
	err   error
}
type safetyMsg struct{ report safety.Report }
type swapExecMsg struct{ result swap.FlowResult }

// chainSwitchedMsg reports the result of re-dialing services for a new chain.
type chainSwitchedMsg struct {
	chainKey string
	balSvc   *balance.Service
	flowSvc  *swap.Flow
	err      error
}

func refreshWalletsCmd(svc *walletpkg.Service) tea.Cmd {
	return func() tea.Msg {
		ws, err := svc.FetchAll()
		return walletsRefreshedMsg{wallets: ws, err: err}
	}
}

// saveConfirmedCmd persists a generated phrase once the user has confirmed
// the backup. Until then nothing is written, so an abandoned backup leaves no
// wallet the user could fund and then lose.
func saveConfirmedCmd(svc *walletpkg.Service, phrase string) tea.Cmd {
	return func() tea.Msg {
		w, err := svc.Import(phrase)
		return createdMsg{w: w, err: err}
	}
}
func importCmd(svc *walletpkg.Service, phrase string) tea.Cmd {
	return func() tea.Msg {
		w, err := svc.Import(phrase)
		return importedMsg{w: w, err: err}
	}
}
func deleteCmd(svc *walletpkg.Service, id string) tea.Cmd {
	return func() tea.Msg {
		return deletedMsg{err: svc.Delete(id)}
	}
}

// quoteCmds builds the quote command plus the async safety
// check when the destination token warrants one.
func (m Model) quoteCmds(from, to swap.TokenInfo, usd string) tea.Cmd {
	quoteCmd := quoteCmd(m.flowSvc, from, to, usd, m.current.Address, m.slippage)
	if safety.ShouldCheck(chain.Get(m.chainKey), to) {
		return tea.Batch(quoteCmd, safetyCmd(m.safetySvc, m.chainKey, to.Address))
	}
	return quoteCmd
}

// quoteCmd quotes the swap off the UI loop.
func quoteCmd(
	f *swap.Flow, from, to swap.TokenInfo, usd, walletAddr string, slippage float64,
) tea.Cmd {
	return func() tea.Msg {
		if f == nil {
			return swapQuoteMsg{err: fmt.Errorf("swap service not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		q, err := f.Quote(ctx, from, to, usd, slippage, walletAddr)
		return swapQuoteMsg{quote: q, err: err}
	}
}

// safetyCmd runs the (cached, fail-closed) token risk check off the UI loop.
func safetyCmd(svc *safety.Service, chainKey, tokenAddr string) tea.Cmd {
	return func() tea.Msg {
		return safetyMsg{report: svc.Check(context.Background(), chainKey, tokenAddr)}
	}
}

func executeSwapCmd(f *swap.Flow, privateKey string, from, to swap.TokenInfo, usd string, slippage float64) tea.Cmd {
	return func() tea.Msg {
		if f == nil {
			return swapExecMsg{result: swap.FlowResult{Success: false, Err: "swap service not configured"}}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		res := f.Execute(ctx, privateKey, from, to, usd, slippage)
		return swapExecMsg{result: res}
	}
}

func btcQuoteCmd(p swap.BTCProvider, f *swap.Flow, req swapbtcpanel.QuoteRequestMsg) tea.Cmd {
	return func() tea.Msg {
		result := swapbtcpanel.QuoteResultMsg{ID: req.ID, Route: swapbtcpanel.RouteQuote{Provider: p.ID}}
		if f == nil {
			result.Route.Err = fmt.Errorf("swap service not configured")
			return result
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result.Route = swapbtcpanel.RouteQuote(p.Quote(f, ctx, req.From, req.USDAmount, req.BTCAddress))
		result.Route.Provider = p.ID
		return result
	}
}

func btcExecuteCmd(p swap.BTCProvider, f *swap.Flow, privateKey string, req swapbtcpanel.ExecuteRequestMsg) tea.Cmd {
	return func() tea.Msg {
		if f == nil {
			return swapbtcpanel.ExecutionResultMsg{ID: req.ID, Result: swap.FlowResult{Err: "swap service not configured"}}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		res := p.Execute(f, ctx, privateKey, req.Quote)
		return swapbtcpanel.ExecutionResultMsg{ID: req.ID, Result: res}
	}
}

// explorerAPIKeyEnv is the env var read at fetch time. Empty value disables
// explorer-API token discovery (chain-config tokens only).
const explorerAPIKeyEnv = "LAZYSWAP_EXPLORER_API_KEY"

func fetchBalancesCmd(svc *balance.Service, address string) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return balancesFetchedMsg{walletAddress: address, err: fmt.Errorf("balance service not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		bs, err := svc.FetchAll(ctx, address, os.Getenv(explorerAPIKeyEnv))
		return balancesFetchedMsg{walletAddress: address, balances: bs, err: err}
	}
}

// switchChainCmd re-dials the balance + swap RPC services for chainKey off the
// UI thread. The previous services are closed by the handler once these are
// ready, so a failed dial leaves the current chain untouched.
func switchChainCmd(chainKey string) tea.Cmd {
	return func() tea.Msg {
		balSvc, err := balance.New(chainKey)
		if err != nil {
			return chainSwitchedMsg{chainKey: chainKey, err: err}
		}
		flowSvc, err := swap.NewFlow(chainKey)
		if err != nil {
			balSvc.Close()
			return chainSwitchedMsg{chainKey: chainKey, err: err}
		}
		return chainSwitchedMsg{chainKey: chainKey, balSvc: balSvc, flowSvc: flowSvc}
	}
}

// balancesCmdForCurrent returns a tea.Cmd that fetches balances for the
// active wallet (or uses the cache when fresh). Returns nil if no wallet is
// selected.
func (m *Model) balancesCmdForCurrent() tea.Cmd {
	if m.current == nil {
		return nil
	}
	addr := m.current.Address
	subtitle := fmt.Sprintf("%s · %s", shortAddr(addr), m.chainName())
	m.tokens.SetSubtitle(subtitle)

	if cached, ok := m.balanceCache[addr]; ok {
		m.tokens.SetBalances(cached)
		m.swapbtc.SetBalances(cached)
		return nil
	}
	m.tokens.SetLoading()
	return fetchBalancesCmd(m.balSvc, addr)
}
