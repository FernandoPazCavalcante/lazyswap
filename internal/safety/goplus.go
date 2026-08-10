package safety

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
)

// goplusBase is the free no-key Token Security endpoint (~30 req/min).
const goplusBase = "https://api.gopluslabs.io/api/v1/token_security"

// goplusChains are the chain IDs the GoPlus API supports among our configured
// chains. Testnets (97, 11155111) are not covered — those return Unknown.
var goplusChains = map[uint64]bool{1: true, 56: true}

// GoPlus checks tokens against the GoPlus Token Security API.
type GoPlus struct {
	HTTP *http.Client
	Base string // override for tests; defaults to goplusBase
}

// goplusToken mirrors the fields we read. Everything is a string ("0"/"1" for
// flags, decimal fractions for taxes); pointers distinguish missing from "0" —
// a missing field means "unknown", not "safe".
type goplusToken struct {
	IsHoneypot           *string `json:"is_honeypot"`
	CannotSellAll        *string `json:"cannot_sell_all"`
	BuyTax               *string `json:"buy_tax"`
	SellTax              *string `json:"sell_tax"`
	IsMintable           *string `json:"is_mintable"`
	IsOpenSource         *string `json:"is_open_source"`
	OwnerChangeBalance   *string `json:"owner_change_balance"`
	HiddenOwner          *string `json:"hidden_owner"`
	SelfDestruct         *string `json:"selfdestruct"`
	CanTakeBackOwnership *string `json:"can_take_back_ownership"`
	TransferPausable     *string `json:"transfer_pausable"`
	IsBlacklisted        *string `json:"is_blacklisted"`
	TradingCooldown      *string `json:"trading_cooldown"`
	IsProxy              *string `json:"is_proxy"`
}

type goplusResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Result  map[string]goplusToken `json:"result"`
}

// Check fetches and scores the GoPlus report for tokenAddr on chainKey.
func (g *GoPlus) Check(ctx context.Context, chainKey, tokenAddr string) (Report, error) {
	c := chain.Get(chainKey)
	if !goplusChains[c.ChainID] {
		return Report{Unknown: true, Reason: fmt.Sprintf("GoPlus does not cover %s", c.Name)}, nil
	}

	base := g.Base
	if base == "" {
		base = goplusBase
	}
	url := fmt.Sprintf("%s/%d?contract_addresses=%s", base, c.ChainID, tokenAddr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Report{}, err
	}
	httpc := g.HTTP
	if httpc == nil {
		httpc = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return Report{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Report{}, fmt.Errorf("goplus: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Report{}, err
	}

	var gr goplusResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return Report{}, fmt.Errorf("goplus: parse: %w", err)
	}
	// Result is keyed by the LOWERCASED address.
	tok, ok := gr.Result[strings.ToLower(tokenAddr)]
	if !ok {
		return Report{Unknown: true, Reason: "GoPlus has no data for this token"}, nil
	}
	return scoreGoplus(tok), nil
}

// set reports a "1" string flag; nil (missing) is NOT set — and NOT safe.
func set(p *string) bool { return p != nil && *p == "1" }

// taxBps parses a GoPlus decimal-fraction tax ("0.05" = 5%) into basis points.
// Returns -1 when missing/unparseable.
func taxBps(p *string) int {
	if p == nil || *p == "" {
		return -1
	}
	f, err := strconv.ParseFloat(*p, 64)
	if err != nil {
		return -1
	}
	return int(f * 10000)
}

func scoreGoplus(t goplusToken) Report {
	r := Report{Source: "goplus"}

	// The core question. If GoPlus can't answer it, the whole report is
	// "unknown" — flags alone must not read as a pass.
	if t.IsHoneypot == nil {
		r.Unknown = true
		r.Reason = "GoPlus could not determine honeypot status"
	}
	if set(t.IsHoneypot) {
		r.Honeypot = true
		r.Flags = append(r.Flags, Flag{Key: "honeypot", Desc: "honeypot: selling is blocked", Severity: LevelHigh})
	}
	if set(t.CannotSellAll) {
		r.Flags = append(r.Flags, Flag{Key: "cannot_sell_all", Desc: "cannot sell the full balance", Severity: LevelHigh})
	}

	r.BuyTaxBps = taxBps(t.BuyTax)
	r.SellTaxBps = taxBps(t.SellTax)
	if r.SellTaxBps >= 1000 {
		r.Flags = append(r.Flags, Flag{Key: "sell_tax", Desc: fmt.Sprintf("sell tax %.1f%%", float64(r.SellTaxBps)/100), Severity: LevelHigh})
	} else if r.SellTaxBps >= 500 {
		r.Flags = append(r.Flags, Flag{Key: "sell_tax", Desc: fmt.Sprintf("sell tax %.1f%%", float64(r.SellTaxBps)/100), Severity: LevelMedium})
	}
	if r.BuyTaxBps >= 500 {
		r.Flags = append(r.Flags, Flag{Key: "buy_tax", Desc: fmt.Sprintf("buy tax %.1f%%", float64(r.BuyTaxBps)/100), Severity: LevelMedium})
	}

	high := map[*string]Flag{
		t.OwnerChangeBalance: {Key: "owner_change_balance", Desc: "owner can edit holder balances", Severity: LevelHigh},
		t.HiddenOwner:        {Key: "hidden_owner", Desc: "hidden owner", Severity: LevelHigh},
		t.SelfDestruct:       {Key: "selfdestruct", Desc: "contract can self-destruct", Severity: LevelHigh},
	}
	for p, f := range high {
		if set(p) {
			r.Flags = append(r.Flags, f)
		}
	}
	medium := map[*string]Flag{
		t.IsMintable:           {Key: "mintable", Desc: "owner can mint new supply", Severity: LevelMedium},
		t.CanTakeBackOwnership: {Key: "take_back_ownership", Desc: "ownership can be reclaimed", Severity: LevelMedium},
		t.TransferPausable:     {Key: "transfer_pausable", Desc: "transfers can be paused", Severity: LevelMedium},
		t.IsBlacklisted:        {Key: "blacklist", Desc: "has a blacklist function", Severity: LevelMedium},
		t.TradingCooldown:      {Key: "trading_cooldown", Desc: "trading cooldown mechanism", Severity: LevelMedium},
		t.IsProxy:              {Key: "proxy", Desc: "proxy contract — logic can change", Severity: LevelMedium},
	}
	for p, f := range medium {
		if set(p) {
			r.Flags = append(r.Flags, f)
		}
	}
	if t.IsOpenSource != nil && *t.IsOpenSource == "0" {
		r.Flags = append(r.Flags, Flag{Key: "closed_source", Desc: "contract source not verified", Severity: LevelMedium})
	}

	sortFlags(r.Flags)
	r.finalize()
	return r
}
