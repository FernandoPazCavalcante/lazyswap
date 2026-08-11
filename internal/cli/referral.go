package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FernandoPazCavalcante/lazyswap/internal/api"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// runReferral implements: lazyswap referral <code|status|apply CODE|claim>
//
// Every subcommand SIWE-authenticates with the default wallet's key
// (stateless — nothing stored on disk), exactly like API-mode swaps.
func runReferral(args []string) int {
	if len(args) == 0 {
		return die("usage: lazyswap referral <code|status|apply CODE|claim>")
	}

	client, err := referralAuth()
	if err != nil {
		return die("%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch args[0] {
	case "code":
		return referralCode(ctx, client)
	case "status":
		return referralStatus(ctx, client)
	case "apply":
		if len(args) != 2 {
			return die("usage: lazyswap referral apply <CODE>")
		}
		return referralApply(ctx, client, args[1])
	case "claim":
		return referralClaim(ctx, client)
	default:
		return die("unknown referral subcommand %q", args[0])
	}
}

// referralAuth unlocks the default wallet and SIWE-authenticates.
func referralAuth() (*api.Client, error) {
	dao, err := wallet.Open()
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = dao.Close() }()

	st, err := settings.Load(dao)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	w, err := unlockSwapWallet(dao, "", st.DefaultWallet)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := api.New("")
	if _, err := client.Authenticate(ctx, w.Address, w.PrivateKey); err != nil {
		return nil, fmt.Errorf("api auth: %w", err)
	}
	return client, nil
}

func referralCode(ctx context.Context, client *api.Client) int {
	info, err := client.ReferralCode(ctx)
	if err != nil {
		return die("referral code: %v", err)
	}
	fmt.Printf("code    %s\n", info.Code)
	fmt.Printf("invite  %s\n", info.InviteLink)
	return 0
}

func referralStatus(ctx context.Context, client *api.Client) int {
	st, err := client.ReferralStats(ctx)
	if err != nil {
		return die("referral status: %v", err)
	}
	code := st.Code
	if code == "" {
		code = "(none — run `lazyswap referral code`)"
	}
	fmt.Printf("code        %s\n", code)
	fmt.Printf("referred    %d wallet(s), %d swap(s)\n", st.TotalReferred, st.TotalSwaps)
	fmt.Printf("earned      $%.2f (claimed $%.2f)\n", st.TotalEarned, st.TotalClaimed)
	fmt.Printf("claimable   $%.2f (minimum $%.0f)\n", st.Claimable, st.MinClaim)
	if st.CanClaim {
		fmt.Println("→ claim with `lazyswap referral claim`")
	}
	return 0
}

func referralApply(ctx context.Context, client *api.Client, code string) int {
	if err := client.ReferralApply(ctx, code); err != nil {
		var apiErr *api.Error
		if errors.As(err, &apiErr) {
			return die("apply: %s", apiErr.Message)
		}
		return die("apply: %v", err)
	}
	fmt.Printf("referral code %s applied — your referrer now earns on your swaps\n", code)
	return 0
}

func referralClaim(ctx context.Context, client *api.Client) int {
	res, err := client.ReferralClaim(ctx)
	if err != nil {
		var apiErr *api.Error
		if errors.As(err, &apiErr) {
			return die("claim: %s", apiErr.Message)
		}
		return die("claim: %v", err)
	}
	fmt.Printf("claim %s created — $%.2f pending manual payout\n", res.ClaimID, res.AmountUsd)
	if res.Note != "" {
		fmt.Printf("note: %s\n", res.Note)
	}
	return 0
}
