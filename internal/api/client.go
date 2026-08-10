// Package api is the Go client for the lazyswap backend (SIWE auth + OpenOcean
// swap routing with the 1% fee). The JWT lives in memory only — it is never
// written to disk. All signing happens locally; the private key is used to
// sign the SIWE message and the swap transaction, and never leaves the machine.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// DefaultBaseURL is the production backend. Override with LAZYSWAP_API_URL.
const DefaultBaseURL = "https://api.lazyswap.org"

// BaseURL resolves the backend base URL from the environment.
func BaseURL() string {
	if v := os.Getenv("LAZYSWAP_API_URL"); v != "" {
		return v
	}
	return DefaultBaseURL
}

// Client talks to the lazyswap backend. Safe for concurrent use.
type Client struct {
	base string
	http *http.Client

	mu   sync.Mutex
	jwt  string
	auth *AuthInfo
}

// New returns a client for the given base URL ("" = BaseURL()).
func New(base string) *Client {
	if base == "" {
		base = BaseURL()
	}
	return &Client{base: base, http: &http.Client{Timeout: 15 * time.Second}}
}

// ─── envelope ────────────────────────────────────────────────────────────────

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *apiError       `json:"error"`
}

// Error is a backend-reported failure with its machine code (NO_PASS,
// RATE_LIMIT, UNAUTHORIZED, …).
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// post sends body as JSON and decodes the success envelope into out.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.mu.Lock()
	if c.jwt != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwt)
	}
	c.mu.Unlock()

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("lazyswap api unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("lazyswap api: bad response (HTTP %d)", resp.StatusCode)
	}
	if !env.Success {
		if env.Error != nil {
			return &Error{Code: env.Error.Code, Message: env.Error.Message}
		}
		return fmt.Errorf("lazyswap api: HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

// ─── SIWE auth ───────────────────────────────────────────────────────────────

// AuthInfo is the identity the backend issued after SIWE verification.
type AuthInfo struct {
	Wallet    string  `json:"wallet"`
	Tier      int     `json:"tier"`
	TierName  string  `json:"tierName"`
	Volume30d float64 `json:"volume30d"`
}

type challengeData struct {
	Nonce   string `json:"nonce"`
	Message string `json:"message"`
}

type verifyData struct {
	JWT string `json:"jwt"`
	AuthInfo
}

// SignPersonal produces an EIP-191 personal_sign signature (hex, 0x-prefixed,
// v ∈ {27,28}) over message with the given private key.
func SignPersonal(message, privateKeyHex string) (string, error) {
	key, err := ethcrypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := ethcrypto.Keccak256([]byte(prefixed))
	sig, err := ethcrypto.Sign(hash, key)
	if err != nil {
		return "", err
	}
	sig[64] += 27
	return fmt.Sprintf("0x%x", sig), nil
}

// Authenticate runs the SIWE flow for the wallet: fetch a challenge, sign it
// locally, and exchange the signature for a JWT. Requires a valid LazySwapPass
// (the backend refuses with NO_PASS otherwise).
func (c *Client) Authenticate(ctx context.Context, address, privateKeyHex string) (AuthInfo, error) {
	var ch challengeData
	if err := c.post(ctx, "/api/v1/siwe/challenge", struct{}{}, &ch); err != nil {
		return AuthInfo{}, err
	}
	sig, err := SignPersonal(ch.Message, privateKeyHex)
	if err != nil {
		return AuthInfo{}, err
	}
	var v verifyData
	err = c.post(ctx, "/api/v1/siwe/verify", map[string]string{
		"wallet":    address,
		"message":   ch.Message,
		"signature": sig,
	}, &v)
	if err != nil {
		return AuthInfo{}, err
	}
	c.mu.Lock()
	c.jwt = v.JWT
	info := v.AuthInfo
	c.auth = &info
	c.mu.Unlock()
	return v.AuthInfo, nil
}

// Authenticated reports whether a JWT is held.
func (c *Client) Authenticated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.jwt != ""
}

// Auth returns the last AuthInfo, or nil before Authenticate.
func (c *Client) Auth() *AuthInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.auth
}

// ─── swap routes ─────────────────────────────────────────────────────────────

// SwapRequest addresses one OpenOcean quote/tx. Amount is in base units
// (wei-style, decimal string) of the input token.
type SwapRequest struct {
	Chain            string  `json:"chain"` // OpenOcean chain key, e.g. "bsc"
	InTokenAddress   string  `json:"inTokenAddress"`
	OutTokenAddress  string  `json:"outTokenAddress"`
	AmountDecimals   string  `json:"amountDecimals"`
	GasPriceDecimals string  `json:"gasPriceDecimals"`
	Slippage         float64 `json:"slippage"`
}

// Token is OpenOcean's token descriptor.
type Token struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Decimals uint8  `json:"decimals"`
	USD      string `json:"usd,omitempty"`
}

// Quote is the aggregated route the backend returned (no tx yet).
type Quote struct {
	InToken      Token  `json:"inToken"`
	OutToken     Token  `json:"outToken"`
	InAmount     string `json:"inAmount"`
	OutAmount    string `json:"outAmount"`
	EstimatedGas string `json:"estimatedGas"`
	MinOutAmount string `json:"minOutAmount"`
	PriceImpact  string `json:"price_impact"`
}

type quoteData struct {
	Quote      Quote   `json:"quote"`
	FeePercent float64 `json:"feePercent"`
	Wallet     string  `json:"wallet"`
}

// QuoteResult is a Quote plus the fee the backend will inject on execution.
type QuoteResult struct {
	Quote
	FeePercent float64
}

// SwapQuote fetches the best aggregated route. Requires Authenticate first.
func (c *Client) SwapQuote(ctx context.Context, req SwapRequest) (QuoteResult, error) {
	var d quoteData
	if err := c.post(ctx, "/api/v1/swap/quote", req, &d); err != nil {
		return QuoteResult{}, err
	}
	return QuoteResult{Quote: d.Quote, FeePercent: d.FeePercent}, nil
}

// Tx is the unsigned transaction to sign and broadcast locally.
type Tx struct {
	To       string `json:"to"`
	Data     string `json:"data"`
	Value    string `json:"value"`
	GasPrice string `json:"gasPrice"`
	GasLimit string `json:"gasLimit"`
	ChainID  int64  `json:"chainId"`
}

// TxMeta describes the route behind a Tx.
type TxMeta struct {
	InToken      string  `json:"inToken"`
	OutToken     string  `json:"outToken"`
	InAmount     string  `json:"inAmount"`
	OutAmount    string  `json:"outAmount"`
	MinOutAmount string  `json:"minOutAmount"`
	PriceImpact  string  `json:"priceImpact"`
	FeePercent   float64 `json:"feePercent"`
	FeeAmountUsd float64 `json:"feeAmountUsd"`
}

type txData struct {
	Tx   Tx     `json:"tx"`
	Meta TxMeta `json:"meta"`
}

// SwapTx builds the unsigned swap transaction (with the referrer fee baked
// in). The caller signs and broadcasts it via their own RPC.
func (c *Client) SwapTx(ctx context.Context, req SwapRequest) (Tx, TxMeta, error) {
	var d txData
	if err := c.post(ctx, "/api/v1/swap/tx", req, &d); err != nil {
		return Tx{}, TxMeta{}, err
	}
	return d.Tx, d.Meta, nil
}
