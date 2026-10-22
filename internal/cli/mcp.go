package cli

import (
	"context"
	"flag"
	"io"
	"strings"

	"github.com/FernandoPazCavalcante/lazyswap/internal/mcp"
)

// runMcp implements: lazyswap mcp [--allow-trading --max-usd <usd>] [--chain <keys>]
//
// Serves the MCP stdio server. Read-only by default; --allow-trading registers
// swap_execute and requires LAZYSWAP_PASSWORD plus a --max-usd cap.
// Errors go to stderr via die — stdout belongs to the JSON-RPC stream.
func runMcp(args []string) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	allowTrading := fs.Bool("allow-trading", false, "register the swap_execute tool")
	maxUSD := fs.Float64("max-usd", 0, "per-swap USD cap (required with --allow-trading)")
	maxUSDTotal := fs.Float64("max-usd-total", 0, "cumulative USD budget for this server run (default: same as --max-usd)")
	chains := fs.String("chain", "", "comma-separated chain allowlist for trading tools")
	allowRisky := fs.Bool("allow-risky", false, "let swap_execute buy HIGH-risk tokens (default: refuse)")
	if err := fs.Parse(args); err != nil {
		return die("%v (try: lazyswap mcp --allow-trading --max-usd 20)", err)
	}

	var allow []string
	for _, k := range strings.Split(*chains, ",") {
		if k = strings.TrimSpace(k); k != "" {
			allow = append(allow, k)
		}
	}

	err := mcp.Run(context.Background(), mcp.Options{
		AllowTrading: *allowTrading,
		MaxUSD:       *maxUSD,
		MaxUSDTotal:  *maxUSDTotal,
		Chains:       allow,
		AllowRisky:   *allowRisky,
		Version:      version,
	})
	if err != nil {
		return die("mcp: %v", err)
	}
	return 0
}
