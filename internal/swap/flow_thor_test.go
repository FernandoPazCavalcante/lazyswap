package swap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
)

// fakeThornode serves the two THORnode endpoints the flow touches.
func fakeThornode(t *testing.T, quoteJSON string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/thorchain/quote/swap":
			_, _ = fmt.Fprint(w, quoteJSON)
		case "/thorchain/inbound_addresses":
			_, _ = fmt.Fprint(w, `[{"chain":"BSC","address":"0x1111111111111111111111111111111111111111","halted":false,"router":"0x2222222222222222222222222222222222222222"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LAZYSWAP_THORNODE_URL", srv.URL)
}

func TestGetThorchainQuote(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	fakeThornode(t, `{"expected_amount_out":"1000000","fees":{"total":"20000"},"outbound_delay_seconds":540,"recommended_min_amount_in":"1"}`)

	f := fakeFlow(t, rpc)
	q, err := f.GetThorchainQuote(context.Background(), bscToken(t, "BNB"), "5", "bc1qexampleaddressxxxxxxxxxxxxxx")
	if err != nil {
		t.Fatalf("thor quote: %v", err)
	}
	if !q.IsThorchain || q.ToToken.Symbol != "BTC" {
		t.Fatalf("not a thor quote: %+v", q)
	}
	if q.EstimatedOutputSats != 1000000 || q.ThorEstimatedSeconds != 540 {
		t.Fatalf("thor fields wrong: %+v", q)
	}
	if q.ThorMemo == "" || q.BTCAddress == "" {
		t.Fatalf("execution quote must carry memo + destination: %+v", q)
	}
	if q.SourceGasBudget != "0.0003 BNB" {
		t.Fatalf("source gas budget = %q, want 0.0003 BNB", q.SourceGasBudget)
	}
}

func TestExecuteThorchainRejectsAnExpiredReviewedQuote(t *testing.T) {
	flow := &Flow{}
	result := flow.ExecuteThorchain(context.Background(), "", FlowQuote{
		IsThorchain: true, BTCAddress: "bc1qexampleaddressxxxxxxxxxxxxxx",
		ThorMemo:  "=:BTC.BTC:bc1qexampleaddressxxxxxxxxxxxxxx:1000",
		ExpiresAt: time.Now().Add(-time.Second),
	})
	if result.Success || !strings.Contains(result.Err, "quote expired") {
		t.Fatalf("expected expiry before any RPC or use of the key: %+v", result)
	}
}

type gasPriceRPC struct{}

func (gasPriceRPC) GasPrice() hexutil.Uint64 { return 1_000_000_000 }

func TestThorSourceGasBudgetUsesSourceCurrency(t *testing.T) {
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	if err := server.RegisterName("eth", gasPriceRPC{}); err != nil {
		t.Fatal(err)
	}
	client := ethclient.NewClient(rpc.DialInProc(server))
	t.Cleanup(client.Close)
	for key, want := range map[string]string{"bsc": "0.0003 BNB", "ethereum": "0.0003 ETH"} {
		flow := Flow{client: client, chain: chain.Get(key)}
		if got := flow.thorSourceGasBudget(context.Background(), 300000); got != want {
			t.Errorf("gas budget on %s = %q, want %q", key, got, want)
		}
	}
}

func TestEstimateThorchain(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	fakeThornode(t, `{"expected_amount_out":"2000000","fees":{"total":"10000"},"outbound_delay_seconds":300,"recommended_min_amount_in":"1"}`)

	f := fakeFlow(t, rpc)
	q, err := f.EstimateThorchain(context.Background(), bscToken(t, "BNB"), "5")
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if !q.IsThorchain || q.EstimatedOutputSats != 2000000 {
		t.Fatalf("estimate wrong: %+v", q)
	}
	if q.ThorMemo != "" || q.BTCAddress != "" {
		t.Fatalf("estimate must not build a memo: %+v", q)
	}
}

func TestThorchainQuoteBelowMinimum(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	// THORnode signals below-minimum via an error message embedding the min
	// (exact wire format pinned by the provider's parser).
	fakeThornode(t, `{"code":3,"message":"amount less than min swap amount (recommended_min_amount_in: 400000000): invalid request"}`)

	f := fakeFlow(t, rpc)
	q, err := f.EstimateThorchain(context.Background(), bscToken(t, "BNB"), "1")
	if err != nil {
		t.Fatalf("below-min must be a soft signal, got err: %v", err)
	}
	if !q.ThorBelowMin || q.ThorMinTokenAmount == "" || q.ThorMinUSD == "" {
		t.Fatalf("below-min fields missing: %+v", q)
	}
}
