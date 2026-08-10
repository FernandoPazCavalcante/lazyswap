//go:build e2e

package e2e

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/FernandoPazCavalcante/lazyswap/internal/tui"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// TestTUISmoke boots the real TUI headless on a fresh data dir and drives the
// full first-run flow: password create + confirm, main screen, tab switch,
// clean quit.
//
// teatest wires the program through tea.WithANSICompressor, which flushes to
// the output buffer only when the program closes — so the test drives input
// blind (paced sends) and asserts all markers against FinalOutput, which
// contains every rendered frame. Markers are stable substrings only — never
// layout (CLAUDE.md rule).
func TestTUISmoke(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("LAZYSWAP_DATA_DIR", dataDir)
	t.Setenv("LAZYSWAP_TEST", "1")

	dao, err := wallet.OpenAt(filepath.Join(dataDir, "wallets.db"))
	if err != nil {
		t.Fatalf("open dao: %v", err)
	}
	defer func() { _ = dao.Close() }()

	// bsc_testnet keeps any background balance fetches cheap; they may fail
	// offline and the smoke asserts nothing about them.
	root, err := tui.NewRoot(dao, "bsc_testnet")
	if err != nil {
		t.Fatalf("new root: %v", err)
	}

	tm := teatest.NewTestModel(t, root, teatest.WithInitialTermSize(120, 40))

	type step struct {
		wait time.Duration
		msg  tea.Msg
	}
	steps := []step{
		{time.Second, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(e2ePassword)}},
		{500 * time.Millisecond, tea.KeyMsg{Type: tea.KeyEnter}}, // first password entered
		{time.Second, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(e2ePassword)}},
		{500 * time.Millisecond, tea.KeyMsg{Type: tea.KeyEnter}},              // confirm → PBKDF2 + wallet create
		{3 * time.Second, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}}}, // main screen → Settings tab (number key)
		{time.Second, tea.KeyMsg{Type: tea.KeyCtrlC}},                         // quit
	}
	for _, s := range steps {
		time.Sleep(s.wait)
		tm.Send(s.msg)
	}

	tm.FinalModel(t, teatest.WithFinalTimeout(15*time.Second))
	raw, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	out := string(raw)

	for _, marker := range []string{
		"Create a password",  // first-access login prompt
		"Confirm password",   // second entry step
		"Tokens",             // main screen tab bar → login + wallet create succeeded
		"Slippage tolerance", // settings panel after 'n' tab switch
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("TUI output never showed %q\n--- last 2000 bytes ---\n%s",
				marker, out[max(0, len(out)-2000):])
		}
	}
}
