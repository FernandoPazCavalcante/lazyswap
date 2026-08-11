package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/paths"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

var update = flag.Bool("update", false, "rewrite golden files")

// tempDAO gives a throwaway wallet DB routed through paths.Override.
func tempDAO(t *testing.T) *wallet.DAO {
	t.Helper()
	t.Setenv("LAZYSWAP_TEST", "1")
	dir := t.TempDir()
	paths.Override(dir)
	t.Cleanup(func() { paths.Override("") })
	dao, err := wallet.OpenAt(filepath.Join(dir, "wallets.db"))
	if err != nil {
		t.Fatalf("open dao: %v", err)
	}
	t.Cleanup(func() { _ = dao.Close() })
	return dao
}

func TestParseSwapArgs(t *testing.T) {
	// Flags may appear before, between, or after positionals.
	a, err := parseSwapArgs([]string{"--chain", "bsc", "0.50", "BNB", "--yes", "USDT", "--quote-only"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if a.usd != "0.50" || a.fromSym != "BNB" || a.toSym != "USDT" ||
		a.chain != "bsc" || !a.yes || !a.quoteOnly {
		t.Fatalf("parsed wrong: %+v", a)
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"too few positionals", []string{"0.5", "BNB"}, "usage:"},
		{"zero amount", []string{"0", "BNB", "USDT"}, "positive USD"},
		// "-1" is consumed by the stdlib flag parser before positional
		// validation, so it surfaces as an unknown-flag error.
		{"negative amount", []string{"-1", "BNB", "USDT"}, "try: lazyswap swap"},
		{"non-numeric amount", []string{"abc", "BNB", "USDT"}, "positive USD"},
		{"api+direct conflict", []string{"1", "BNB", "USDT", "--api", "--direct"}, "mutually exclusive"},
		{"unknown flag", []string{"1", "BNB", "USDT", "--nope"}, "try: lazyswap swap"},
	}
	for _, c := range cases {
		if _, err := parseSwapArgs(c.args); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want containing %q", c.name, err, c.want)
		}
	}
}

func TestResolveSwapMode(t *testing.T) {
	cases := []struct {
		setting string
		args    swapArgs
		want    string
	}{
		{"", swapArgs{}, settings.SwapModeDirect},
		{settings.SwapModeAPI, swapArgs{}, settings.SwapModeAPI},
		{settings.SwapModeDirect, swapArgs{}, settings.SwapModeDirect},
		{"", swapArgs{apiMode: true}, settings.SwapModeAPI},
		{settings.SwapModeAPI, swapArgs{directMode: true}, settings.SwapModeDirect},
		{settings.SwapModeDirect, swapArgs{apiMode: true}, settings.SwapModeAPI},
	}
	for _, c := range cases {
		got := resolveSwapMode(settings.Settings{SwapMode: c.setting}, c.args)
		if got != c.want {
			t.Errorf("setting=%q args=%+v: got %q want %q", c.setting, c.args, got, c.want)
		}
	}
}

func TestResolveSwapEnv(t *testing.T) {
	dao := tempDAO(t)

	// Defaults: bsc + configured slippage.
	env, err := resolveSwapEnv(dao, swapArgs{fromSym: "BNB", toSym: "USDT", slippage: -1})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if env.chainKey != "bsc" || env.slippage != settings.DefaultSlippage {
		t.Fatalf("defaults wrong: %+v", env)
	}

	// Flag overrides: chain + slippage.
	env, err = resolveSwapEnv(dao, swapArgs{fromSym: "ETH", toSym: "USDT", chain: "ethereum", slippage: 2})
	if err != nil || env.chainKey != "ethereum" || env.slippage != 2 {
		t.Fatalf("overrides wrong: %+v err=%v", env, err)
	}

	// Errors.
	if _, err := resolveSwapEnv(dao, swapArgs{fromSym: "BNB", toSym: "USDT", chain: "nope"}); err == nil {
		t.Fatal("unknown chain must error")
	}
	if _, err := resolveSwapEnv(dao, swapArgs{fromSym: "NOPE", toSym: "USDT", slippage: -1}); err == nil {
		t.Fatal("unknown token must error")
	}
}

// checkGolden compares got with testdata/<name>.golden; -update rewrites it.
func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("output diverges from %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func TestGoldenQuoteOutput(t *testing.T) {
	c := chain.Get("bsc")
	direct := swap.FlowQuote{
		FromToken: swap.TokenInfo{Symbol: "BNB"}, ToToken: swap.TokenInfo{Symbol: "USDT"},
		USDAmountFormatted: "$5.00", FromTokenAmount: "0.008300", NetFromTokenAmount: "0.008288",
		FromTokenPriceLine: "@ $602.42/BNB", EstimatedOutput: "4.985000", MinOutput: "4.960075",
		Slippage: 0.5, FeePercent: 0.15, FeeAmount: "0.000012", NeedsApproval: true,
	}
	var buf bytes.Buffer
	fprintQuote(&buf, c, "0xA502F4896E1b2B93080EabfFC60018b8D089b872", direct)
	checkGolden(t, "quote_direct", buf.Bytes())

	api := direct
	api.Mode = "api"
	api.PriceImpact = "0.08%"
	api.FeePercent = 1
	api.NeedsApproval = false
	buf.Reset()
	fprintQuote(&buf, c, "0xA502F4896E1b2B93080EabfFC60018b8D089b872", api)
	checkGolden(t, "quote_api", buf.Bytes())
}

func TestGoldenUsage(t *testing.T) {
	var buf bytes.Buffer
	usage(&buf)
	checkGolden(t, "help", buf.Bytes())
}

func TestTxURLEmptyExplorer(t *testing.T) {
	got := txURL(chain.Config{ExplorerAPIURL: ""}, "0xdead")
	if got != "" {
		t.Fatalf("empty explorer should yield empty URL, got %q", got)
	}
}
