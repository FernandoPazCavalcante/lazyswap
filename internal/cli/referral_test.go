package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// referralBackend fakes SIWE + referral endpoints for the CLI flow.
func referralBackend(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/siwe/challenge":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"nonce":"n1","message":"sign me\nNonce: n1"}}`)
		case "/api/v1/siwe/verify":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"jwt":"jwt-1","wallet":"0xabc","tier":0,"tierName":"Tourist","volume30d":0}}`)
		case "/api/v1/referrals/code":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"code":"LAZY-AB2C","wallet":"0xabc","inviteLink":"https://lazyswap.org?ref=LAZY-AB2C","createdAt":"t0"}}`)
		case "/api/v1/referrals/me":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"code":"LAZY-AB2C","totalReferred":2,"totalSwaps":5,"totalVolume":500,"totalEarned":110,"totalClaimed":0,"claimable":110,"canClaim":true,"minClaim":100}}`)
		case "/api/v1/referrals/apply":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"applied":"LAZY-ZZZZ"}}`)
		case "/api/v1/referrals/claim":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"claimId":"cl1","amountUsd":110,"status":"pending","note":"manual payout from treasury"}}`)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LAZYSWAP_API_URL", srv.URL)
}

func TestRunReferralFlow(t *testing.T) {
	referralBackend(t)
	seedEncryptedWallet(t)

	out, code := captureStdout(t, func() int { return runReferral([]string{"code"}) })
	if code != 0 || !strings.Contains(out, "LAZY-AB2C") || !strings.Contains(out, "?ref=") {
		t.Fatalf("code: exit %d\n%s", code, out)
	}

	out, code = captureStdout(t, func() int { return runReferral([]string{"status"}) })
	if code != 0 {
		t.Fatalf("status exit %d:\n%s", code, out)
	}
	mustContainAll(t, out, "LAZY-AB2C", "2 wallet(s)", "$110.00", "referral claim")

	out, code = captureStdout(t, func() int { return runReferral([]string{"apply", "LAZY-ZZZZ"}) })
	if code != 0 || !strings.Contains(out, "applied") {
		t.Fatalf("apply: exit %d\n%s", code, out)
	}

	out, code = captureStdout(t, func() int { return runReferral([]string{"claim"}) })
	if code != 0 || !strings.Contains(out, "cl1") || !strings.Contains(out, "$110.00") {
		t.Fatalf("claim: exit %d\n%s", code, out)
	}
}

func TestRunReferralUsageErrors(t *testing.T) {
	referralBackend(t)
	seedEncryptedWallet(t)

	if code := runReferral(nil); code != 1 {
		t.Fatalf("no args exit %d", code)
	}
	if _, code := captureStdout(t, func() int { return runReferral([]string{"apply"}) }); code != 1 {
		t.Fatalf("apply without code exit %d", code)
	}
	if _, code := captureStdout(t, func() int { return runReferral([]string{"frobnicate"}) }); code != 1 {
		t.Fatalf("unknown subcommand exit %d", code)
	}
}

func mustContainAll(t *testing.T, out string, markers ...string) {
	t.Helper()
	for _, m := range markers {
		if !strings.Contains(out, m) {
			t.Fatalf("output missing %q:\n%s", m, out)
		}
	}
}
