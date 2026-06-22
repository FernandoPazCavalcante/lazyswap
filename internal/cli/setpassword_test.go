package cli

import "testing"

func TestShellSingleQuote(t *testing.T) {
	cases := map[string]string{
		"hunter2":    `'hunter2'`,
		"with space": `'with space'`,
		`a$b"c`:      `'a$b"c'`,         // $ and " are literal inside single quotes
		`it's a key`: `'it'\''s a key'`, // embedded single quote
		`''`:         `''\'''\'''`,
	}
	for in, want := range cases {
		if got := shellSingleQuote(in); got != want {
			t.Errorf("shellSingleQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
