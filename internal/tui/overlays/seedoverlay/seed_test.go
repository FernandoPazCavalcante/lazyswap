package seedoverlay

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const phrase = "abandon ability able about above absent absorb abstract absurd abuse access accident"

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

var enter = tea.KeyMsg{Type: tea.KeyEnter}

// typeLine types s and presses enter, running any resulting command once.
func typeLine(m Model, s string) (Model, tea.Msg) {
	m, _ = m.Update(key(s))
	m, cmd := m.Update(enter)
	if cmd == nil {
		return m, nil
	}
	return m, cmd()
}

func TestBackupQuizConfirmsOnlyWithRightWords(t *testing.T) {
	m := NewBackup(phrase, [2]int{2, 9}) // words #3 "able" and #10 "abuse"
	m, _ = m.Update(enter)
	if m.Step() != "quiz" {
		t.Fatalf("step = %q, want quiz", m.Step())
	}
	m, msg := typeLine(m, "about")
	if msg != nil || m.Err() == "" {
		t.Fatalf("wrong word accepted: msg=%v err=%q", msg, m.Err())
	}
	m, msg = typeLine(m, " ABLE ")
	if msg != nil || m.Err() != "" {
		t.Fatalf("first right word: msg=%v err=%q", msg, m.Err())
	}
	_, msg = typeLine(m, "abuse")
	got, ok := msg.(ConfirmedMsg)
	if !ok || got.Phrase != phrase {
		t.Fatalf("want ConfirmedMsg with the phrase, got %#v", msg)
	}
}

func TestEscClosesWithoutConfirming(t *testing.T) {
	for _, m := range []Model{NewBackup(phrase, [2]int{0, 1}), NewReveal("0xabc", nil)} {
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if _, ok := cmd().(CloseMsg); !ok {
			t.Fatalf("esc did not close")
		}
	}
}

func TestRevealChecksPasswordFirst(t *testing.T) {
	verify := func(pw string) (string, error) {
		if pw != "hunter22" {
			return "", errors.New("invalid password")
		}
		return phrase, nil
	}
	m := NewReveal("0xabc", verify)
	m, _ = m.Update(enter) // empty password is ignored
	if m.Step() != "password" {
		t.Fatalf("empty password advanced to %q", m.Step())
	}
	m, msg := typeLine(m, "wrong")
	m, _ = m.Update(msg)
	if m.Step() != "password" || m.Err() == "" {
		t.Fatalf("wrong password: step=%q err=%q", m.Step(), m.Err())
	}
	m, msg = typeLine(m, "hunter22")
	m, _ = m.Update(msg)
	if m.Step() != "show" || m.Err() != "" {
		t.Fatalf("right password: step=%q err=%q", m.Step(), m.Err())
	}
	if _, cmd := m.Update(enter); cmd == nil {
		t.Fatal("enter on the shown phrase should close")
	} else if _, ok := cmd().(CloseMsg); !ok {
		t.Fatal("reveal enter did not close")
	}
}

func TestRandomPicksDistinctAndInRange(t *testing.T) {
	for range 500 {
		p, err := RandomPicks(12)
		if err != nil {
			t.Fatal(err)
		}
		if p[0] == p[1] || p[0] < 0 || p[1] > 11 || p[0] > p[1] {
			t.Fatalf("bad picks %v", p)
		}
	}
}

// TestViewNeverPanics walks every reachable step and renders it. This asserts
// no layout — only that rendering a legal state does not crash. bubbletea calls
// View after every Update, including the frame between the quiz passing and the
// parent persisting the wallet, which is where an out-of-range pick index once
// took down the whole program.
func TestViewNeverPanics(t *testing.T) {
	reveal := NewReveal("0xabc", func(string) (string, error) { return phrase, nil })
	reveal.SetSize(80, 24)

	backup := NewBackup(phrase, [2]int{2, 9})
	backup.SetSize(80, 24)
	shown, _ := backup.Update(enter)
	firstWord, _ := typeLine(shown, "able")
	confirmed, _ := typeLine(firstWord, "abuse")

	for _, m := range []Model{reveal, shown, firstWord, confirmed} {
		if m.View() == "" {
			t.Fatalf("step %q rendered nothing", m.Step())
		}
	}
	if confirmed.Step() != "done" {
		t.Fatalf("after the quiz: step = %q, want done", confirmed.Step())
	}
}

func TestKeysAreInertWhileTheWalletIsBeingCreated(t *testing.T) {
	m := NewBackup(phrase, [2]int{2, 9})
	m, _ = m.Update(enter)
	m, _ = typeLine(m, "able")
	m, _ = typeLine(m, "abuse")

	// Esc here would discard a wallet the parent is already writing.
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc}); cmd != nil {
		t.Fatalf("esc after confirming produced %#v, want nothing", cmd())
	}
	// A second enter must not re-run the quiz or re-emit ConfirmedMsg.
	next, cmd := m.Update(enter)
	if cmd != nil {
		t.Fatalf("enter after confirming produced %#v, want nothing", cmd())
	}
	if next.Step() != "done" {
		t.Fatalf("step = %q, want done", next.Step())
	}
}
