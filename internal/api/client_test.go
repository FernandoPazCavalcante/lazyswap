package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

// testKey is a throwaway key (never funded); address derived below.
const testKey = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"

func testAddress(t *testing.T) string {
	t.Helper()
	pk, err := ethcrypto.HexToECDSA(testKey)
	if err != nil {
		t.Fatal(err)
	}
	return ethcrypto.PubkeyToAddress(pk.PublicKey).Hex()
}

func TestSignPersonalRecoverable(t *testing.T) {
	msg := "lazyswap.org wants you to sign in\n\nNonce: abc-123"
	sigHex, err := SignPersonal(msg, testKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(sigHex, "0x"))
	if err != nil || len(sig) != 65 {
		t.Fatalf("bad signature encoding: %v len=%d", err, len(sig))
	}
	if sig[64] != 27 && sig[64] != 28 {
		t.Fatalf("v byte = %d, want 27/28", sig[64])
	}

	// Recover and compare to the known address (what the backend must do).
	sig[64] -= 27
	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(msg), msg)
	pub, err := ethcrypto.SigToPub(ethcrypto.Keccak256([]byte(prefixed)), sig)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if got := ethcrypto.PubkeyToAddress(*pub).Hex(); got != testAddress(t) {
		t.Fatalf("recovered %s, want %s", got, testAddress(t))
	}
}

func TestAuthenticateHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/siwe/challenge":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"nonce":"n1","message":"sign me\nNonce: n1"}}`)
		case "/api/v1/siwe/verify":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["wallet"] == "" || !strings.HasPrefix(body["signature"], "0x") || body["message"] == "" {
				w.WriteHeader(400)
				_, _ = fmt.Fprint(w, `{"success":false,"error":{"code":"BAD_REQUEST","message":"missing"}}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"jwt":"jwt-1","wallet":"0xabc","tier":1,"tierName":"Trader","volume30d":1234.5}}`)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL)
	info, err := c.Authenticate(context.Background(), testAddress(t), testKey)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if !c.Authenticated() || info.TierName != "Trader" || c.Auth().Tier != 1 {
		t.Fatalf("auth state wrong: %+v", info)
	}
}

func TestBackendErrorCodeSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/siwe/challenge" {
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"nonce":"n1","message":"m\nNonce: n1"}}`)
			return
		}
		w.WriteHeader(403)
		_, _ = fmt.Fprint(w, `{"success":false,"error":{"code":"NO_PASS","message":"No valid LazySwapPass NFT found"}}`)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL)
	_, err := c.Authenticate(context.Background(), testAddress(t), testKey)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != "NO_PASS" {
		t.Fatalf("want *Error NO_PASS, got %v", err)
	}
}

func TestSwapQuoteSendsBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/swap/quote" {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Authorization") != "Bearer jwt-test" {
			w.WriteHeader(401)
			_, _ = fmt.Fprint(w, `{"success":false,"error":{"code":"UNAUTHORIZED","message":"Missing JWT"}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"success":true,"data":{"quote":{"inToken":{"address":"a","symbol":"BNB","decimals":18},"outToken":{"address":"b","symbol":"USDT","decimals":18},"inAmount":"1000000000000000000","outAmount":"600000000000000000000","estimatedGas":"210000","minOutAmount":"597000000000000000000","price_impact":"0.01%"},"feePercent":1,"wallet":"0xabc"}}`)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL)
	c.jwt = "jwt-test"
	q, err := c.SwapQuote(context.Background(), SwapRequest{Chain: "bsc"})
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if q.FeePercent != 1 || q.OutToken.Symbol != "USDT" || q.PriceImpact != "0.01%" {
		t.Fatalf("quote parsed wrong: %+v", q)
	}
}

func TestOOAddressMapsNative(t *testing.T) {
	native := swap.TokenInfo{Symbol: "BNB", Address: swap.NativeSentinel}
	if got := ooAddress(native); got != OONativeAddress {
		t.Fatalf("native mapped to %s", got)
	}
	erc := swap.TokenInfo{Symbol: "USDT", Address: "0x55d3"}
	if got := ooAddress(erc); got != "0x55d3" {
		t.Fatalf("erc20 mapped to %s", got)
	}
}

func TestFormatBase(t *testing.T) {
	if got := formatBase("600000000000000000000", 18); got != "600" {
		t.Fatalf("formatBase = %q, want 600", got)
	}
}
