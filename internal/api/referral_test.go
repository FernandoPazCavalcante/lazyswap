package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func referralServer(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jwt-test" {
			w.WriteHeader(401)
			_, _ = fmt.Fprint(w, `{"success":false,"error":{"code":"UNAUTHORIZED","message":"Missing JWT"}}`)
			return
		}
		switch r.URL.Path {
		case "/api/v1/referrals/code":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"code":"LAZY-AB2C","wallet":"0xabc","inviteLink":"https://lazyswap.org?ref=LAZY-AB2C","createdAt":"t0"}}`)
		case "/api/v1/referrals/me":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"code":"LAZY-AB2C","totalReferred":3,"totalSwaps":7,"totalVolume":800,"totalEarned":160,"totalClaimed":40,"claimable":120,"canClaim":true,"minClaim":100}}`)
		case "/api/v1/referrals/apply":
			w.WriteHeader(409)
			_, _ = fmt.Fprint(w, `{"success":false,"error":{"code":"ALREADY_REFERRED","message":"This wallet already has a referrer"}}`)
		case "/api/v1/referrals/claim":
			_, _ = fmt.Fprint(w, `{"success":true,"data":{"claimId":"cl1","amountUsd":120,"status":"pending","note":"manual payout"}}`)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL)
	c.jwt = "jwt-test"
	return c
}

func TestReferralCodeAndStats(t *testing.T) {
	c := referralServer(t)

	info, err := c.ReferralCode(context.Background())
	if err != nil || info.Code != "LAZY-AB2C" || info.InviteLink == "" {
		t.Fatalf("code: %+v err=%v", info, err)
	}

	st, err := c.ReferralStats(context.Background())
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if st.TotalReferred != 3 || st.Claimable != 120 || !st.CanClaim || st.MinClaim != 100 {
		t.Fatalf("stats parsed wrong: %+v", st)
	}
}

func TestReferralApplySurfacesErrorCode(t *testing.T) {
	c := referralServer(t)
	err := c.ReferralApply(context.Background(), "LAZY-XXXX")
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != "ALREADY_REFERRED" {
		t.Fatalf("want ALREADY_REFERRED, got %v", err)
	}
}

func TestReferralClaim(t *testing.T) {
	c := referralServer(t)
	res, err := c.ReferralClaim(context.Background())
	if err != nil || res.ClaimID != "cl1" || res.AmountUsd != 120 || res.Status != "pending" {
		t.Fatalf("claim: %+v err=%v", res, err)
	}
}
