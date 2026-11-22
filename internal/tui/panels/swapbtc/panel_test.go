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

const THORChain = swap.THORChain

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

func TestFormRequestsQuotesBeforeEnteringAnAddress(t *testing.T) {
	m := seeded()
	m, _ = m.Update(keyEnter())
	if m.field != fieldAmount || !m.Capturing() {
		t.Fatal("expected to be editing the USD amount")
	}
	m, _ = m.Update(keyRunes("50"))
	m, cmd := m.Update(keyEnter())
	if m.state != stateQuotes || m.Capturing() || cmd == nil {
		t.Fatal("confirming the amount should request quotes immediately")
	}
	msg, ok := cmd().(QuoteRequestMsg)
	if !ok || msg.From.Symbol != "BNB" || msg.USDAmount != "50" || msg.BTCAddress != "" {
		t.Fatalf("bad estimate request: %+v", msg)
	}
}

func TestSubmitWithInvalidInputsRejected(t *testing.T) {
	m := seeded()
	m, _ = m.Update(keyEnter()) // token → amount
	m, _ = m.Update(keyRunes("NaN"))
	m, cmd := m.Update(keyEnter()) // reject a non-finite amount
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
	for _, key := range []tea.KeyMsg{keyEnter(), keyRunes("50"), keyEnter()} {
		m, _ = m.Update(key)
	}
	if m.state != stateQuotes {
		t.Fatalf("state = %v, want stateQuotes", m.state)
	}
	return m
}

func (m Model) quoteFor(provider Provider, sats int64, expiry time.Time) RouteQuote {
	return RouteQuote{
		Provider: provider,
		Quote: swap.FlowQuote{
			FromToken: swap.TokenInfo{Symbol: "BNB"}, USDAmount: "50",
			EstimatedOutputSats: sats, BTCAddress: m.destination, ExpiresAt: expiry,
		},
	}
}

func TestQuotesRankOnlyValidRoutes(t *testing.T) {
	m := requestQuotes(t, "LI.FI", "NEAR Intents", "MAYAChain")
	now := time.Now()
	for _, route := range []RouteQuote{
		m.quoteFor(THORChain, 10000, now.Add(time.Minute)),
		m.quoteFor("LI.FI", 14000, now.Add(time.Minute)),
		m.quoteFor("NEAR Intents", 21000, now.Add(-time.Minute)),
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
	m = requestForDestination(t, m)
	expiry := time.Now().Add(time.Minute)
	for _, route := range []RouteQuote{m.quoteFor(THORChain, 10000, expiry), m.quoteFor("LI.FI", 10500, expiry)} {
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
	m, _ = m.Update(QuoteResultMsg{ID: m.requestID, Route: m.quoteFor("LI.FI", 11000, expiry)})
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
	m := requestForDestination(t, requestQuotes(t))
	now := time.Now()
	expiry := now.Add(time.Minute)
	route := m.quoteFor(THORChain, 10000, expiry)
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

func requestForDestination(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = m.Update(keyRunes("a"))
	m, _ = m.Update(keyRunes(validBTC))
	next, cmd := m.Update(keyEnter())
	if cmd == nil || cmd().(QuoteRequestMsg).BTCAddress != validBTC {
		t.Fatal("expected a fresh quote for the BTC address")
	}
	return next
}

func TestEstimatePromptsForAddressBeforeReview(t *testing.T) {
	m := requestQuotes(t)
	expiry := time.Now().Add(time.Minute)
	estimate := m.quoteFor(THORChain, 10000, expiry)
	estimateID := m.requestID
	m, _ = m.Update(QuoteResultMsg{ID: estimateID, Route: estimate})
	m, _ = m.Update(keyEnter())
	if m.state != stateAddress || !m.Capturing() {
		t.Fatal("reviewing an estimate should prompt for a BTC address")
	}

	m, _ = m.Update(keyRunes("short"))
	m, cmd := m.Update(keyEnter())
	if cmd != nil || m.state != stateAddress || m.quoteErr == "" {
		t.Fatal("a short address should leave the user in the address editor")
	}
	m.addr.SetValue(validBTC)
	m, cmd = m.Update(keyEnter())
	if cmd == nil {
		t.Fatal("expected new quotes after entering an address")
	}
	req, ok := cmd().(QuoteRequestMsg)
	if !ok || req.ID == estimateID || req.BTCAddress != validBTC || m.selected != THORChain {
		t.Fatalf("unexpected quote request: %+v", req)
	}

	m, _ = m.Update(QuoteResultMsg{ID: estimateID, Route: estimate})
	m, _ = m.Update(keyEnter())
	if m.state != stateQuotes || !m.selectedQuote().pending() {
		t.Fatal("a destination-less estimate advanced to review after entering an address")
	}
	m, _ = m.Update(QuoteResultMsg{ID: req.ID, Route: m.quoteFor(THORChain, 10500, expiry)})
	m, _ = m.Update(keyEnter())
	if m.state != stateReview || m.selectedQuote().Quote.EstimatedOutputSats != 10500 {
		t.Fatal("expected to review the new, destination-specific values")
	}
}

func TestClearingAddressRequestsEstimatesAgain(t *testing.T) {
	m := requestForDestination(t, requestQuotes(t))
	m, _ = m.Update(keyRunes("a"))
	m.addr.SetValue("")
	m, cmd := m.Update(keyEnter())
	if cmd == nil || cmd().(QuoteRequestMsg).BTCAddress != "" || m.state != stateQuotes {
		t.Fatal("clearing the address should return to the comparison with fresh estimates")
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
