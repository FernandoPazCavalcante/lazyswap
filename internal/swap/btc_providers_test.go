package swap

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
)

type btcTransport func(*http.Request) (*http.Response, error)

func (t btcTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func mockBTCAPI(t *testing.T, handle func(*http.Request) string) {
	t.Helper()
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = btcTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(handle(r)))}, nil
	})
}
func TestActiveBTCProviderCatalog(t *testing.T) {
	p := ActiveBTCProviders()
	if len(p) != 2 || p[0].ID != LIFI || p[1].ID != NEARIntents {
		t.Fatalf("unexpected BTC providers: %+v", p)
	}
	q := p[1].Quote(&Flow{chainKey: "ethereum"}, context.Background(), BTCQuoteRequest{})
	if !errors.Is(q.Err, ErrBTCDestinationRequired) {
		t.Fatalf("missing recipient: %v", q.Err)
	}
}
func TestLIFIAddresslessEstimate(t *testing.T) {
	mockBTCAPI(t, func(r *http.Request) string {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if r.URL.Host != "li.quest" || r.URL.Path != "/v1/advanced/routes" || body["toAddress"] != nil || body["toTokenAddress"] != "bitcoin" || body["fromAmount"] != "100" {
			t.Fatalf("wrong quote request: %v", body)
		}
		return `{"routes":[{"fromChainId":1,"toChainId":20000000000001,"fromAmount":"100","toAmount":"999","toAmountMin":"899","toToken":{"symbol":"BTC","decimals":8},"steps":[{"includedSteps":[{"tool":"thorswap"}]}]},{"fromChainId":1,"toChainId":20000000000001,"fromAmount":"100","toAmount":"345","toAmountMin":"340","toToken":{"symbol":"BTC","decimals":8},"steps":[{"tool":"other","estimate":{"executionDuration":120,"gasCosts":[{"amount":"1000000000000000","token":{"chainId":1,"address":"0x0000000000000000000000000000000000000000"}}]}}]}]}`
	})
	a, err := fetchLIFI(context.Background(), chain.Get("ethereum"), BTCQuoteRequest{From: TokenInfo{Address: NativeSentinel}}, "100")
	if err != nil || a.expected != "345" || a.minimum != "340" || a.seconds != 120 || a.gas != "0.001 ETH" {
		t.Fatalf("estimate: %+v %v", a, err)
	}
}
func TestNearEstimateUsesWalletForRefund(t *testing.T) {
	const recipient = "1BoatSLRHtKNngkdXEeobR76b53LETtpyT"
	const sender = "0x1111111111111111111111111111111111111111"
	mockBTCAPI(t, func(r *http.Request) string {
		if r.URL.Host != "1click.chaindefuser.com" {
			t.Fatalf("wrong API %s", r.URL.Host)
		}
		if r.URL.Path == "/v0/tokens" {
			return `[{"assetId":"origin","blockchain":"eth","decimals":18,"symbol":"ETH"},{"assetId":"dest","blockchain":"btc","decimals":8,"symbol":"BTC"}]`
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if r.URL.Path != "/v0/quote" || body["recipient"] != recipient || body["refundTo"] != sender || body["dry"] != true || body["originAsset"] != "origin" || body["destinationAsset"] != "dest" {
			t.Fatalf("wrong near request: %v", body)
		}
		return `{"quote":{"amountIn":"100","amountOut":"456","minAmountOut":"450","timeEstimate":80}}`
	})
	a, err := fetchNear(context.Background(), chain.Get("ethereum"), BTCQuoteRequest{From: TokenInfo{Address: NativeSentinel, Decimals: 18}, Sender: sender, Destination: recipient}, "100")
	if err != nil || a.expected != "456" || a.minimum != "450" || a.seconds != 80 {
		t.Fatalf("estimate: %+v %v", a, err)
	}
}
