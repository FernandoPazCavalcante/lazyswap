package mcp

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	apiclient "github.com/FernandoPazCavalcante/lazyswap/internal/api"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/pass"
	"github.com/FernandoPazCavalcante/lazyswap/internal/safety"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

// flagDescs flattens risk flags into their descriptions for error text.
func flagDescs(fs []safety.Flag) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Desc
	}
	return out
}

// rpcTimeout bounds every handler that touches the chain, mirroring the CLI.
const rpcTimeout = 2 * time.Minute

// explorerAPIKeyEnv mirrors the TUI: set it to enable ERC-20 token discovery
// from explorer transfer history during get_balances.
const explorerAPIKeyEnv = "LAZYSWAP_EXPLORER_API_KEY"

// register adds every tool to srv. Trading tools (swap_execute, buy_pass) are
// registered only when Options.AllowTrading is set — absent tools cannot be
// called by a prompt-injected agent.
func (s *server) register(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{Name: "list_chains",
		Description: "List supported chains with their swappable token symbols."}, s.listChains)
	sdk.AddTool(srv, &sdk.Tool{Name: "get_wallets",
		Description: "List wallet addresses (never private keys)."}, s.getWallets)
	sdk.AddTool(srv, &sdk.Tool{Name: "get_balances",
		Description: "Fetch token balances (with USD values) for a wallet on a chain."}, s.getBalances)
	sdk.AddTool(srv, &sdk.Tool{Name: "swap_quote",
		Description: "Quote a swap in USD terms: token amounts, estimated/min output, fee. Read-only, does not trade."}, s.swapQuote)
	sdk.AddTool(srv, &sdk.Tool{Name: "get_settings",
		Description: "Read the persisted settings: chain, slippage, default wallet."}, s.getSettings)
	sdk.AddTool(srv, &sdk.Tool{Name: "set_settings",
		Description: "Update persisted settings: default chain, slippage percent, default wallet."}, s.setSettings)
	sdk.AddTool(srv, &sdk.Tool{Name: "get_pass_status",
		Description: "Check LazySwapPass (ERC-721) validity and expiry for a wallet."}, s.getPassStatus)
	sdk.AddTool(srv, &sdk.Tool{Name: "get_referral_stats",
		Description: "Referral dashboard: code, referred wallets, earned/claimable USD. Read-only; needs --allow-trading (SIWE auth signs with the wallet key)."}, s.getReferralStats)

	if s.opts.AllowTrading {
		sdk.AddTool(srv, &sdk.Tool{Name: "swap_execute",
			Description: fmt.Sprintf("Execute an on-chain swap. Capped at $%.2f per swap. Quotes first via swap_quote are recommended.", s.opts.MaxUSD)}, s.swapExecute)
		sdk.AddTool(srv, &sdk.Tool{Name: "buy_pass",
			Description: "Mint a LazySwapPass NFT for the wallet (costs the on-chain mint price plus gas)."}, s.buyPass)
	}
}

// ---- list_chains ----

type listChainsIn struct{}

type chainOut struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	ChainID      uint64   `json:"chainId"`
	NativeSymbol string   `json:"nativeSymbol"`
	Tokens       []string `json:"tokens" jsonschema:"swappable token symbols"`
	Default      bool     `json:"default" jsonschema:"true for the configured default chain"`
}

type listChainsOut struct {
	Chains []chainOut `json:"chains"`
}

func (s *server) listChains(ctx context.Context, req *sdk.CallToolRequest, in listChainsIn) (*sdk.CallToolResult, listChainsOut, error) {
	st, err := settings.Load(s.dao)
	if err != nil {
		return nil, listChainsOut{}, err
	}
	var out listChainsOut
	for _, key := range chain.OrderedKeys {
		c := chain.Get(key)
		out.Chains = append(out.Chains, chainOut{
			Key:          key,
			Name:         c.Name,
			ChainID:      c.ChainID,
			NativeSymbol: c.NativeSymbol,
			Tokens:       strings.Split(swap.AvailableSymbols(c), ", "),
			Default:      key == st.ChainKey,
		})
	}
	return nil, out, nil
}

// ---- get_wallets ----

type getWalletsIn struct{}

type walletOut struct {
	Address string `json:"address"`
	Default bool   `json:"default"`
}

type getWalletsOut struct {
	Wallets []walletOut `json:"wallets"`
}

func (s *server) getWallets(ctx context.Context, req *sdk.CallToolRequest, in getWalletsIn) (*sdk.CallToolResult, getWalletsOut, error) {
	ws, err := s.dao.FetchAll()
	if err != nil {
		return nil, getWalletsOut{}, err
	}
	st, err := settings.Load(s.dao)
	if err != nil {
		return nil, getWalletsOut{}, err
	}
	out := getWalletsOut{Wallets: []walletOut{}}
	for _, w := range ws {
		out.Wallets = append(out.Wallets, walletOut{
			Address: w.Address,
			Default: strings.EqualFold(w.Address, st.DefaultWallet),
		})
	}
	return nil, out, nil
}

// ---- get_balances ----

type getBalancesIn struct {
	Wallet string `json:"wallet,omitempty" jsonschema:"wallet address (default: configured default wallet)"`
	Chain  string `json:"chain,omitempty" jsonschema:"chain key (default: configured chain)"`
}

type balanceOut struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Balance  string `json:"balance"`
	USDValue string `json:"usdValue,omitempty"`
}

type getBalancesOut struct {
	Chain    string       `json:"chain"`
	Wallet   string       `json:"wallet"`
	Balances []balanceOut `json:"balances"`
}

func (s *server) getBalances(ctx context.Context, req *sdk.CallToolRequest, in getBalancesIn) (*sdk.CallToolResult, getBalancesOut, error) {
	key, err := s.chainKey(in.Chain)
	if err != nil {
		return nil, getBalancesOut{}, err
	}
	w, err := s.pickWallet(in.Wallet)
	if err != nil {
		return nil, getBalancesOut{}, err
	}
	svc, err := s.balances(key)
	if err != nil {
		return nil, getBalancesOut{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	bs, err := svc.FetchAll(ctx, w.Address, os.Getenv(explorerAPIKeyEnv))
	if err != nil {
		return nil, getBalancesOut{}, err
	}
	out := getBalancesOut{Chain: key, Wallet: w.Address, Balances: []balanceOut{}}
	for _, b := range bs {
		out.Balances = append(out.Balances, balanceOut{
			Symbol:   b.Symbol,
			Name:     b.Name,
			Address:  b.Address,
			Balance:  b.Balance,
			USDValue: b.USDValue,
		})
	}
	return nil, out, nil
}

// ---- swap_quote / swap_execute ----

type swapIn struct {
	USD      float64 `json:"usd" jsonschema:"USD amount to swap, e.g. 5"`
	From     string  `json:"from" jsonschema:"source token symbol, e.g. BNB"`
	To       string  `json:"to" jsonschema:"destination token symbol, e.g. USDT"`
	Slippage float64 `json:"slippage,omitempty" jsonschema:"slippage percent; omit for the configured default"`
	Wallet   string  `json:"wallet,omitempty" jsonschema:"wallet address (default: configured default wallet)"`
	Chain    string  `json:"chain,omitempty" jsonschema:"chain key (default: configured chain)"`
	Mode     string  `json:"mode,omitempty" jsonschema:"swap route: direct (on-chain V2 router, no fee) or api (OpenOcean best rate, 1% fee, needs LazySwap Pass); default: configured"`
}

// resolveSwap maps a swapIn to everything a quote or execute needs.
func (s *server) resolveSwap(in swapIn) (key string, from, to swap.TokenInfo, usd string, slip float64, err error) {
	if in.USD <= 0 {
		err = fmt.Errorf("usd must be a positive amount, got %v", in.USD)
		return
	}
	if key, err = s.chainKey(in.Chain); err != nil {
		return
	}
	c := chain.Get(key)
	if from, err = swap.ResolveToken(c, in.From); err != nil {
		return
	}
	if to, err = swap.ResolveToken(c, in.To); err != nil {
		return
	}
	if slip, err = s.slippage(in.Slippage); err != nil {
		return
	}
	usd = strconv.FormatFloat(in.USD, 'f', -1, 64)
	return
}

// swapQuoteOut is a FlowQuote plus the token risk verdict, so agents see the
// safety picture without an extra round-trip.
type swapQuoteOut struct {
	swap.FlowQuote
	Safety *safety.Report `json:"safety,omitempty" jsonschema:"risk report for the token being bought; absent for native/stablecoin buys"`
}

// riskReport assesses the destination token; nil when no check applies.
func (s *server) riskReport(ctx context.Context, key string, to swap.TokenInfo) *safety.Report {
	if !safety.ShouldCheck(chain.Get(key), to) {
		return nil
	}
	rep := s.safety.Check(ctx, key, to.Address)
	return &rep
}

func (s *server) swapQuote(ctx context.Context, req *sdk.CallToolRequest, in swapIn) (*sdk.CallToolResult, swapQuoteOut, error) {
	key, from, to, usd, slip, err := s.resolveSwap(in)
	if err != nil {
		return nil, swapQuoteOut{}, err
	}
	mode, err := s.swapMode(in.Mode)
	if err != nil {
		return nil, swapQuoteOut{}, err
	}
	w, err := s.pickWallet(in.Wallet)
	if err != nil {
		return nil, swapQuoteOut{}, err
	}
	f, err := s.flow(key)
	if err != nil {
		return nil, swapQuoteOut{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	var q swap.FlowQuote
	if mode == settings.SwapModeAPI {
		ac, err := s.apiAuthed(ctx, in.Wallet)
		if err != nil {
			return nil, swapQuoteOut{}, err
		}
		q, err = apiclient.QuoteFlow(ctx, ac, f, chain.Get(key), from, to, usd, slip)
		if err != nil {
			return nil, swapQuoteOut{}, err
		}
	} else {
		q, err = f.Quote(ctx, from, to, usd, slip, w.Address)
		if err != nil {
			return nil, swapQuoteOut{}, err
		}
		q.Mode = settings.SwapModeDirect
	}
	return nil, swapQuoteOut{FlowQuote: q, Safety: s.riskReport(ctx, key, to)}, nil
}

func (s *server) swapExecute(ctx context.Context, req *sdk.CallToolRequest, in swapIn) (*sdk.CallToolResult, swap.FlowResult, error) {
	key, from, to, usd, slip, err := s.resolveSwap(in)
	if err != nil {
		return nil, swap.FlowResult{}, err
	}
	// Code-enforced guardrails: a prompt-injected agent cannot exceed the cap
	// or leave the allowlisted chains.
	if in.USD > s.opts.MaxUSD {
		return nil, swap.FlowResult{}, fmt.Errorf("refused: $%v exceeds the --max-usd cap of $%v", in.USD, s.opts.MaxUSD)
	}
	if !s.tradingChainAllowed(key) {
		return nil, swap.FlowResult{}, fmt.Errorf("refused: trading on %q is not in the --chain allowlist %v", key, s.opts.Chains)
	}
	// No human squints at a warning here, so a HIGH risk verdict is a refusal
	// unless the server was started with --allow-risky. Unknown (testnets, API
	// outage) stays advisory — refusing would brick testnet trading.
	if rep := s.riskReport(ctx, key, to); rep != nil && rep.Level == safety.LevelHigh && !s.opts.AllowRisky {
		return nil, swap.FlowResult{}, fmt.Errorf("refused: token risk is HIGH (%s) — restart with --allow-risky to override", strings.Join(flagDescs(rep.Flags), "; "))
	}
	mode, err := s.swapMode(in.Mode)
	if err != nil {
		return nil, swap.FlowResult{}, err
	}
	w, err := s.unlockWallet(in.Wallet)
	if err != nil {
		return nil, swap.FlowResult{}, err
	}
	f, err := s.flow(key)
	if err != nil {
		return nil, swap.FlowResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	var res swap.FlowResult
	if mode == settings.SwapModeAPI {
		ac, err := s.apiAuthed(ctx, in.Wallet)
		if err != nil {
			return nil, swap.FlowResult{}, err
		}
		res = apiclient.ExecuteFlow(ctx, ac, f, chain.Get(key), w.PrivateKey, from, to, usd, slip)
	} else {
		res = f.Execute(ctx, w.PrivateKey, from, to, usd, slip)
	}
	if !res.Success {
		return nil, swap.FlowResult{}, fmt.Errorf("swap failed: %s", res.Err)
	}
	return nil, res, nil
}

// ---- get_settings / set_settings ----

type getSettingsIn struct{}

type settingsOut struct {
	Chain         string  `json:"chain"`
	Slippage      float64 `json:"slippage"`
	DefaultWallet string  `json:"defaultWallet,omitempty"`
}

func (s *server) getSettings(ctx context.Context, req *sdk.CallToolRequest, in getSettingsIn) (*sdk.CallToolResult, settingsOut, error) {
	st, err := settings.Load(s.dao)
	if err != nil {
		return nil, settingsOut{}, err
	}
	return nil, settingsOut{Chain: st.ChainKey, Slippage: st.Slippage, DefaultWallet: st.DefaultWallet}, nil
}

type setSettingsIn struct {
	Chain         string   `json:"chain,omitempty" jsonschema:"default chain key"`
	Slippage      *float64 `json:"slippage,omitempty" jsonschema:"default slippage percent (0-100)"`
	DefaultWallet string   `json:"defaultWallet,omitempty" jsonschema:"default wallet address"`
}

func (s *server) setSettings(ctx context.Context, req *sdk.CallToolRequest, in setSettingsIn) (*sdk.CallToolResult, settingsOut, error) {
	if in.Chain != "" {
		if !chain.Has(in.Chain) {
			return nil, settingsOut{}, fmt.Errorf("unknown chain %q (configured: %v)", in.Chain, chain.OrderedKeys)
		}
		if err := settings.SetChain(s.dao, in.Chain); err != nil {
			return nil, settingsOut{}, err
		}
	}
	if in.Slippage != nil {
		if err := settings.SetSlippage(s.dao, *in.Slippage); err != nil {
			return nil, settingsOut{}, err
		}
	}
	if in.DefaultWallet != "" {
		if _, err := s.pickWallet(in.DefaultWallet); err != nil {
			return nil, settingsOut{}, err
		}
		if err := settings.SetDefaultWallet(s.dao, in.DefaultWallet); err != nil {
			return nil, settingsOut{}, err
		}
	}
	return s.getSettings(ctx, req, getSettingsIn{})
}

// ---- get_referral_stats ----

type referralStatsIn struct {
	Wallet string `json:"wallet,omitempty" jsonschema:"wallet address (default: configured default wallet)"`
}

func (s *server) getReferralStats(ctx context.Context, req *sdk.CallToolRequest, in referralStatsIn) (*sdk.CallToolResult, apiclient.ReferralStats, error) {
	ac, err := s.apiAuthed(ctx, in.Wallet)
	if err != nil {
		return nil, apiclient.ReferralStats{}, err
	}
	st, err := ac.ReferralStats(ctx)
	if err != nil {
		return nil, apiclient.ReferralStats{}, err
	}
	return nil, st, nil
}

// ---- get_pass_status / buy_pass ----

type passIn struct {
	Wallet string `json:"wallet,omitempty" jsonschema:"wallet address (default: configured default wallet)"`
	Chain  string `json:"chain,omitempty" jsonschema:"chain key (default: configured chain)"`
}

type passStatusOut struct {
	Chain        string `json:"chain"`
	Wallet       string `json:"wallet"`
	Deployed     bool   `json:"deployed" jsonschema:"false when the pass contract is not on this chain"`
	HasValidPass bool   `json:"hasValidPass"`
	ExpiresAt    string `json:"expiresAt,omitempty" jsonschema:"RFC3339 expiry of the latest pass"`
}

func (s *server) getPassStatus(ctx context.Context, req *sdk.CallToolRequest, in passIn) (*sdk.CallToolResult, passStatusOut, error) {
	key, err := s.chainKey(in.Chain)
	if err != nil {
		return nil, passStatusOut{}, err
	}
	w, err := s.pickWallet(in.Wallet)
	if err != nil {
		return nil, passStatusOut{}, err
	}
	out := passStatusOut{Chain: key, Wallet: w.Address}
	if chain.Get(key).PassAddress == "" {
		return nil, out, nil
	}
	svc, err := pass.New(key)
	if err != nil {
		return nil, passStatusOut{}, err
	}
	defer svc.Close()
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	st, err := svc.Status(ctx, w.Address)
	if err != nil {
		return nil, passStatusOut{}, err
	}
	out.Deployed = true
	out.HasValidPass = st.HasValidPass
	if !st.ExpiresAt.IsZero() {
		out.ExpiresAt = st.ExpiresAt.Format(time.RFC3339)
	}
	return nil, out, nil
}

type buyPassOut struct {
	Chain  string `json:"chain"`
	Wallet string `json:"wallet"`
	TxHash string `json:"txHash"`
}

func (s *server) buyPass(ctx context.Context, req *sdk.CallToolRequest, in passIn) (*sdk.CallToolResult, buyPassOut, error) {
	key, err := s.chainKey(in.Chain)
	if err != nil {
		return nil, buyPassOut{}, err
	}
	if chain.Get(key).PassAddress == "" {
		return nil, buyPassOut{}, fmt.Errorf("the LazySwapPass contract is not deployed on %q", key)
	}
	if !s.tradingChainAllowed(key) {
		return nil, buyPassOut{}, fmt.Errorf("refused: trading on %q is not in the --chain allowlist %v", key, s.opts.Chains)
	}
	w, err := s.unlockWallet(in.Wallet)
	if err != nil {
		return nil, buyPassOut{}, err
	}
	svc, err := pass.New(key)
	if err != nil {
		return nil, buyPassOut{}, err
	}
	defer svc.Close()
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	tx, err := svc.Buy(ctx, w.PrivateKey)
	if err != nil {
		return nil, buyPassOut{}, err
	}
	return nil, buyPassOut{Chain: key, Wallet: w.Address, TxHash: tx}, nil
}
