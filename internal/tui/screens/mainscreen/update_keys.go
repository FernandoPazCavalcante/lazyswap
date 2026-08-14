package mainscreen

import (
	"strconv"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/importoverlay"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/swapoverlay"
)

// handleNormalKey handles the normal-mode key bindings. handled=false lets the
// key fall through to the focused panel.
func (m Model) handleNormalKey(k tea.KeyMsg) (Model, tea.Cmd, bool) {
	// When the active tab is editing a text field, keys go straight to it
	// before any global shortcut runs — otherwise letters like the 'c' in a
	// bech32 BTC address would be swallowed by create/delete/import/swap.
	if m.focus == focusRight && m.capturingInput() {
		next, cmd := m.routeToActiveTab(k)
		return next, cmd, true
	}
	if next, cmd, handled := m.handleTabSwitchKey(k); handled {
		return next, cmd, true
	}
	switch k.String() {
	case "c":
		m.mode = modeConfirmCreate
		return m, nil, true
	case "d":
		if m.current != nil {
			m.mode = modeConfirmDelete
		}
		return m, nil, true
	case "i":
		next, cmd := m.openImportOverlay()
		return next, cmd, true
	case "s":
		next, cmd := m.openSwapOverlay()
		return next, cmd, true
	case "r":
		return m, m.balancesCmdForCurrent(), true
	case "a":
		if m.activeTab == tabTokens {
			next, cmd := m.addSelectedTokenToAlerts()
			return next, cmd, true
		}
		return m, nil, false
	case "y":
		next, cmd := m.copyCurrentAddress()
		return next, cmd, true
	case "tab":
		if m.focus == focusLeft {
			m.focus = focusRight
		} else {
			m.focus = focusLeft
		}
		m.applyFocusStyles()
		return m, nil, true
	}
	return m, nil, false
}

// handleTabSwitchKey switches right-panel tabs on number keys, unless the
// active tab is currently capturing text input (then the digit is a literal
// char).
func (m Model) handleTabSwitchKey(k tea.KeyMsg) (Model, tea.Cmd, bool) {
	if m.capturingInput() {
		return m, nil, false
	}
	n, err := strconv.Atoi(k.String())
	if err != nil || !m.tabExists(n) {
		return m, nil, false
	}
	m.activeTab = tab(n)
	m.focus = focusRight
	m.applyFocusStyles()
	return m, m.onTabActivated(), true
}

func (m Model) openImportOverlay() (Model, tea.Cmd) {
	m.imp = importoverlay.New()
	m.imp.SetSize(m.width, m.height)
	m.mode = modeImport
	return m, m.imp.Init()
}

func (m Model) openSwapOverlay() (Model, tea.Cmd) {
	if m.current == nil || m.flowSvc == nil || m.balSvc == nil {
		return m, nil
	}
	cached, ok := m.balanceCache[m.current.Address]
	if !ok || len(cached) == 0 {
		m.errMsg = "swap: balances not loaded — press 'r' to refresh"
		return m, nil
	}
	m.errMsg = ""
	m.swap = swapoverlay.New(cached, m.balSvc.Chain())
	m.swap.SetSize(m.width, m.height)
	m.mode = modeSwap
	return m, m.swap.Init()
}

func (m Model) copyCurrentAddress() (Model, tea.Cmd) {
	if m.current == nil {
		return m, nil
	}
	if err := clipboard.WriteAll(m.current.Address); err != nil {
		m.errMsg = "copy: " + err.Error()
		m.notice = ""
	} else {
		m.errMsg = ""
		m.notice = "copied " + shortAddr(m.current.Address)
	}
	return m, nil
}
