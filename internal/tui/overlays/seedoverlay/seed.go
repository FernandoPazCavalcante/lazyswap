// Package seedoverlay shows a wallet's recovery phrase in two flows:
//
//   - Reveal: re-asks the lazyswap password (the session is already unlocked,
//     but a phrase on screen deserves a fresh check), then shows the words.
//   - Backup: shown right after a phrase is generated, before the wallet is
//     saved. The user must type back two randomly chosen words; only then is
//     ConfirmedMsg emitted and the parent persists the wallet. Esc discards
//     the phrase and nothing is saved.
//
// The overlay never touches the store: the parent supplies the verify func
// and acts on ConfirmedMsg / CloseMsg.
package seedoverlay

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

// CloseMsg ends the overlay with nothing to persist (reveal done, or backup
// abandoned).
type CloseMsg struct{}

// ConfirmedMsg reports that the user proved they wrote the phrase down.
type ConfirmedMsg struct{ Phrase string }

// VerifyFunc checks the password and returns the phrase to reveal.
type VerifyFunc func(password string) (string, error)

type revealedMsg struct {
	phrase string
	err    error
}

type step int

const (
	stepPassword step = iota
	stepShow
	stepQuiz
)

// Model is the overlay state.
type Model struct {
	backup  bool
	step    step
	address string
	phrase  string
	words   []string
	picks   [2]int // 0-based word indices asked in the quiz
	asked   int    // how many quiz words were answered correctly
	verify  VerifyFunc
	input   textinput.Model
	errMsg  string
	busy    bool
	width   int
	height  int
}

// NewReveal starts the reveal flow for the wallet at address.
func NewReveal(address string, verify VerifyFunc) Model {
	m := Model{address: address, verify: verify, step: stepPassword, input: newInput()}
	m.input.EchoMode = textinput.EchoPassword
	m.input.Placeholder = "lazyswap password"
	return m
}

// NewBackup starts the backup flow for a freshly generated phrase. picks are
// the two word positions to quiz; use RandomPicks in production.
func NewBackup(phrase string, picks [2]int) Model {
	m := Model{backup: true, step: stepShow, input: newInput(), picks: picks}
	m.setPhrase(phrase)
	return m
}

// RandomPicks returns two distinct word positions in [0, n), n >= 2.
func RandomPicks(n int) ([2]int, error) {
	a, err := randIntn(n)
	if err != nil {
		return [2]int{}, err
	}
	b, err := randIntn(n - 1)
	if err != nil {
		return [2]int{}, err
	}
	if b >= a {
		b++ // skip a, keeping b uniform over the other positions
	}
	if b < a {
		a, b = b, a
	}
	return [2]int{a, b}, nil
}

func randIntn(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

func newInput() textinput.Model {
	ti := textinput.New()
	ti.CharLimit = 128
	ti.Width = 40
	ti.Prompt = "› "
	ti.Focus()
	return ti
}

func (m *Model) setPhrase(p string) {
	m.phrase = p
	m.words = strings.Fields(p)
}

// Init blinks the cursor.
func (m Model) Init() tea.Cmd { return textinput.Blink }

// SetSize records the terminal size for centering.
func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

// Update handles keys and the async password check.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case revealedMsg:
		return m.onRevealed(msg), nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			return m, closeCmd
		}
		if msg.Type == tea.KeyEnter {
			return m.onEnter()
		}
		if m.step == stepShow {
			return m, nil // the phrase screen takes no text
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func closeCmd() tea.Msg { return CloseMsg{} }

func (m Model) onEnter() (Model, tea.Cmd) {
	switch m.step {
	case stepPassword:
		if m.busy || m.input.Value() == "" {
			return m, nil
		}
		m.busy = true
		m.errMsg = ""
		pw, verify := m.input.Value(), m.verify
		m.input.SetValue("")
		return m, func() tea.Msg {
			p, err := verify(pw)
			return revealedMsg{phrase: p, err: err}
		}
	case stepShow:
		if !m.backup {
			return m, closeCmd
		}
		m.step = stepQuiz
		m.input.Placeholder = "word"
		return m, nil
	default:
		return m.checkQuizWord()
	}
}

func (m Model) onRevealed(msg revealedMsg) Model {
	m.busy = false
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m
	}
	m.setPhrase(msg.phrase)
	m.step = stepShow
	return m
}

func (m Model) checkQuizWord() (Model, tea.Cmd) {
	want := m.words[m.picks[m.asked]]
	got := strings.ToLower(strings.TrimSpace(m.input.Value()))
	m.input.SetValue("")
	if got != want {
		m.errMsg = fmt.Sprintf("that is not word #%d — check your copy and try again", m.picks[m.asked]+1)
		return m, nil
	}
	m.errMsg = ""
	m.asked++
	if m.asked < len(m.picks) {
		return m, nil
	}
	phrase := m.phrase
	return m, func() tea.Msg { return ConfirmedMsg{Phrase: phrase} }
}

// Step reports the current step name (for tests).
func (m Model) Step() string {
	return [...]string{"password", "show", "quiz"}[m.step]
}

// Err returns the inline error, if any (for tests).
func (m Model) Err() string { return m.errMsg }

// View renders the overlay centered on screen.
func (m Model) View() string {
	title := "Recovery phrase"
	if m.backup {
		title = "Back up your new wallet"
	}
	rows := []string{theme.Text().Bold(true).Render(title), ""}
	rows = append(rows, m.body()...)
	box := theme.Border(true).Width(70).Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	out := []string{box}
	if m.errMsg != "" {
		out = append(out, theme.Error().Render(m.errMsg))
	}
	out = append(out, theme.Dim().Render(m.hint()))
	content := lipgloss.JoinVertical(lipgloss.Center, out...)
	if m.width == 0 || m.height == 0 {
		return content
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) body() []string {
	switch m.step {
	case stepPassword:
		msg := "Enter your lazyswap password to show the phrase for " + m.address + "."
		if m.busy {
			msg = "Checking password…"
		}
		return []string{theme.Dim().Render(msg), "", m.input.View()}
	case stepShow:
		return []string{
			theme.Error().Render("Anyone with these words controls the wallet."),
			theme.Dim().Render("Write them down on paper. Never paste them into a website, chat or AI agent."),
			"",
			theme.Text().Render(m.grid()),
		}
	default:
		prompt := fmt.Sprintf("Type word #%d to confirm you wrote the phrase down.", m.picks[m.asked]+1)
		return []string{theme.Dim().Render(prompt), "", m.input.View()}
	}
}

// grid lays the words out in three numbered columns.
func (m Model) grid() string {
	var b strings.Builder
	for i, w := range m.words {
		fmt.Fprintf(&b, "%2d. %-12s", i+1, w)
		if i%3 == 2 {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) hint() string {
	switch {
	case m.step == stepPassword:
		return "enter: show  ·  esc: cancel"
	case m.step == stepShow && m.backup:
		return "enter: I wrote it down  ·  esc: discard (wallet is not created)"
	case m.step == stepShow:
		return "enter/esc: close"
	default:
		return "enter: check  ·  esc: discard (wallet is not created)"
	}
}
