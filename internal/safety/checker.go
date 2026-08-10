package safety

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

// Checker assesses one token on one chain.
type Checker interface {
	Check(ctx context.Context, chainKey, tokenAddr string) (Report, error)
}

// cacheTTL keeps reports fresh enough while staying far under the GoPlus
// free-tier rate limit and absorbing TUI re-renders.
const cacheTTL = 5 * time.Minute

// checkTimeout bounds the external call — the check sits in the swap hot path.
const checkTimeout = 6 * time.Second

// Service is the fail-closed entry point used by the CLI, TUI and MCP. Errors
// never escape: they become Unknown reports, so a GoPlus outage can warn but
// can never silently pass a token.
type Service struct {
	checker Checker

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	report Report
	at     time.Time
}

// New returns a Service backed by GoPlus.
func New() *Service {
	return NewWith(&GoPlus{})
}

// NewWith returns a Service backed by a specific checker (tests, future
// local-sim source).
func NewWith(c Checker) *Service {
	return &Service{checker: c, cache: map[string]cached{}}
}

// ShouldCheck reports whether the destination token is worth assessing: the
// chain's native/gas token and well-known stablecoins are skipped.
func ShouldCheck(c chain.Config, tok swap.TokenInfo) bool {
	if tok.Address == swap.NativeSentinel || tok.Address == "" {
		return false
	}
	switch strings.ToUpper(tok.Symbol) {
	case "USDT", "USDC", "DAI", "BUSD":
		return false
	}
	return !strings.EqualFold(tok.Address, c.StablecoinAddr)
}

// Check returns a Report for tokenAddr on chainKey, from cache when fresh.
// It never returns an error — failures come back as Unknown reports.
func (s *Service) Check(ctx context.Context, chainKey, tokenAddr string) Report {
	key := chainKey + ":" + strings.ToLower(tokenAddr)
	s.mu.Lock()
	if c, ok := s.cache[key]; ok && time.Since(c.at) < cacheTTL {
		s.mu.Unlock()
		return c.report
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	rep, err := s.checker.Check(ctx, chainKey, tokenAddr)
	if err != nil {
		applog.Warnf("safety: check %s failed: %v", key, err)
		// Fail closed: an unreachable risk API is a warning, not a pass.
		return Report{Unknown: true, Reason: "risk check failed"}
	}

	s.mu.Lock()
	s.cache[key] = cached{report: rep, at: time.Now()}
	s.mu.Unlock()
	return rep
}
