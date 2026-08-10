package swap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
)

// ResolveToken maps a symbol to a TokenInfo for the given chain. The chain's
// native symbol resolves to the native sentinel. Case-insensitive.
func ResolveToken(c chain.Config, symbol string) (TokenInfo, error) {
	up := strings.ToUpper(symbol)
	if up == strings.ToUpper(c.NativeSymbol) {
		return TokenInfo{Symbol: c.NativeSymbol, Address: NativeSentinel, Decimals: c.NativeDecimals}, nil
	}
	if t, ok := c.Tokens[up]; ok {
		return TokenInfo{Symbol: t.Symbol, Address: t.Address, Decimals: t.Decimals}, nil
	}
	return TokenInfo{}, fmt.Errorf("unknown token %q on %s; available: %s", symbol, c.Name, AvailableSymbols(c))
}

// AvailableSymbols lists the chain's swappable symbols (native + configured
// tokens), sorted, comma-separated.
func AvailableSymbols(c chain.Config) string {
	syms := []string{c.NativeSymbol}
	for k := range c.Tokens {
		syms = append(syms, k)
	}
	sort.Strings(syms)
	return strings.Join(syms, ", ")
}
