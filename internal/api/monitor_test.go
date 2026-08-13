package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// monitorServer fakes the backend's monitor routes and records the last
// request's auth header and decoded body.
func monitorServer(t *testing.T, respond map[string]string) (*httptest.Server, *string) {
	t.Helper()
	var lastAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAuth = r.Header.Get("Authorization")
		body, ok := respond[r.Method+" "+r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"NOT_FOUND","message":"nope"}}`))
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &lastAuth
}

func TestMonitorRegisterStoresKey(t *testing.T) {
	srv, _ := monitorServer(t, map[string]string{
		"POST /api/v1/auth/register": `{"success":true,"data":{"id":"u1","apiKey":"key1"}}`,
	})
	m := NewMonitor(srv.URL, "")
	if m.HasKey() {
		t.Fatal("fresh monitor should have no key")
	}
	acc, err := m.Register(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if acc.ID != "u1" || acc.APIKey != "key1" {
		t.Fatalf("bad account: %+v", acc)
	}
	if !m.HasKey() {
		t.Fatal("key should be stored after Register")
	}
}

func TestMonitorWatchlistSendsBearerKey(t *testing.T) {
	srv, lastAuth := monitorServer(t, map[string]string{
		"GET /api/v1/watchlist": `{"success":true,"data":[
			{"id":"w1","chainKey":"bsc","tokenAddress":"0xT","tokenSymbol":"CAKE",
			 "thresholdPct":5,"direction":"both","baselineMode":"rolling","isActive":true}]}`,
	})
	m := NewMonitor(srv.URL, "key1")
	items, err := m.Watchlist(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *lastAuth != "Bearer key1" {
		t.Fatalf("auth header = %q", *lastAuth)
	}
	if len(items) != 1 || items[0].TokenSymbol != "CAKE" || items[0].ThresholdPct != 5 {
		t.Fatalf("bad items: %+v", items)
	}
}

func TestMonitorAddWatchPostsBodyAndReturnsID(t *testing.T) {
	var got AddWatchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":"w9"}}`))
	}))
	t.Cleanup(srv.Close)
	m := NewMonitor(srv.URL, "key1")
	id, err := m.AddWatch(context.Background(), AddWatchRequest{
		ChainKey: "bsc", TokenAddress: "0xT", TokenSymbol: "CAKE", ThresholdPercent: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "w9" {
		t.Fatalf("id = %q", id)
	}
	if got.ChainKey != "bsc" || got.ThresholdPercent != 5 {
		t.Fatalf("bad body: %+v", got)
	}
}

func TestMonitorErrorSurfacesBackendCode(t *testing.T) {
	srv, _ := monitorServer(t, map[string]string{
		"POST /api/v1/watchlist": `{"success":false,"error":{"code":"LIMIT_REACHED","message":"Max 3 tokens for free tier"}}`,
	})
	m := NewMonitor(srv.URL, "key1")
	_, err := m.AddWatch(context.Background(), AddWatchRequest{})
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	if apiErr.Code != "LIMIT_REACHED" {
		t.Fatalf("code = %q", apiErr.Code)
	}
}

func TestMonitorRemoveAndTelegramLink(t *testing.T) {
	srv, _ := monitorServer(t, map[string]string{
		"DELETE /api/v1/watchlist/w1": `{"success":true}`,
		"POST /api/v1/notifications/channels/telegram": `{"success":true,"data":{"code":"123456","expiresAt":"2026-01-01T00:00:00Z","deepLink":"https://t.me/Bot?start=123456"}}`,
	})
	m := NewMonitor(srv.URL, "key1")
	if err := m.RemoveWatch(context.Background(), "w1"); err != nil {
		t.Fatal(err)
	}
	link, err := m.StartTelegramLink(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if link.Code != "123456" || link.DeepLink == "" {
		t.Fatalf("bad link: %+v", link)
	}
}
