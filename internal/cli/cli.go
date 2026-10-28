// Package cli implements the non-interactive command surface. main.go routes
// here whenever lazyswap is invoked with arguments; with none, the TUI launches.
//
// All settings (chain, slippage, default wallet) are shared with the TUI via
// the settings package, so `lazyswap config set-*` and the TUI settings tab
// read and write the same persisted values.
package cli

import (
	"fmt"
	"io"
	"os"
)

// version is overridden at release time via -ldflags "-X .../cli.version=...".
var version = "dev"

// Run dispatches a CLI invocation and returns the process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "swap":
		return runSwap(args[1:])
	case "config":
		return runConfig(args[1:])
	case "wallets":
		return runWallets(args[1:])
	case "set":
		return runSet(args[1:])
	case "mcp":
		return runMcp(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	case "version", "--version", "-v":
		fmt.Println(version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "lazyswap: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

// die prints an error to stderr and returns exit code 1.
func die(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "lazyswap: "+format+"\n", a...)
	return 1
}

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `lazyswap — vim-style EVM wallet (TUI + CLI)

Usage:
  lazyswap                              launch the TUI
  lazyswap swap <usd> <FROM> <TO>       swap $<usd> worth of FROM into TO
  lazyswap config [show]                print current settings
  lazyswap config set-wallet <address>  set the default wallet
  lazyswap config set-chain <key>       set the default chain
  lazyswap config set-slippage <pct>    set the default slippage
  lazyswap wallets                      list wallet addresses
  lazyswap wallets reveal               print the recovery phrase (asks for the password)
  lazyswap wallets export --keystore <path>  write an encrypted keystore V3 JSON file
  lazyswap set password                 print an export line; use: eval "$(lazyswap set password)"
  lazyswap mcp                          serve the MCP server for AI agents (stdio)
  lazyswap update                       replace this binary with the newest release
  lazyswap help | version

swap flags:
  --wallet <address>   override the default wallet for this swap
  --chain <key>        override the default chain for this swap
  --slippage <pct>     override the default slippage for this swap
  --yes                skip the y/N confirmation
  --no-safety          skip the pre-swap token risk check
  --safety-block       refuse the swap when token risk is HIGH
  --quote-only         print the quote (and risk check) then exit without executing

wallets reveal / export flags:
  --address <address>  wallet to use (default: configured default, or the only wallet)
  --keystore <path>    export: output file; never overwrites an existing one
  --allow-pipe         reveal: print even when stdout is not a terminal

update flags:
  --check              report what is available, install nothing
  --version <tag>      install this exact release tag instead of the newest

mcp flags:
  --allow-trading        register the swap_execute tool (default: read-only)
  --max-usd <usd>        per-swap USD cap, required with --allow-trading
  --max-usd-total <usd>  cumulative budget for the whole run (default: --max-usd)
  --chain <keys>         comma-separated chain allowlist for trading tools
  --allow-risky          let swap_execute buy HIGH-risk tokens (default: refuse)

Example:
  lazyswap swap 0.50 BNB USDT     # $0.50 of BNB into USDT on the default chain

Environment:
  LAZYSWAP_PASSWORD    wallet password (skips the interactive prompt)
  LAZYSWAP_KEYSTORE_PASSPHRASE  passphrase for "wallets export" (skips the prompt)
  LAZYSWAP_DATA_DIR    data directory (default ~/.lazyswap)
  LAZYSWAP_RPC_URL     override every chain's RPC endpoint
  LAZYSWAP_THORNODE_URL  override the THORNode API endpoint
  LAZYSWAP_API_URL     override the price-monitor backend
  LAZYSWAP_TEST        set to 1 to send the app log to /dev/null
`)
}
