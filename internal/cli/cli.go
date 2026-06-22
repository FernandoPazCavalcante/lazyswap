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
	fmt.Fprint(w, `lazyswap — vim-style EVM wallet (TUI + CLI)

Usage:
  lazyswap                              launch the TUI
  lazyswap swap <usd> <FROM> <TO>       swap $<usd> worth of FROM into TO
  lazyswap config [show]                print current settings
  lazyswap config set-wallet <address>  set the default wallet
  lazyswap config set-chain <key>       set the default chain
  lazyswap config set-slippage <pct>    set the default slippage
  lazyswap wallets                      list wallet addresses
  lazyswap set password                 print an export line; use: eval "$(lazyswap set password)"
  lazyswap help | version

swap flags:
  --wallet <address>   override the default wallet for this swap
  --chain <key>        override the default chain for this swap
  --slippage <pct>     override the default slippage for this swap
  --yes                skip the y/N confirmation

Example:
  lazyswap swap 0.50 BNB USDT     # $0.50 of BNB into USDT on the default chain

Environment:
  LAZYSWAP_PASSWORD    wallet password (skips the interactive prompt)
  LAZYSWAP_DATA_DIR    data directory (default ~/.lazyswap)
`)
}
