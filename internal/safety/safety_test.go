package safety

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

// serve returns a GoPlus checker pointed at a stub returning body for any request.
func serve(t *testing.T, status int, body string) *GoPlus {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return &GoPlus{Base: srv.URL}
}

const addr = "0xAbCdEF0000000000000000000000000000000001"

// goplusBody wraps token fields into a full response keyed by the LOWERCASED address.
func goplusBody(fields string) string {
	return fmt.Sprintf(`{"code":1,"message":"OK","result":{"%s":{%s}}}`, strings.ToLower(addr), fields)
}

func TestGoplusHoneypotIsHigh(t *testing.T) {
	g := serve(t, 200, goplusBody(`"is_honeypot":"1","sell_tax":"0.35","buy_tax":"0"`))
	r, err := g.Check(context.Background(), "bsc", addr)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.Honeypot || r.Level != LevelHigh || r.Unknown {
		t.Fatalf("want high-risk honeypot, got %+v", r)
	}
	if r.SellTaxBps != 3500 {
		t.Fatalf("sell tax bps = %d, want 3500", r.SellTaxBps)
	}
}

func TestGoplusCleanTokenIsLow(t *testing.T) {
	g := serve(t, 200, goplusBody(`"is_honeypot":"0","cannot_sell_all":"0","buy_tax":"0","sell_tax":"0","is_open_source":"1"`))
	r, err := g.Check(context.Background(), "bsc", addr)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if r.Level != LevelLow || r.Unknown || len(r.Flags) != 0 {
		t.Fatalf("want clean low-risk, got %+v", r)
	}
}

func TestGoplusMissingHoneypotFieldIsUnknown(t *testing.T) {
	// Flags string "0"/"1" only; a MISSING core field must yield Unknown, not safe.
	g := serve(t, 200, goplusBody(`"buy_tax":"0","sell_tax":"0"`))
	r, err := g.Check(context.Background(), "bsc", addr)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.Unknown {
		t.Fatalf("missing is_honeypot must be Unknown, got %+v", r)
	}
}

func TestGoplusNoResultIsUnknown(t *testing.T) {
	g := serve(t, 200, `{"code":1,"message":"OK","result":{}}`)
	r, err := g.Check(context.Background(), "bsc", addr)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.Unknown {
		t.Fatalf("empty result must be Unknown, got %+v", r)
	}
}

func TestGoplusTestnetIsUnknown(t *testing.T) {
	g := serve(t, 200, goplusBody(`"is_honeypot":"0"`))
	r, err := g.Check(context.Background(), "bsc_testnet", addr)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.Unknown {
		t.Fatalf("testnet must be Unknown, got %+v", r)
	}
}

func TestGoplusMediumFlags(t *testing.T) {
	g := serve(t, 200, goplusBody(`"is_honeypot":"0","is_mintable":"1","buy_tax":"0","sell_tax":"0"`))
	r, err := g.Check(context.Background(), "bsc", addr)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if r.Level != LevelMedium || len(r.Flags) != 1 || r.Flags[0].Key != "mintable" {
		t.Fatalf("want medium mintable, got %+v", r)
	}
}

func TestServiceFailsClosed(t *testing.T) {
	svc := &Service{checker: serve(t, 500, "boom"), cache: map[string]cached{}}
	r := svc.Check(context.Background(), "bsc", addr)
	if !r.Unknown {
		t.Fatalf("API failure must be Unknown, got %+v", r)
	}
}

func TestServiceCaches(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, goplusBody(`"is_honeypot":"0"`))
	}))
	t.Cleanup(srv.Close)
	svc := &Service{checker: &GoPlus{Base: srv.URL}, cache: map[string]cached{}}
	svc.Check(context.Background(), "bsc", addr)
	svc.Check(context.Background(), "bsc", strings.ToLower(addr)) // same token, different case
	if calls != 1 {
		t.Fatalf("expected 1 upstream call, got %d", calls)
	}
}

func TestShouldCheck(t *testing.T) {
	c := chain.Get("bsc")
	native := swap.TokenInfo{Symbol: c.NativeSymbol, Address: swap.NativeSentinel}
	if ShouldCheck(c, native) {
		t.Fatal("native token must be skipped")
	}
	usdt := swap.TokenInfo{Symbol: "USDT", Address: c.Tokens["USDT"].Address}
	if ShouldCheck(c, usdt) {
		t.Fatal("stablecoin must be skipped")
	}
	meme := swap.TokenInfo{Symbol: "MEME", Address: addr}
	if !ShouldCheck(c, meme) {
		t.Fatal("arbitrary token must be checked")
	}
}

func TestFormatReport(t *testing.T) {
	r := Report{Unknown: true, Reason: "risk check failed"}
	if s := FormatReport(r); !strings.Contains(s, "proceed with caution") {
		t.Fatalf("unknown format wrong: %q", s)
	}
	r2 := scoreGoplus(goplusToken{IsHoneypot: ptr("1")})
	if s := FormatReport(r2); !strings.Contains(s, "HIGH") || !strings.Contains(s, "honeypot") {
		t.Fatalf("high format wrong: %q", s)
	}
}

func ptr(s string) *string { return &s }
