package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// runSwap implements: lazyswap swap <usd> <FROM> <TO> [flags]
func runSwap(args []string) int {
	fs := flag.NewFlagSet("swap", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we print our own errors
	walletFlag := fs.String("wallet", "", "wallet address to swap from")
	chainFlag := fs.String("chain", "", "chain key (default: configured)")
	slipFlag := fs.Float64("slippage", -1, "slippage percent (default: configured)")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")

	// stdlib flag stops at the first positional; loop so flags may appear before,
	// after, or between the three positionals (e.g. `swap 0.50 BNB USDT --chain bsc`).
	var pos []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return die("%v (try: lazyswap swap 0.50 BNB USDT)", err)
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		pos = append(pos, rest[0])
		rest = rest[1:]
	}
	if len(pos) != 3 {
		return die("usage: lazyswap swap <usd> <FROM> <TO>  (e.g. swap 0.50 BNB USDT)")
	}
	usd, fromSym, toSym := pos[0], pos[1], pos[2]
	if v, err := strconv.ParseFloat(usd, 64); err != nil || v <= 0 {
		return die("amount must be a positive USD number, got %q", usd)
	}

	dao, err := wallet.Open()
	if err != nil {
		return die("open database: %v", err)
	}
	defer dao.Close()

	st, err := settings.Load(dao)
	if err != nil {
		return die("load settings: %v", err)
	}

	// Resolve effective chain / slippage (flag > setting).
	chainKey := st.ChainKey
	if *chainFlag != "" {
		if !chain.Has(*chainFlag) {
			return die("unknown chain %q", *chainFlag)
		}
		chainKey = *chainFlag
	}
	slippage := st.Slippage
	if *slipFlag >= 0 {
		slippage = *slipFlag
	}
	c := chain.Get(chainKey)

	fromTok, err := resolveToken(c, fromSym)
	if err != nil {
		return die("%v", err)
	}
	toTok, err := resolveToken(c, toSym)
	if err != nil {
		return die("%v", err)
	}

	// Unlock and resolve the wallet.
	pw, err := readPassword()
	if err != nil {
		return die("%v", err)
	}
	svc, err := wallet.Unlock(dao, pw)
	if err != nil {
		return die("%v", err)
	}
	ws, err := wallet.NewService(dao, svc).FetchAll()
	if err != nil {
		return die("load wallets: %v", err)
	}
	w, err := pickWallet(ws, *walletFlag, st.DefaultWallet)
	if err != nil {
		return die("%v", err)
	}

	// Quote.
	flow, err := swap.NewFlow(chainKey)
	if err != nil {
		return die("connect to %s: %v", c.Name, err)
	}
	defer flow.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	q, err := flow.Quote(ctx, fromTok, toTok, usd, slippage, w.Address)
	if err != nil {
		return die("quote: %v", err)
	}
	printQuote(c, w.Address, q)

	// Confirm.
	if !*yes {
		ok, err := confirm()
		if err != nil {
			return die("%v", err)
		}
		if !ok {
			fmt.Println("aborted.")
			return 0
		}
	}

	// Execute.
	res := flow.Execute(ctx, w.PrivateKey, fromTok, toTok, usd, slippage)
	if !res.Success {
		return die("swap failed: %s", res.Err)
	}
	fmt.Printf("\n✓ swapped — tx %s\n", res.TxHash)
	if url := txURL(c, res.TxHash); url != "" {
		fmt.Printf("  %s\n", url)
	}
	return 0
}

// resolveToken maps a symbol to a swap.TokenInfo for the given chain. The
// chain's native symbol resolves to the native sentinel.
func resolveToken(c chain.Config, symbol string) (swap.TokenInfo, error) {
	up := strings.ToUpper(symbol)
	if up == strings.ToUpper(c.NativeSymbol) {
		return swap.TokenInfo{Symbol: c.NativeSymbol, Address: swap.NativeSentinel, Decimals: c.NativeDecimals}, nil
	}
	if t, ok := c.Tokens[up]; ok {
		return swap.TokenInfo{Symbol: t.Symbol, Address: t.Address, Decimals: t.Decimals}, nil
	}
	return swap.TokenInfo{}, fmt.Errorf("unknown token %q on %s; available: %s", symbol, c.Name, availableSymbols(c))
}

func availableSymbols(c chain.Config) string {
	syms := []string{c.NativeSymbol}
	for k := range c.Tokens {
		syms = append(syms, k)
	}
	sort.Strings(syms)
	return strings.Join(syms, ", ")
}

// pickWallet resolves which wallet to use: --wallet flag > configured default >
// the only wallet. Errors when ambiguous or not found.
func pickWallet(ws []wallet.Wallet, flagAddr, defaultAddr string) (wallet.Wallet, error) {
	if len(ws) == 0 {
		return wallet.Wallet{}, errors.New("no wallets — create one in the TUI (`lazyswap`)")
	}
	if flagAddr != "" {
		if w, ok := findWallet(ws, flagAddr); ok {
			return w, nil
		}
		return wallet.Wallet{}, fmt.Errorf("no wallet with address %s", flagAddr)
	}
	if defaultAddr != "" {
		if w, ok := findWallet(ws, defaultAddr); ok {
			return w, nil
		}
		return wallet.Wallet{}, fmt.Errorf("default wallet %s not found; set one with `lazyswap config set-wallet`", defaultAddr)
	}
	if len(ws) == 1 {
		return ws[0], nil
	}
	return wallet.Wallet{}, errors.New("multiple wallets — set a default (`lazyswap config set-wallet <addr>`) or pass --wallet")
}

func findWallet(ws []wallet.Wallet, addr string) (wallet.Wallet, bool) {
	for _, w := range ws {
		if strings.EqualFold(w.Address, addr) {
			return w, true
		}
	}
	return wallet.Wallet{}, false
}

func printQuote(c chain.Config, walletAddr string, q swap.FlowQuote) {
	fmt.Printf("Swap on %s using %s\n", c.Name, walletAddr)
	fmt.Printf("  spend     %s of %s (%s %s) %s\n",
		q.USDAmountFormatted, q.FromToken.Symbol, q.NetFromTokenAmount, q.FromToken.Symbol, q.FromTokenPriceLine)
	fmt.Printf("  receive   ~%s %s\n", q.EstimatedOutput, q.ToToken.Symbol)
	fmt.Printf("  min recv  %s %s (slippage %.2f%%)\n", q.MinOutput, q.ToToken.Symbol, q.Slippage)
	fmt.Printf("  fee       %s %s (%.2f%%)\n", q.FeeAmount, q.FromToken.Symbol, q.FeePercent)
	if q.NeedsApproval {
		fmt.Printf("  note      token approval will be sent first\n")
	}
}

// readPassword returns $LAZYSWAP_PASSWORD if set, otherwise prompts (no echo).
// In a non-interactive context with no env var, it errors.
func readPassword() (string, error) {
	if pw := os.Getenv("LAZYSWAP_PASSWORD"); pw != "" {
		return pw, nil
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("no password: set LAZYSWAP_PASSWORD or run in a terminal")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(b), nil
}

// confirm prompts y/N on stdin. Refuses (errors) in a non-interactive context
// so a piped swap never executes silently without --yes.
func confirm() (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, errors.New("refusing to execute non-interactively without --yes")
	}
	fmt.Print("\nProceed? [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes", nil
}

// txURL derives the human block-explorer transaction URL from the chain's
// explorer API host.
// ponytail: simple string transform off ExplorerAPIURL; if a chain ever uses
// a non-"api."-prefixed explorer host, give it an explicit field in chain-config.
func txURL(c chain.Config, hash string) string {
	base := strings.TrimSuffix(c.ExplorerAPIURL, "/api")
	base = strings.Replace(base, "://api.", "://", 1)
	if base == "" {
		return ""
	}
	return base + "/tx/" + hash
}
