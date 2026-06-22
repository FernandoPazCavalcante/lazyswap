package cli

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// runSet handles `lazyswap set password`. A child process cannot mutate its
// parent shell's environment, so the only way to "set LAZYSWAP_PASSWORD" is to
// print an export line the caller evaluates:
//
//	eval "$(lazyswap set password)"
//
// To avoid leaking the password to the visible terminal, the export line is only
// printed when stdout is captured (not a TTY); run bare, it refuses and shows the
// eval form instead.
func runSet(args []string) int {
	if len(args) == 0 || args[0] != "password" {
		return die("usage: lazyswap set password")
	}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		// stdout is the screen, not captured — printing the password would leak it.
		return die("run this so your shell captures the password:\n  eval \"$(lazyswap set password)\"")
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return die("set password must run in a terminal")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return die("read password: %v", err)
	}

	fmt.Printf("export LAZYSWAP_PASSWORD=%s\n", shellSingleQuote(string(b)))
	return 0
}

// shellSingleQuote wraps s so it is safe as a single shell word, handling any
// embedded single quotes ('  ->  '\”).
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
