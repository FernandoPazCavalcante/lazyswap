package mainscreen

import (
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Update ──────────────────────────────────────────────────────────────────

// Update dispatches in a fixed order: app-level messages (service results and
// panel requests) first, then mode-specific handling (overlays and
// confirmations), then normal-mode key bindings, and finally routing to the
// focused panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(ws.Width, ws.Height)
		return m, nil
	}
	if next, cmd, handled := m.handleAppMsg(msg); handled {
		return next, cmd
	}
	// Mode-specific dispatch.
	if next, cmd, handled := m.handleModeMsg(msg); handled {
		return next, cmd
	}
	// Normal mode key bindings.
	if k, ok := msg.(tea.KeyMsg); ok {
		if next, cmd, handled := m.handleNormalKey(k); handled {
			return next, cmd
		}
	}
	// Route to the focused panel.
	if m.focus == focusRight {
		return m.routeToActiveTab(msg)
	}
	var cmd tea.Cmd
	m.panel, cmd = m.panel.Update(msg)
	return m, cmd
}

// handleAppMsg tries each message group in turn. handled=false means the
// message is not an app-level one and falls through to mode / key handling.
func (m Model) handleAppMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	if next, cmd, handled := m.handleWalletMsg(msg); handled {
		return next, cmd, true
	}
	if next, cmd, handled := m.handleSwapMsg(msg); handled {
		return next, cmd, true
	}
	if next, cmd, handled := m.handlePassMsg(msg); handled {
		return next, cmd, true
	}
	if next, cmd, handled := m.handleSwapBTCMsg(msg); handled {
		return next, cmd, true
	}
	if next, cmd, handled := m.handleAlertsMsg(msg); handled {
		return next, cmd, true
	}
	return m.handleSettingsMsg(msg)
}

// handleModeMsg gives the active overlay / confirmation mode first claim on a
// message. Normal mode returns handled=false so keys fall through.
func (m Model) handleModeMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch m.mode {
	case modeImport:
		var cmd tea.Cmd
		m.imp, cmd = m.imp.Update(msg)
		return m, cmd, true
	case modeSwap:
		var cmd tea.Cmd
		m.swap, cmd = m.swap.Update(msg)
		return m, cmd, true
	case modeConfirmCreate:
		next, cmd := m.updateConfirmCreate(msg)
		return next, cmd, true
	case modeConfirmDelete:
		next, cmd := m.updateConfirmDelete(msg)
		return next, cmd, true
	case modeWalletQR:
		next, cmd := m.updateWalletQR(msg)
		return next, cmd, true
	}
	return m, nil, false
}

func (m Model) updateConfirmCreate(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if k.String() == "y" || k.String() == "Y" {
			return m, createCmd(m.svc)
		}
		m.mode = modeNormal
	}
	return m, nil
}

func (m Model) updateConfirmDelete(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if k.String() == "y" || k.String() == "Y" {
			if m.current == nil {
				m.mode = modeNormal
				return m, nil
			}
			return m, deleteCmd(m.svc, m.current.ID)
		}
		m.mode = modeNormal
	}
	return m, nil
}

func (m Model) updateWalletQR(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "c", "y":
			if m.current != nil {
				if err := clipboard.WriteAll(m.current.Address); err != nil {
					m.walletCopyStatus = "copy failed: " + err.Error()
				} else {
					m.walletCopyStatus = "address copied to clipboard"
				}
			}
		case "esc", "q", "enter":
			m.mode = modeNormal
			m.walletCopyStatus = ""
		}
	}
	return m, nil
}

// routeToActiveTab forwards a message to the currently selected right-panel tab.
func (m Model) routeToActiveTab(msg tea.Msg) (Model, tea.Cmd) {
	switch m.activeTab {
	case tabAlerts:
		var cmd tea.Cmd
		m.alerts, cmd = m.alerts.Update(msg)
		return m, cmd
	case tabSettings:
		var cmd tea.Cmd
		m.settings, cmd = m.settings.Update(msg)
		return m, cmd
	case tabSwapBTC:
		var cmd tea.Cmd
		m.swapbtc, cmd = m.swapbtc.Update(msg)
		return m, cmd
	case tabPass:
		var cmd tea.Cmd
		m.pass, cmd = m.pass.Update(msg)
		return m, cmd
	default:
		var cmd tea.Cmd
		m.tokens, cmd = m.tokens.Update(msg)
		return m, cmd
	}
}
