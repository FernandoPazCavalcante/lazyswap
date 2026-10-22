package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/safety"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// swapArgs holds the parsed `swap` flags and positionals.
type swapArgs struct {
	usd, fromSym, toSym string
	wallet, chain       string
	slippage            float64
	yes, noSafety       bool
	safetyBlock         bool
	quoteOnly           bool
}

// swapEnv is the resolved settings/chain/token context for one swap.
type swapEnv struct {
	st       settings.Settings
	chainKey string
	slippage float64
	c        chain.Config
	fromTok  swap.TokenInfo
	toTok    swap.TokenInfo
}

// runSwap implements: lazyswap swap <usd> <FROM> <TO> [flags]
func runSwap(args []string) int {
	a, err := parseSwapArgs(args)
	if err != nil {
		return die("%v", err)
	}

	dao, err := wallet.Open()
	if err != nil {
		return die("open database: %v", err)
	}
	defer func() { _ = dao.Close() }()

	env, err := resolveSwapEnv(dao, a)
	if err != nil {
		return die("%v", err)
	}
	w, err := unlockSwapWallet(dao, a.wallet, env.st.DefaultWallet)
	if err != nil {
		return die("%v", err)
	}

	flow, err := swap.NewFlow(env.chainKey)
	if err != nil {
		return die("connect to %s: %v", env.c.Name, err)
	}
	defer flow.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	q, err := flow.Quote(ctx, env.fromTok, env.toTok, a.usd, env.slippage, w.Address)
	if err != nil {
		return die("quote: %v", err)
	}
	printQuote(env.c, w.Address, q)

	if err := runSafetyCheck(ctx, a, env); err != nil {
		return die("%v", err)
	}
	if a.quoteOnly {
		return 0
	}
	return confirmAndExecute(ctx, flow, env, a, w)
}

// parseSwapArgs parses the swap flags and the three positionals into a struct.
func parseSwapArgs(args []string) (swapArgs, error) {
	fs := flag.NewFlagSet("swap", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we print our own errors
	walletFlag := fs.String("wallet", "", "wallet address to swap from")
	chainFlag := fs.String("chain", "", "chain key (default: configured)")
	slipFlag := fs.Float64("slippage", -1, "slippage percent (default: configured)")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	noSafety := fs.Bool("no-safety", false, "skip the pre-swap token risk check")
	safetyBlock := fs.Bool("safety-block", false, "refuse to swap when the risk check comes back HIGH")
	quoteOnly := fs.Bool("quote-only", false, "print the quote (and risk check) then exit without executing")

	// stdlib flag stops at the first positional; loop so flags may appear before,
	// after, or between the three positionals (e.g. `swap 0.50 BNB USDT --chain bsc`).
	var pos []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return swapArgs{}, fmt.Errorf("%v (try: lazyswap swap 0.50 BNB USDT)", err)
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		pos = append(pos, rest[0])
		rest = rest[1:]
	}
	if len(pos) != 3 {
		return swapArgs{}, errors.New("usage: lazyswap swap <usd> <FROM> <TO>  (e.g. swap 0.50 BNB USDT)")
	}
	if v, err := strconv.ParseFloat(pos[0], 64); err != nil || v <= 0 {
		return swapArgs{}, fmt.Errorf("amount must be a positive USD number, got %q", pos[0])
	}
	return swapArgs{
		usd: pos[0], fromSym: pos[1], toSym: pos[2],
		wallet: *walletFlag, chain: *chainFlag, slippage: *slipFlag,
		yes: *yes, noSafety: *noSafety, safetyBlock: *safetyBlock,
		quoteOnly: *quoteOnly,
	}, nil
}

// resolveSwapEnv loads settings and resolves the effective chain, slippage
// (flag > setting), and both tokens.
func resolveSwapEnv(dao *wallet.DAO, a swapArgs) (swapEnv, error) {
	st, err := settings.Load(dao)
	if err != nil {
		return swapEnv{}, fmt.Errorf("load settings: %w", err)
	}
	chainKey := st.ChainKey
	if a.chain != "" {
		if !chain.Has(a.chain) {
			return swapEnv{}, fmt.Errorf("unknown chain %q", a.chain)
		}
		chainKey = a.chain
	}
	slippage := st.Slippage
	if a.slippage >= 0 {
		slippage = a.slippage
	}
	c := chain.Get(chainKey)

	fromTok, err := swap.ResolveToken(c, a.fromSym)
	if err != nil {
		return swapEnv{}, err
	}
	toTok, err := swap.ResolveToken(c, a.toSym)
	if err != nil {
		return swapEnv{}, err
	}
	return swapEnv{st: st, chainKey: chainKey, slippage: slippage, c: c, fromTok: fromTok, toTok: toTok}, nil
}

// unlockSwapWallet prompts for the password, unlocks the store, and picks the
// wallet to swap from (explicit flag > configured default > only wallet).
func unlockSwapWallet(dao *wallet.DAO, explicitAddr, defaultAddr string) (wallet.Wallet, error) {
	pw, err := readPassword()
	if err != nil {
		return wallet.Wallet{}, err
	}
	svc, err := wallet.Unlock(dao, pw)
	if err != nil {
		return wallet.Wallet{}, err
	}
	ws, err := wallet.NewService(dao, svc).FetchAll()
	if err != nil {
		return wallet.Wallet{}, fmt.Errorf("load wallets: %w", err)
	}
	return wallet.Pick(ws, explicitAddr, defaultAddr)
}

// runSafetyCheck risk-checks the token being bought. Advisory by default;
// --safety-block turns a HIGH verdict into a refusal.
func runSafetyCheck(ctx context.Context, a swapArgs, env swapEnv) error {
	if a.noSafety || !safety.ShouldCheck(env.c, env.toTok) {
		return nil
	}
	rep := safety.New().Check(ctx, env.chainKey, env.toTok.Address)
	fmt.Printf("\n%s\n", safety.FormatReport(rep))
	if a.safetyBlock && rep.Level == safety.LevelHigh {
		return errors.New("refusing to swap: token risk is HIGH (drop --safety-block to override)")
	}
	return nil
}

// confirmAndExecute prompts for confirmation (unless --yes), executes the swap
// and prints the resulting tx hash + explorer URL.
func confirmAndExecute(
	ctx context.Context,
	flow *swap.Flow,
	env swapEnv,
	a swapArgs,
	w wallet.Wallet,
) int {
	if !a.yes {
		ok, err := confirm()
		if err != nil {
			return die("%v", err)
		}
		if !ok {
			fmt.Println("aborted.")
			return 0
		}
	}

	res := flow.Execute(ctx, w.PrivateKey, env.fromTok, env.toTok, a.usd, env.slippage)
	if !res.Success {
		return die("swap failed: %s", res.Err)
	}
	fmt.Printf("\n✓ swapped — tx %s\n", res.TxHash)
	if url := txURL(env.c, res.TxHash); url != "" {
		fmt.Printf("  %s\n", url)
	}
	return 0
}

func printQuote(c chain.Config, walletAddr string, q swap.FlowQuote) {
	fprintQuote(os.Stdout, c, walletAddr, q)
}

// fprintQuote renders the quote to w (separated from printQuote for golden tests).
func fprintQuote(w io.Writer, c chain.Config, walletAddr string, q swap.FlowQuote) {
	_, _ = fmt.Fprintf(w, "Swap on %s using %s\n", c.Name, walletAddr)
	_, _ = fmt.Fprintf(w, "  spend     %s of %s (%s %s) %s\n",
		q.USDAmountFormatted, q.FromToken.Symbol, q.FromTokenAmount, q.FromToken.Symbol, q.FromTokenPriceLine)
	_, _ = fmt.Fprintf(w, "  receive   ~%s %s\n", q.EstimatedOutput, q.ToToken.Symbol)
	_, _ = fmt.Fprintf(w, "  min recv  %s %s (slippage %.2f%%)\n", q.MinOutput, q.ToToken.Symbol, q.Slippage)
	if q.NeedsApproval {
		_, _ = fmt.Fprintf(w, "  note      token approval will be sent first\n")
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
