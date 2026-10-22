package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// keystorePassphraseEnv supplies the export passphrase non-interactively.
const keystorePassphraseEnv = "LAZYSWAP_KEYSTORE_PASSPHRASE"

// stdoutIsTerminal reports whether stdout is a TTY. A var so tests can stand
// in for a terminal; the reveal refuses a pipe by default because a recovery
// phrase in a pipe tends to end up in a log, a CI artifact or shell history.
var stdoutIsTerminal = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// runWallets dispatches `wallets`, `wallets reveal` and `wallets export`.
func runWallets(args []string) int {
	if len(args) == 0 || args[0] == "list" {
		return listWallets()
	}
	switch args[0] {
	case "reveal":
		return revealWallet(args[1:])
	case "export":
		return exportWallet(args[1:])
	default:
		return die("unknown wallets subcommand %q (list | reveal | export)", args[0])
	}
}

// listWallets prints every wallet address. Addresses are stored plaintext, so
// this needs no password.
func listWallets() int {
	dao, err := wallet.Open()
	if err != nil {
		return die("open database: %v", err)
	}
	defer func() { _ = dao.Close() }()

	ws, err := dao.FetchAll()
	if err != nil {
		return die("fetch wallets: %v", err)
	}
	if len(ws) == 0 {
		fmt.Println("no wallets yet — create one in the TUI (`lazyswap`)")
		return 0
	}
	st, _ := settings.Load(dao)
	for _, w := range ws {
		marker := "  "
		if w.Address == st.DefaultWallet {
			marker = "* "
		}
		fmt.Printf("%s%s\n", marker, w.Address)
	}
	return 0
}

// walletsArgs holds the parsed reveal/export flags.
type walletsArgs struct {
	address   string
	keystore  string
	allowPipe bool
}

func parseWalletsArgs(name string, args []string) (walletsArgs, error) {
	var a walletsArgs
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&a.address, "address", "", "wallet address (default: configured default, or the only wallet)")
	fs.StringVar(&a.keystore, "keystore", "", "output path for the keystore V3 JSON file")
	fs.BoolVar(&a.allowPipe, "allow-pipe", false, "print the phrase even when stdout is not a terminal")
	if err := fs.Parse(args); err != nil {
		return a, err
	}
	if fs.NArg() > 0 {
		return a, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return a, nil
}

// unlockedWallet opens the store, verifies the password and resolves the
// target wallet (explicit > configured default > only wallet). The returned
// close func releases the database.
func unlockedWallet(explicit string) (*wallet.Service, wallet.Wallet, func(), error) {
	dao, err := wallet.Open()
	if err != nil {
		return nil, wallet.Wallet{}, nil, fmt.Errorf("open database: %w", err)
	}
	closeFn := func() { _ = dao.Close() }
	svc, w, err := pickUnlocked(dao, explicit)
	if err != nil {
		closeFn()
		return nil, wallet.Wallet{}, nil, err
	}
	return svc, w, closeFn, nil
}

func pickUnlocked(dao *wallet.DAO, explicit string) (*wallet.Service, wallet.Wallet, error) {
	st, _ := settings.Load(dao)
	pw, err := readPassword()
	if err != nil {
		return nil, wallet.Wallet{}, err
	}
	c, err := wallet.Unlock(dao, pw)
	if err != nil {
		return nil, wallet.Wallet{}, err
	}
	svc := wallet.NewService(dao, c)
	ws, err := svc.FetchAll()
	if err != nil {
		return nil, wallet.Wallet{}, fmt.Errorf("load wallets: %w", err)
	}
	w, err := wallet.Pick(ws, explicit, st.DefaultWallet)
	return svc, w, err
}

// revealWallet implements: lazyswap wallets reveal [--address X] [--allow-pipe]
func revealWallet(args []string) int {
	a, err := parseWalletsArgs("wallets reveal", args)
	if err != nil {
		return die("%v", err)
	}
	if !a.allowPipe && !stdoutIsTerminal() {
		return die("refusing to print a recovery phrase to a pipe or file (pass --allow-pipe if you mean it)")
	}
	svc, w, closeFn, err := unlockedWallet(a.address)
	if err != nil {
		return die("%v", err)
	}
	defer closeFn()
	phrase, err := svc.Reveal(w.Address)
	if err != nil {
		return die("%v", err)
	}
	fmt.Fprintln(os.Stderr, "Anyone with these words controls this wallet. Write them down offline;")
	fmt.Fprintln(os.Stderr, "never paste them into a website, a chat or an AI agent.")
	fmt.Fprintf(os.Stderr, "\nwallet %s\n\n", w.Address)
	fmt.Println(phrase)
	return 0
}

// exportWallet implements: lazyswap wallets export --keystore <path> [--address X]
func exportWallet(args []string) int {
	a, err := parseWalletsArgs("wallets export", args)
	if err != nil {
		return die("%v", err)
	}
	if a.keystore == "" {
		return die("usage: lazyswap wallets export --keystore <path> [--address <address>]")
	}
	svc, w, closeFn, err := unlockedWallet(a.address)
	if err != nil {
		return die("%v", err)
	}
	defer closeFn()
	pass, err := readKeystorePassphrase()
	if err != nil {
		return die("%v", err)
	}
	if err := svc.ExportKeystore(w.Address, pass, a.keystore); err != nil {
		return die("%v", err)
	}
	fmt.Printf("✓ wrote keystore for %s to %s (mode 0600)\n", w.Address, a.keystore)
	return 0
}

// readKeystorePassphrase returns $LAZYSWAP_KEYSTORE_PASSPHRASE if set,
// otherwise prompts twice without echo. It is deliberately a separate secret
// from the lazyswap password: the file is meant to leave this machine.
func readKeystorePassphrase() (string, error) {
	if p := os.Getenv(keystorePassphraseEnv); p != "" {
		return p, nil
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("no keystore passphrase: set %s or run in a terminal", keystorePassphraseEnv)
	}
	first, err := promptHidden(fd, "Keystore passphrase: ")
	if err != nil {
		return "", err
	}
	again, err := promptHidden(fd, "Repeat passphrase: ")
	if err != nil {
		return "", err
	}
	if first != again {
		return "", errors.New("passphrases do not match")
	}
	return first, nil
}

func promptHidden(fd int, label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	return string(b), nil
}
