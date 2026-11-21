package swapbtc

import (
	"slices"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

const validBTC = "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh" // 42 chars

func seeded(providers ...Provider) Model {
	m := New(append([]Provider{THORChain}, providers...)...)
	m.SetSize(60, 30)
	m.SetBalances([]balance.TokenBalance{
		{Symbol: "BNB", Address: "native", Decimals: 18, Balance: "1.5", USDValue: "$900.00"},
	})
	return m
}

func keyRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func keyEnter() tea.KeyMsg         { return tea.KeyMsg{Type: tea.KeyEnter} }

func TestFormFlowEmitsQuoteRequest(t *testing.T) {
	m := seeded()

	// token → amount
	m, _ = m.Update(keyEnter())
	if m.field != fieldAmount {
		t.Fatalf("after enter on token, field = %v, want fieldAmount", m.field)
	}
	if !m.Capturing() {
		t.Fatal("should capture input while editing amount")
	}

	// type amount, advance to addr
	m, _ = m.Update(keyRunes("50"))
	m, _ = m.Update(keyEnter())
	if m.field != fieldAddr {
		t.Fatalf("after enter on amount, field = %v, want fieldAddr", m.field)
	}

	// type a valid BTC address, submit
	m, _ = m.Update(keyRunes(validBTC))
	m, cmd := m.Update(keyEnter())
	if m.state != stateQuotes {
		t.Fatalf("after submit, state = %v, want stateQuotes", m.state)
	}
	if cmd == nil {
		t.Fatal("expected a QuoteRequestMsg command")
	}
	msg, ok := cmd().(QuoteRequestMsg)
	if !ok {
		t.Fatalf("expected QuoteRequestMsg, got %T", cmd())
	}
	if msg.From.Symbol != "BNB" || msg.USDAmount != "50" || msg.BTCAddress != validBTC {
		t.Fatalf("bad request: %+v", msg)
	}
}

func TestSubmitWithInvalidInputsRejected(t *testing.T) {
	m := seeded()
	m, _ = m.Update(keyEnter()) // token → amount
	m, _ = m.Update(keyEnter()) // amount empty → addr
	m, _ = m.Update(keyRunes("short"))
	m, cmd := m.Update(keyEnter()) // submit with bad amount + short addr
	if cmd != nil {
		t.Fatal("invalid inputs should not emit a quote request")
	}
	if m.state != stateForm {
		t.Fatalf("state = %v, want stateForm", m.state)
	}
	if m.quoteErr == "" {
		t.Fatal("expected a validation error message")
	}
}

func requestQuotes(t *testing.T, providers ...Provider) Model {
	t.Helper()
	m := seeded(providers...)
	for _, key := range []tea.KeyMsg{keyEnter(), keyRunes("50"), keyEnter(), keyRunes(validBTC), keyEnter()} {
		m, _ = m.Update(key)
	}
	if m.state != stateQuotes {
		t.Fatalf("state = %v, want stateQuotes", m.state)
	}
	return m
}

func quoteFor(provider Provider, sats int64, expiry time.Time) RouteQuote {
	return RouteQuote{
		Provider: provider,
		Quote: swap.FlowQuote{
			FromToken: swap.TokenInfo{Symbol: "BNB"}, USDAmount: "50",
			EstimatedOutputSats: sats, BTCAddress: validBTC, ExpiresAt: expiry,
		},
	}
}

func TestQuotesRankOnlyValidRoutes(t *testing.T) {
	m := requestQuotes(t, "LI.FI", "NEAR Intents", "MAYAChain")
	now := time.Now()
	for _, route := range []RouteQuote{
		quoteFor(THORChain, 10000, now.Add(time.Minute)),
		quoteFor("LI.FI", 14000, now.Add(time.Minute)),
		quoteFor("NEAR Intents", 21000, now.Add(-time.Minute)),
		{Provider: "MAYAChain", Err: errTest{}},
	} {
		m, _ = m.onQuote(QuoteResultMsg{ID: m.requestID, Route: route}, now)
	}
	var providers []Provider
	for _, route := range m.orderedQuotes(now) {
		providers = append(providers, route.Provider)
	}
	if want := []Provider{"LI.FI", THORChain, "NEAR Intents", "MAYAChain"}; !slices.Equal(providers, want) {
		t.Fatalf("ordered providers = %v, want %v", providers, want)
	}
	if m.selected != THORChain {
		t.Fatalf("new quotes moved selection to %q", m.selected)
	}
}

func TestReviewConfirmsOnlySelectedRoute(t *testing.T) {
	m := requestQuotes(t, "LI.FI")
	expiry := time.Now().Add(time.Minute)
	for _, route := range []RouteQuote{quoteFor(THORChain, 10000, expiry), quoteFor("LI.FI", 10500, expiry)} {
		m, _ = m.Update(QuoteResultMsg{ID: m.requestID, Route: route})
	}
	m, _ = m.Update(keyRunes("k"))
	if _, cmd := m.Update(keyRunes("y")); cmd != nil {
		t.Fatal("quote list should require review before executing")
	}
	m, cmd := m.Update(keyEnter())
	if m.state != stateReview || cmd != nil {
		t.Fatal("enter should review the route without executing")
	}
	m, _ = m.Update(QuoteResultMsg{ID: m.requestID, Route: quoteFor("LI.FI", 11000, expiry)})
	_, cmd = m.Update(keyRunes("y"))
	if cmd == nil {
		t.Fatal("expected the confirmed swap request")
	}
	msg, ok := cmd().(ExecuteRequestMsg)
	if !ok || msg.Provider != "LI.FI" || msg.Quote.EstimatedOutputSats != 10500 || msg.Quote.BTCAddress != validBTC {
		t.Fatalf("bad execution request: %+v", msg)
	}
}

func TestExpiredReviewRequiresRefreshAndNewSelection(t *testing.T) {
	m := requestQuotes(t)
	now := time.Now()
	expiry := now.Add(time.Minute)
	route := quoteFor(THORChain, 10000, expiry)
	m, cmd := m.onQuote(QuoteResultMsg{ID: m.requestID, Route: route}, now)
	if cmd == nil {
		t.Fatal("received a valid quote without starting the expiry timer")
	}
	m, _ = m.handleQuoteKey("enter", now)
	m, cmd = m.handleReviewKey("y", expiry)
	if cmd != nil || m.state != stateQuotes {
		t.Fatal("expired review remained executable")
	}
	oldID := m.requestID
	m, cmd = m.Update(keyRunes("r"))
	if cmd == nil || cmd().(QuoteRequestMsg).ID == oldID {
		t.Fatal("refresh did not request a new quote")
	}
	m, _ = m.onQuote(QuoteResultMsg{ID: oldID, Route: route}, now)
	m, cmd = m.handleQuoteKey("enter", now)
	if cmd != nil || m.state != stateQuotes {
		t.Fatal("a stale response reopened review")
	}
}

func TestAmountConfirmStartsEstimate(t *testing.T) {
	m := seeded()
	m, _ = m.Update(keyEnter())     // token → amount
	m, _ = m.Update(keyRunes("50")) // type amount
	m, _ = m.Update(keyEnter())     // confirm amount → addr + estimate
	if m.field != fieldAddr {
		t.Fatalf("field = %v, want fieldAddr", m.field)
	}
	if !m.estimating {
		t.Fatal("confirming amount should start an estimate")
	}
	if m.from.Symbol != "BNB" {
		t.Fatalf("estimate source = %q, want BNB", m.from.Symbol)
	}
}

func TestEstimateResultStoresBTCAndSats(t *testing.T) {
	m := seeded()
	m.estimating = true
	m.field = fieldAddr
	m, _ = m.Update(EstimateResultMsg{Quote: swap.FlowQuote{
		EstimatedOutput:     "0.00072145",
		EstimatedOutputSats: 72145,
	}})
	if m.estimating {
		t.Fatal("estimate result should clear the estimating flag")
	}
	if m.estimate == nil {
		t.Fatal("estimate should be set")
	}
	if m.estimate.EstimatedOutputSats != 72145 {
		t.Fatalf("unexpected estimate: %+v", m.estimate)
	}
}

func TestTypingAmountClearsStaleEstimate(t *testing.T) {
	m := seeded()
	m, _ = m.Update(keyEnter()) // token → amount
	m.estimate = &swap.FlowQuote{EstimatedOutput: "0.001"}
	m, _ = m.Update(keyRunes("7")) // a keystroke invalidates the estimate
	if m.estimate != nil {
		t.Fatal("typing a new amount should drop the stale estimate")
	}
}

func TestFormatSats(t *testing.T) {
	cases := map[int64]string{0: "0", 999: "999", 1000: "1,000", 72145: "72,145", 100000000: "100,000,000"}
	for in, want := range cases {
		if got := formatSats(in); got != want {
			t.Fatalf("formatSats(%d) = %q, want %q", in, got, want)
		}
	}
}

type errTest struct{}

func (errTest) Error() string { return "quote failed" }
