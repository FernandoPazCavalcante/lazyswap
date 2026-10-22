package swap

import "testing"

func TestMinOutputForAcrossTokenDecimals(t *testing.T) {
	cases := []struct {
		name      string
		estimated string
		slippage  float64
		decimals  uint8
		want      string
	}{
		// The old code formatted to a fixed 6 places regardless of the token.
		{"18 decimals caps at 8 places", "4.985", 0.5, 18, "4.96007500"},
		{"8 decimals (WBTC)", "0.05", 0.5, 8, "0.04975000"},
		{"6 decimals (USDC)", "4.985", 0.5, 6, "4.960075"},
		// 2 decimals used to be unswappable: a 6-place string overflowed
		// ParseUnits and the swap died with a parse error.
		{"2 decimals rounds to its own precision", "100", 1, 2, "99.00"},
		{"0 decimals", "100", 1, 0, "99"},
		{"zero slippage keeps the estimate", "4.985", 0, 6, "4.985000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := minOutputFor(c.estimated, c.slippage, c.decimals)
			if err != nil {
				t.Fatalf("minOutputFor(%q, %v, %d): %v", c.estimated, c.slippage, c.decimals, err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestMinOutputForRefusesWhenItWouldBeZero(t *testing.T) {
	// Each of these used to yield amountOutMin = 0 — a swap that accepts any
	// output, including dust. Failing is strictly better.
	cases := []struct {
		name      string
		estimated string
		slippage  float64
		decimals  uint8
	}{
		{"dust on a 2-decimal token", "0.004", 0.5, 2},
		{"dust on a 6-decimal token", "0.0000004", 0.5, 6},
		{"sub-wei-ish on 18 decimals (capped at 8 places)", "0.000000001", 0.5, 18},
		{"100% slippage leaves nothing", "4.985", 100, 6},
		{"zero estimate", "0", 0.5, 18},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := minOutputFor(c.estimated, c.slippage, c.decimals)
			if err == nil {
				t.Fatalf("expected refusal, got min output %q", got)
			}
			if got != "" {
				t.Fatalf("refusal must not return a value, got %q", got)
			}
		})
	}
}

func TestMinOutputForRejectsUnparseableEstimate(t *testing.T) {
	if _, err := minOutputFor("not-a-number", 0.5, 18); err == nil {
		t.Fatal("expected an error for an unparseable estimate")
	}
	// The old code discarded this error with `_`, turning it into a silent zero.
	if _, err := minOutputFor("", 0.5, 18); err == nil {
		t.Fatal("expected an error for an empty estimate")
	}
}
