package mainscreen

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/api"
	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	passpkg "github.com/FernandoPazCavalcante/lazyswap/internal/pass"
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
type passStatusMsg struct {
	status passpkg.Status
	err    error
}
type passBoughtMsg struct {
	txHash string
	err    error
}

// chainSwitchedMsg reports the result of re-dialing services for a new chain.
type chainSwitchedMsg struct {
	chainKey string
	balSvc   *balance.Service
	flowSvc  *swap.Flow
	passSvc  *passpkg.Service
	err      error
}

func refreshWalletsCmd(svc *walletpkg.Service) tea.Cmd {
	return func() tea.Msg {
		ws, err := svc.FetchAll()
		return walletsRefreshedMsg{wallets: ws, err: err}
	}
}
func createCmd(svc *walletpkg.Service) tea.Cmd {
	return func() tea.Msg {
		w, err := svc.Create()
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

// quoteCmds builds the quote command (hybrid route) plus the async safety
// check when the destination token warrants one.
func (m Model) quoteCmds(from, to swap.TokenInfo, usd string) tea.Cmd {
	quoteCmd := hybridQuoteCmd(m.flowSvc, m.apiClient, m.chainKey,
		from, to, usd, m.current.Address, m.current.PrivateKey, m.slippage, m.useAPIRoute())
	if safety.ShouldCheck(chain.Get(m.chainKey), to) {
		return tea.Batch(quoteCmd, safetyCmd(m.safetySvc, m.chainKey, to.Address))
	}
	return quoteCmd
}

// hybridQuoteCmd quotes via the API route when asked, authenticating lazily
// with the wallet's key; any API failure logs and falls back to direct, so
// the overlay always gets a quote.
func hybridQuoteCmd(
	f *swap.Flow, ac *api.Client, chainKey string,
	from, to swap.TokenInfo, usd, walletAddr, privKey string,
	slippage float64, useAPI bool,
) tea.Cmd {
	return func() tea.Msg {
		if f == nil {
			return swapQuoteMsg{err: fmt.Errorf("swap service not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cfg := chain.Get(chainKey)
		if useAPI && cfg.OpenOceanKey != "" {
			ok := ac.Authenticated()
			if !ok {
				if _, err := ac.Authenticate(ctx, walletAddr, privKey); err != nil {
					applog.Warnf("api auth failed, using direct route: %v", err)
				} else {
					ok = true
				}
			}
			if ok {
				q, err := api.QuoteFlow(ctx, ac, f, cfg, from, to, usd, slippage)
				if err == nil {
					return swapQuoteMsg{quote: q}
				}
				applog.Warnf("api quote failed, using direct route: %v", err)
			}
		}
		q, err := f.Quote(ctx, from, to, usd, slippage, walletAddr)
		return swapQuoteMsg{quote: q, err: err}
	}
}

// executeAPICmd runs the backend-routed execution off the UI loop.
func executeAPICmd(
	ac *api.Client, f *swap.Flow, chainKey, privKey string,
	from, to swap.TokenInfo, usd string, slippage float64,
) tea.Cmd {
	return func() tea.Msg {
		if f == nil {
			return swapExecMsg{result: swap.FlowResult{Success: false, Err: "swap service not configured"}}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		return swapExecMsg{result: api.ExecuteFlow(ctx, ac, f, chain.Get(chainKey), privKey, from, to, usd, slippage)}
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

// mintPassCmd mints a LazySwap Pass from the wallet (one-click; the private key
// is already decrypted in-session), blocking until the tx is mined.
func mintPassCmd(svc *passpkg.Service, privateKey string) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return passBoughtMsg{err: fmt.Errorf("pass service not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		tx, err := svc.Buy(ctx, privateKey)
		return passBoughtMsg{txHash: tx, err: err}
	}
}

func thorQuoteCmd(f *swap.Flow, from swap.TokenInfo, usd, btc string) tea.Cmd {
	return func() tea.Msg {
		if f == nil {
			return swapbtcpanel.QuoteResultMsg{Err: fmt.Errorf("swap service not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		q, err := f.GetThorchainQuote(ctx, from, usd, btc)
		return swapbtcpanel.QuoteResultMsg{Quote: q, Err: err}
	}
}

func thorEstimateCmd(f *swap.Flow, from swap.TokenInfo, usd string) tea.Cmd {
	return func() tea.Msg {
		if f == nil {
			return swapbtcpanel.EstimateResultMsg{Err: fmt.Errorf("swap service not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		q, err := f.EstimateThorchain(ctx, from, usd)
		return swapbtcpanel.EstimateResultMsg{Quote: q, Err: err}
	}
}

func thorExecuteCmd(f *swap.Flow, privateKey string, from swap.TokenInfo, usd, btc string) tea.Cmd {
	return func() tea.Msg {
		if f == nil {
			return swapbtcpanel.ExecutionResultMsg{Result: swap.FlowResult{Success: false, Err: "swap service not configured"}}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		res := f.ExecuteThorchain(ctx, privateKey, from, usd, btc)
		return swapbtcpanel.ExecutionResultMsg{Result: res}
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
		// Pass is optional per chain: a nil service (no deployment) just makes
		// the Pass tab show "not available" — never blocks the switch.
		passSvc, _ := passpkg.New(chainKey)
		return chainSwitchedMsg{chainKey: chainKey, balSvc: balSvc, flowSvc: flowSvc, passSvc: passSvc}
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

// refreshPassCmd fetches the current wallet's pass status off the UI thread.
// Returns nil when no pass service (chain without a pass) or no wallet.
func (m *Model) refreshPassCmd() tea.Cmd {
	if m.passSvc == nil || m.current == nil {
		return nil
	}
	svc := m.passSvc
	addr := m.current.Address
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		st, err := svc.Status(ctx, addr)
		return passStatusMsg{status: st, err: err}
	}
}
