package swap

import (
	"fmt"
	"strconv"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
)

// maxMinOutputPrecision caps how many fractional digits we render for the
// minimum-output figure. Beyond this the digits are float64 noise, not
// precision — the amounts themselves are integers and tracked in issue #9.
const maxMinOutputPrecision = 8

// minOutputFor computes the minimum acceptable output for a quote, formatted to
// the output token's own precision, and refuses rather than return a figure
// that offers no protection.
//
// This guards two ways the old `FormatFloat(est*(1-slip/100), 'f', 6, 64)`
// could hand the router an amountOutMin of 0 — a swap with no slippage
// protection at all, which is strictly worse than failing:
//
//   - A fixed 6 fractional digits overflows a token with fewer decimals, so
//     balance.ParseUnits rejects the string and the swap dies late, with a
//     parse error, instead of early with an explanation.
//   - A small enough amount rounds to "0.000000", which parses cleanly to
//     zero. The swap then executes accepting *any* output, including dust.
func minOutputFor(estimatedOutput string, slippage float64, decimals uint8) (string, error) {
	est, err := strconv.ParseFloat(estimatedOutput, 64)
	if err != nil {
		return "", fmt.Errorf("unparseable estimated output %q: %w", estimatedOutput, err)
	}
	prec := int(decimals)
	if prec > maxMinOutputPrecision {
		prec = maxMinOutputPrecision
	}
	s := strconv.FormatFloat(est*(1-slippage/100), 'f', prec, 64)

	// Check the value the router would actually receive, not the string.
	v, err := balance.ParseUnits(s, decimals)
	if err != nil {
		return "", fmt.Errorf("min output %q invalid for a %d-decimal token: %w", s, decimals, err)
	}
	if v.Sign() <= 0 {
		return "", fmt.Errorf(
			"refusing to swap with no slippage protection: min output rounds to %s for a %d-decimal token "+
				"(estimated %s, slippage %.2f%%) — try a larger amount",
			s, decimals, estimatedOutput, slippage)
	}
	return s, nil
}
