package mainscreen

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/safety"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/importoverlay"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/swapoverlay"
)

// handleSwapMsg handles import / swap overlay requests plus quote, safety, and
// execution results. handled=false for any other message.
func (m Model) handleSwapMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case importoverlay.CancelMsg:
		m.mode = modeNormal
		return m, nil, true
	case importoverlay.SubmitMsg:
		next, cmd := m.onImportSubmit(msg)
		return next, cmd, true
	case swapoverlay.CancelMsg:
		m.mode = modeNormal
		return m, nil, true
	case swapoverlay.QuoteRequestMsg:
		next, cmd := m.onSwapQuoteRequest(msg)
		return next, cmd, true
	case swapoverlay.ModeToggleMsg:
		next, cmd := m.onSwapModeToggle(msg)
		return next, cmd, true
	case swapoverlay.ExecuteRequestMsg:
		next, cmd := m.onSwapExecuteRequest(msg)
		return next, cmd, true
	case swapQuoteMsg:
		next, cmd := m.onSwapQuoteResult(msg)
		return next, cmd, true
	case safetyMsg:
		m.swap, _ = m.swap.Update(swapoverlay.SafetyResultMsg{Report: msg.report})
		return m, nil, true
	case swapExecMsg:
		next, cmd := m.onSwapExecResult(msg)
		return next, cmd, true
	}
	return m, nil, false
}

func (m Model) onImportSubmit(msg importoverlay.SubmitMsg) (Model, tea.Cmd) {
	if msg.Phrase == "" {
		m.imp.SetErr("empty mnemonic")
		return m, nil
	}
	return m, importCmd(m.svc, msg.Phrase)
}

func (m Model) onSwapQuoteRequest(msg swapoverlay.QuoteRequestMsg) (Model, tea.Cmd) {
	if m.current == nil {
		return m, nil
	}
	return m, m.quoteCmds(msg.From, msg.To, msg.USDAmount)
}

func (m Model) onSwapModeToggle(msg swapoverlay.ModeToggleMsg) (Model, tea.Cmd) {
	if m.current == nil {
		return m, nil
	}
	if m.useAPIRoute() {
		m.swapMode = settings.SwapModeDirect
	} else {
		m.swapMode = settings.SwapModeAPI
	}
	m.persistSwapMode(m.swapMode)
	return m, m.quoteCmds(msg.From, msg.To, msg.USDAmount)
}

func (m Model) onSwapExecuteRequest(msg swapoverlay.ExecuteRequestMsg) (Model, tea.Cmd) {
	if m.current == nil {
		return m, nil
	}
	applog.Tracef("mainscreen — executing %s swap %s → %s $%s for %s",
		m.lastQuoteMode, msg.From.Symbol, msg.To.Symbol, msg.USDAmount, m.current.Address)
	// Execute over the route that produced the quote the user confirmed.
	if m.lastQuoteMode == "api" {
		return m, executeAPICmd(m.apiClient, m.flowSvc, m.chainKey, m.current.PrivateKey, msg.From, msg.To, msg.USDAmount, m.slippage)
	}
	return m, executeSwapCmd(m.flowSvc, m.current.PrivateKey, msg.From, msg.To, msg.USDAmount, m.slippage)
}

func (m Model) onSwapQuoteResult(msg swapQuoteMsg) (Model, tea.Cmd) {
	if msg.err == nil {
		m.lastQuoteMode = msg.quote.Mode
	}
	pending := msg.err == nil && safety.ShouldCheck(chain.Get(m.chainKey), msg.quote.ToToken)
	m.swap, _ = m.swap.Update(swapoverlay.QuoteResultMsg{Quote: msg.quote, Err: msg.err, SafetyPending: pending})
	return m, nil
}

func (m Model) onSwapExecResult(msg swapExecMsg) (Model, tea.Cmd) {
	applog.Infof("swap result success=%v tx=%s err=%s", msg.result.Success, msg.result.TxHash, msg.result.Err)
	m.swap, _ = m.swap.Update(swapoverlay.ExecutionResultMsg{Result: msg.result})
	// On success, drop the stale cache AND refetch so the swapped-in token
	// shows up without the user pressing 'r'.
	if msg.result.Success && m.current != nil {
		delete(m.balanceCache, m.current.Address)
		return m, m.balancesCmdForCurrent()
	}
	return m, nil
}
