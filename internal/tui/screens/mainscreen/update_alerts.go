package mainscreen

// Alerts-tab plumbing: the panel emits request messages, the parent runs the
// monitor API calls off the UI thread and pushes results back into the panel.

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/api"
	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	alertspanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/alerts"
)

// defaultThresholdPct is applied when adding a token from the Tokens tab.
// Edit via the API (PATCH /watchlist/:id) until the panel grows an editor.
const defaultThresholdPct = 5.0

type monitorRegisteredMsg struct {
	acc api.MonitorAccount
	err error
}

type monitorListMsg struct {
	items []api.WatchItem
	err   error
}

type monitorRemovedMsg struct{ err error }

type monitorTelegramMsg struct {
	link api.TelegramLink
	err  error
}

type monitorAddedMsg struct {
	symbol string
	err    error
}

// handleAlertsMsg handles alerts-tab requests and monitor API results.
// handled=false for any other message.
func (m Model) handleAlertsMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case alertspanel.RegisterRequestMsg:
		return m, m.registerMonitorCmd(), true
	case alertspanel.RemoveRequestMsg:
		return m, m.removeWatchCmd(msg.ID), true
	case alertspanel.TelegramRequestMsg:
		return m, m.telegramLinkCmd(), true
	case monitorRegisteredMsg:
		next := m.onMonitorRegistered(msg)
		return next, next.alertsRefreshCmd(), true
	case monitorListMsg:
		if msg.err != nil {
			m.alerts.SetError(msg.err.Error())
		} else {
			m.alerts.SetItems(msg.items)
		}
		return m, nil, true
	case monitorRemovedMsg:
		if msg.err != nil {
			m.alerts.SetError(msg.err.Error())
			return m, nil, true
		}
		return m, m.alertsRefreshCmd(), true
	case monitorTelegramMsg:
		if msg.err != nil {
			m.alerts.SetError(msg.err.Error())
		} else {
			m.alerts.SetLink(msg.link)
		}
		return m, nil, true
	case monitorAddedMsg:
		if msg.err != nil {
			m.errMsg = "alerts: " + msg.err.Error()
			return m, nil, true
		}
		m.errMsg = ""
		m.notice = msg.symbol + " added to alerts (±5%)"
		if m.activeTab == tabAlerts {
			return m, m.alertsRefreshCmd(), true
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) onMonitorRegistered(msg monitorRegisteredMsg) Model {
	if msg.err != nil {
		m.alerts.SetError(msg.err.Error())
		return m
	}
	m.alerts.SetRegistered(true)
	if m.dao != nil {
		if err := m.dao.SetConfig(cfgMonitorAPIKey, msg.acc.APIKey); err != nil {
			applog.Error("persist monitor key", err)
		}
		if err := m.dao.SetConfig(cfgMonitorUserID, msg.acc.ID); err != nil {
			applog.Error("persist monitor user id", err)
		}
	}
	return m
}

// alertsRefreshCmd fetches the watchlist; nil (and an unregistered panel
// state) when no API key is held yet.
func (m *Model) alertsRefreshCmd() tea.Cmd {
	if !m.monitor.HasKey() {
		m.alerts.SetRegistered(false)
		return nil
	}
	m.alerts.SetLoading()
	mon := m.monitor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		items, err := mon.Watchlist(ctx)
		return monitorListMsg{items: items, err: err}
	}
}

func (m *Model) registerMonitorCmd() tea.Cmd {
	mon := m.monitor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		acc, err := mon.Register(ctx)
		return monitorRegisteredMsg{acc: acc, err: err}
	}
}

func (m *Model) removeWatchCmd(id string) tea.Cmd {
	mon := m.monitor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return monitorRemovedMsg{err: mon.RemoveWatch(ctx, id)}
	}
}

func (m *Model) telegramLinkCmd() tea.Cmd {
	mon := m.monitor
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		link, err := mon.StartTelegramLink(ctx)
		return monitorTelegramMsg{link: link, err: err}
	}
}

// addSelectedTokenToAlerts adds the token highlighted in the Tokens tab to
// the watchlist with the default threshold.
func (m Model) addSelectedTokenToAlerts() (Model, tea.Cmd) {
	if !m.monitor.HasKey() {
		m.errMsg = "alerts: not registered — open the Alerts tab (3) and press R"
		return m, nil
	}
	sel := m.tokens.Selected()
	if sel == nil {
		return m, nil
	}
	if sel.Address == balance.NativeAddress {
		m.errMsg = "alerts: native gas token can't be watched — pick an ERC-20"
		return m, nil
	}
	m.errMsg = ""
	mon := m.monitor
	req := api.AddWatchRequest{
		ChainKey:         m.chainKey,
		TokenAddress:     sel.Address,
		TokenSymbol:      sel.Symbol,
		ThresholdPercent: defaultThresholdPct,
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, err := mon.AddWatch(ctx, req)
		return monitorAddedMsg{symbol: req.TokenSymbol, err: err}
	}
}
