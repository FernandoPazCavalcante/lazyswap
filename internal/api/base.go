// Package api is the Go client for the lazyswap price-monitor backend.
//
// It does NOT build or sign transactions. The swap-routing client that used to
// live here was removed in v0.1.0: it signed transaction data supplied by the
// server, which is a trust relationship a self-custody wallet should not have.
// See docs/FEES.md and SECURITY.md.
package api

import (
	"encoding/json"
	"fmt"
	"os"
)

// DefaultBaseURL is the production backend.
//
// Note it is `.app`, matching the domain the site and infrastructure use.
// Earlier builds shipped `.org` here, which no infrastructure ever served.
// Override with LAZYSWAP_API_URL.
const DefaultBaseURL = "https://api.lazyswap.app"

// BaseURL resolves the backend base URL from the environment.
func BaseURL() string {
	if v := os.Getenv("LAZYSWAP_API_URL"); v != "" {
		return v
	}
	return DefaultBaseURL
}

// apiError is the backend's error object inside the response envelope.
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
