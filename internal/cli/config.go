package cli

import (
	"fmt"
	"strconv"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// runConfig handles `config`, `config show`, and the `config set-*` setters.
func runConfig(args []string) int {
	dao, err := wallet.Open()
	if err != nil {
		return die("open database: %v", err)
	}
	defer func() { _ = dao.Close() }()

	sub := "show"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "show":
		return showConfig(dao)
	case "set-wallet":
		return setWallet(dao, args)
	case "set-chain":
		return setChain(dao, args)
	case "set-slippage":
		return setSlippage(dao, args)
	default:
		return die("unknown config subcommand %q", sub)
	}
}

// showConfig prints the current persisted settings.
func showConfig(dao *wallet.DAO) int {
	st, err := settings.Load(dao)
	if err != nil {
		return die("load settings: %v", err)
	}
	walletAddr := st.DefaultWallet
	if walletAddr == "" {
		walletAddr = "(unset — uses the only wallet, or pass --wallet)"
	}
	fmt.Printf("chain      %s (%s)\n", st.ChainKey, chain.Get(st.ChainKey).Name)
	fmt.Printf("slippage   %s%%\n", strconv.FormatFloat(st.Slippage, 'f', -1, 64))
	fmt.Printf("wallet     %s\n", walletAddr)
	return 0
}

// setWallet handles `config set-wallet <address>`.
func setWallet(dao *wallet.DAO, args []string) int {
	if len(args) != 2 {
		return die("usage: lazyswap config set-wallet <address>")
	}
	addr := args[1]
	if _, err := dao.GetByAddress(addr); err != nil {
		return die("no wallet with address %s (run `lazyswap wallets`)", addr)
	}
	if err := settings.SetDefaultWallet(dao, addr); err != nil {
		return die("save: %v", err)
	}
	fmt.Printf("default wallet set to %s\n", addr)
	return 0
}

// setChain handles `config set-chain <key>`.
func setChain(dao *wallet.DAO, args []string) int {
	if len(args) != 2 {
		return die("usage: lazyswap config set-chain <key>")
	}
	if err := settings.SetChain(dao, args[1]); err != nil {
		return die("%v", err)
	}
	fmt.Printf("default chain set to %s\n", args[1])
	return 0
}

// setSlippage handles `config set-slippage <pct>`.
func setSlippage(dao *wallet.DAO, args []string) int {
	if len(args) != 2 {
		return die("usage: lazyswap config set-slippage <pct>")
	}
	v, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return die("slippage must be a number: %v", err)
	}
	if err := settings.SetSlippage(dao, v); err != nil {
		return die("%v", err)
	}
	fmt.Printf("default slippage set to %s%%\n", args[1])
	return 0
}
