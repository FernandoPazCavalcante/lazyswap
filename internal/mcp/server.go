// Package mcp serves lazyswap to AI agents over the Model Context Protocol
// (stdio transport). Read-only by default; swap_execute is registered only
// when trading is explicitly enabled and is guarded by a per-swap USD cap and
// an optional chain allowlist.
//
// Hard rule: in MCP mode stdout is the JSON-RPC channel. Nothing in this
// package (or anything it calls) may print — log via applog only.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// Options configure the MCP server surface.
type Options struct {
	AllowTrading bool     // register swap_execute / buy_pass
	MaxUSD       float64  // per-swap USD cap; required > 0 when AllowTrading
	Chains       []string // allowlist for trading tools; empty = any configured chain
	Version      string
}

// server holds shared state behind the tool handlers. Flows and balance
// services are dialed lazily per chain and cached; the mutex guards the maps
// (the underlying ethclient is safe for concurrent use).
type server struct {
	dao  *wallet.DAO
	opts Options
	pw   string // LAZYSWAP_PASSWORD; verified at startup when trading is enabled

	mu    sync.Mutex
	flows map[string]*swap.Flow
	bals  map[string]*balance.Service
}

// Run validates options, opens the wallet DAO and serves MCP over stdio until
// ctx is cancelled or the client disconnects.
func Run(ctx context.Context, opts Options) error {
	if opts.AllowTrading && opts.MaxUSD <= 0 {
		return errors.New("--allow-trading requires --max-usd > 0")
	}
	for _, k := range opts.Chains {
		if !chain.Has(k) {
			return fmt.Errorf("unknown chain %q in --chain (configured: %v)", k, chain.OrderedKeys)
		}
	}

	dao, err := wallet.Open()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer dao.Close()

	s := &server{
		dao:   dao,
		opts:  opts,
		pw:    os.Getenv("LAZYSWAP_PASSWORD"),
		flows: map[string]*swap.Flow{},
		bals:  map[string]*balance.Service{},
	}
	if opts.AllowTrading {
		// The password comes from the environment only — never from a tool
		// parameter, so it can never land in an agent's context. Verify it now
		// so a typo fails at startup, not at the first swap_execute.
		if s.pw == "" {
			return errors.New("--allow-trading requires LAZYSWAP_PASSWORD to be set")
		}
		if _, err := wallet.Unlock(dao, s.pw); err != nil {
			return err
		}
	}

	srv := sdk.NewServer(&sdk.Implementation{Name: "lazyswap", Version: opts.Version}, nil)
	s.register(srv)
	return srv.Run(ctx, &sdk.StdioTransport{})
}

// chainKey resolves the effective chain: explicit argument > configured default.
func (s *server) chainKey(explicit string) (string, error) {
	if explicit != "" {
		if !chain.Has(explicit) {
			return "", fmt.Errorf("unknown chain %q (configured: %v)", explicit, chain.OrderedKeys)
		}
		return explicit, nil
	}
	st, err := settings.Load(s.dao)
	if err != nil {
		return "", err
	}
	return st.ChainKey, nil
}

// tradingChainAllowed reports whether trading tools may act on the chain.
func (s *server) tradingChainAllowed(key string) bool {
	return len(s.opts.Chains) == 0 || slices.Contains(s.opts.Chains, key)
}

// slippage resolves the effective slippage: explicit (>0) > configured default.
func (s *server) slippage(explicit float64) (float64, error) {
	if explicit > 0 {
		return explicit, nil
	}
	st, err := settings.Load(s.dao)
	if err != nil {
		return 0, err
	}
	return st.Slippage, nil
}

// pickWallet resolves the wallet by address (or the configured default).
// Private keys stay encrypted — this is address-level only.
func (s *server) pickWallet(explicit string) (wallet.Wallet, error) {
	ws, err := s.dao.FetchAll()
	if err != nil {
		return wallet.Wallet{}, err
	}
	st, err := settings.Load(s.dao)
	if err != nil {
		return wallet.Wallet{}, err
	}
	return wallet.Pick(ws, explicit, st.DefaultWallet)
}

// unlockWallet resolves the wallet with its private key decrypted. Trading
// tools only.
func (s *server) unlockWallet(explicit string) (wallet.Wallet, error) {
	svc, err := wallet.Unlock(s.dao, s.pw)
	if err != nil {
		return wallet.Wallet{}, err
	}
	ws, err := wallet.NewService(s.dao, svc).FetchAll()
	if err != nil {
		return wallet.Wallet{}, err
	}
	st, err := settings.Load(s.dao)
	if err != nil {
		return wallet.Wallet{}, err
	}
	return wallet.Pick(ws, explicit, st.DefaultWallet)
}

func (s *server) flow(key string) (*swap.Flow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.flows[key]; ok {
		return f, nil
	}
	f, err := swap.NewFlow(key)
	if err != nil {
		return nil, err
	}
	s.flows[key] = f
	return f, nil
}

func (s *server) balances(key string) (*balance.Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.bals[key]; ok {
		return b, nil
	}
	b, err := balance.New(key)
	if err != nil {
		return nil, err
	}
	s.bals[key] = b
	return b, nil
}
