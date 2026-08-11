package swap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseRawTx(t *testing.T) {
	f := &Flow{} // parseRawTx touches f.client only when gasPrice is missing

	p, err := f.parseRawTx(context.Background(), RawTx{
		Value: "1000", GasPrice: "2000000000", GasLimit: "100000", Data: "0xdeadbeef",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.value.String() != "1000" || p.gasPrice.String() != "2000000000" {
		t.Fatalf("value/gasPrice wrong: %+v", p)
	}
	if p.gasLimit != 125000 { // 100000 + 25% pad
		t.Fatalf("gas limit = %d, want 125000 (25%% pad)", p.gasLimit)
	}
	if len(p.data) != 4 {
		t.Fatalf("calldata len = %d, want 4", len(p.data))
	}

	// Empty value defaults to zero.
	p, err = f.parseRawTx(context.Background(), RawTx{GasPrice: "1", GasLimit: "1", Data: "0x"})
	if err != nil || p.value.Sign() != 0 {
		t.Fatalf("empty value: %+v err=%v", p, err)
	}

	// Malformed fields error out.
	if _, err := f.parseRawTx(context.Background(), RawTx{Value: "xx", GasPrice: "1", GasLimit: "1"}); err == nil {
		t.Fatal("bad value must error")
	}
	if _, err := f.parseRawTx(context.Background(), RawTx{GasPrice: "1", GasLimit: "nope"}); err == nil {
		t.Fatal("bad gas limit must error")
	}
	if _, err := f.parseRawTx(context.Background(), RawTx{GasPrice: "1", GasLimit: "1", Data: "0xzz"}); err == nil {
		t.Fatal("bad calldata must error")
	}
}

func TestZeroIfEmpty(t *testing.T) {
	if zeroIfEmpty("") != "0" || zeroIfEmpty("  ") != "0" || zeroIfEmpty("5") != "5" {
		t.Fatal("zeroIfEmpty wrong")
	}
}

func TestThorMinHelpers(t *testing.T) {
	amount, usd := thorMin(400_000_000, 2.0) // 4 tokens at $2
	if amount == "" || usd == "" {
		t.Fatalf("thorMin empty: %q %q", amount, usd)
	}
	if !strings.Contains(usd, "$") {
		t.Fatalf("usd not formatted: %q", usd)
	}

	err := belowMinErr("USDT", 400_000_000, 2.0)
	if err == nil || !strings.Contains(err.Error(), "USDT") {
		t.Fatalf("belowMinErr wrong: %v", err)
	}
}

func TestFlowQuoteJSONTags(t *testing.T) {
	// The MCP/API surface depends on these key names — pin them.
	b, err := json.Marshal(FlowQuote{Mode: "api", PriceImpact: "0.1%"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"fromToken"`, `"usdAmount"`, `"estimatedOutput"`, `"minOutput"`, `"mode":"api"`, `"priceImpact"`} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("FlowQuote JSON missing %s: %s", key, b)
		}
	}
	rb, _ := json.Marshal(FlowResult{Success: true, Err: ""})
	if !strings.Contains(string(rb), `"txHash"`) || strings.Contains(string(rb), `"error"`) {
		t.Fatalf("FlowResult JSON tags wrong: %s", rb)
	}
}

func TestFailResult(t *testing.T) {
	r := failResult(TokenInfo{Symbol: "BNB"}, TokenInfo{Symbol: "USDT"}, "5", "boom")
	if r.Success || r.Err != "boom" || r.FromToken != "BNB" || r.ToToken != "USDT" {
		t.Fatalf("failResult wrong: %+v", r)
	}
}
