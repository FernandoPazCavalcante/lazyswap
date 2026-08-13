package api

// Monitor is the token-price-monitor side of the backend. Unlike Client it
// authenticates with a bearer API key (no SIWE, no wallet): registering is
// anonymous and the key is the whole identity. Safe for concurrent use.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Monitor talks to the price-monitor routes of the lazyswap backend.
type Monitor struct {
	base string
	http *http.Client

	mu  sync.Mutex
	key string
}

// NewMonitor returns a monitor client for the given base URL ("" = BaseURL())
// holding apiKey ("" until Register).
func NewMonitor(base, apiKey string) *Monitor {
	if base == "" {
		base = BaseURL()
	}
	return &Monitor{base: base, http: &http.Client{Timeout: 15 * time.Second}, key: apiKey}
}

// HasKey reports whether an API key is held.
func (m *Monitor) HasKey() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.key != ""
}

// do sends an optional JSON body and decodes the success envelope into out.
func (m *Monitor) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, m.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	m.mu.Lock()
	if m.key != "" {
		req.Header.Set("Authorization", "Bearer "+m.key)
	}
	m.mu.Unlock()

	resp, err := m.http.Do(req)
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

// MonitorAccount is the identity issued by Register.
type MonitorAccount struct {
	ID     string `json:"id"`
	APIKey string `json:"apiKey"`
}

// Register creates an anonymous monitor account and stores its key on the
// client. The caller is responsible for persisting the account.
func (m *Monitor) Register(ctx context.Context) (MonitorAccount, error) {
	var acc MonitorAccount
	if err := m.do(ctx, http.MethodPost, "/api/v1/auth/register", struct{}{}, &acc); err != nil {
		return MonitorAccount{}, err
	}
	m.mu.Lock()
	m.key = acc.APIKey
	m.mu.Unlock()
	return acc, nil
}

// WatchItem is one monitored token.
type WatchItem struct {
	ID           string  `json:"id"`
	ChainKey     string  `json:"chainKey"`
	TokenAddress string  `json:"tokenAddress"`
	TokenSymbol  string  `json:"tokenSymbol"`
	ThresholdPct float64 `json:"thresholdPct"`
	Direction    string  `json:"direction"`
	BaselineMode string  `json:"baselineMode"`
	IsActive     bool    `json:"isActive"`
}

// Watchlist lists the account's monitored tokens.
func (m *Monitor) Watchlist(ctx context.Context) ([]WatchItem, error) {
	var items []WatchItem
	if err := m.do(ctx, http.MethodGet, "/api/v1/watchlist", nil, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// AddWatchRequest adds one token to the watchlist.
type AddWatchRequest struct {
	ChainKey         string  `json:"chainKey"`
	TokenAddress     string  `json:"tokenAddress"`
	TokenSymbol      string  `json:"tokenSymbol"`
	ThresholdPercent float64 `json:"thresholdPercent"`
}

// AddWatch adds a token and returns the new watchlist item id.
func (m *Monitor) AddWatch(ctx context.Context, req AddWatchRequest) (string, error) {
	var d struct {
		ID string `json:"id"`
	}
	if err := m.do(ctx, http.MethodPost, "/api/v1/watchlist", req, &d); err != nil {
		return "", err
	}
	return d.ID, nil
}

// RemoveWatch deletes a watchlist item.
func (m *Monitor) RemoveWatch(ctx context.Context, id string) error {
	return m.do(ctx, http.MethodDelete, "/api/v1/watchlist/"+id, nil, nil)
}

// TelegramLink is a short-lived code for linking a Telegram account.
type TelegramLink struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expiresAt"`
	DeepLink  string `json:"deepLink"`
}

// StartTelegramLink requests a linking code. The user sends the code to the
// bot (or opens the deep link) and the backend stores their chat id.
func (m *Monitor) StartTelegramLink(ctx context.Context) (TelegramLink, error) {
	var l TelegramLink
	if err := m.do(ctx, http.MethodPost, "/api/v1/notifications/channels/telegram", struct{}{}, &l); err != nil {
		return TelegramLink{}, err
	}
	return l, nil
}
