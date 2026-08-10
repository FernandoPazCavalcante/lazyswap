package mainscreen

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	passpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/lazyswappass"
	settingspanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/settings"
	swapbtcpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/swapbtc"
)

// handlePassMsg handles Lazyswap Pass buy requests and mint / status results.
// handled=false for any other message.
func (m Model) handlePassMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case passpanel.BuyRequestMsg:
		if m.current == nil || m.passSvc == nil {
			return m, nil, true
		}
		applog.Tracef("mainscreen — minting pass for %s", m.current.Address)
		m.pass.SetBuying(true)
		return m, mintPassCmd(m.passSvc, m.current.PrivateKey), true
	case passBoughtMsg:
		if msg.err != nil {
			applog.Error("pass mint", msg.err)
			m.pass.SetError(msg.err.Error())
			return m, nil, true
		}
		applog.Infof("pass minted tx=%s", msg.txHash)
		// Re-read status so the panel flips to "active" with the new expiry.
		return m, m.refreshPassCmd(), true
	case passStatusMsg:
		if msg.err != nil {
			m.pass.SetError(msg.err.Error())
			return m, nil, true
		}
		m.pass.SetStatus(msg.status)
		return m, nil, true
	}
	return m, nil, false
}

// handleSwapBTCMsg handles Swap BTC tab requests and THORchain results.
// handled=false for any other message.
func (m Model) handleSwapBTCMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case swapbtcpanel.EstimateRequestMsg:
		if m.current == nil {
			return m, nil, true
		}
		return m, thorEstimateCmd(m.flowSvc, msg.From, msg.USDAmount), true
	case swapbtcpanel.QuoteRequestMsg:
		if m.current == nil {
			return m, nil, true
		}
		return m, thorQuoteCmd(m.flowSvc, msg.From, msg.USDAmount, msg.BTCAddress), true
	case swapbtcpanel.ExecuteRequestMsg:
		if m.current == nil {
			return m, nil, true
		}
		applog.Tracef("mainscreen — executing BTC swap %s → BTC $%s for %s",
			msg.From.Symbol, msg.USDAmount, m.current.Address)
		return m, thorExecuteCmd(m.flowSvc, m.current.PrivateKey, msg.From, msg.USDAmount, msg.BTCAddress), true
	case swapbtcpanel.QuoteResultMsg, swapbtcpanel.ExecutionResultMsg, swapbtcpanel.TickMsg, swapbtcpanel.EstimateResultMsg:
		next, cmd := m.onSwapBTCResult(msg)
		return next, cmd, true
	}
	return m, nil, false
}

// onSwapBTCResult forwards parent → panel results + countdown ticks to the
// Swap BTC tab even when it isn't focused, so an in-flight quote/swap finishes
// cleanly.
func (m Model) onSwapBTCResult(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.swapbtc, cmd = m.swapbtc.Update(msg)
	if r, ok := msg.(swapbtcpanel.ExecutionResultMsg); ok && r.Result.Success && m.current != nil {
		delete(m.balanceCache, m.current.Address)
		return m, tea.Batch(cmd, m.balancesCmdForCurrent())
	}
	return m, cmd
}

// handleSettingsMsg handles settings-tab requests and the chain-switch result.
// handled=false for any other message.
func (m Model) handleSettingsMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case settingspanel.ShowWalletMsg:
		if m.current == nil {
			m.errMsg = "no wallet selected"
			return m, nil, true
		}
		m.mode = modeWalletQR
		m.walletCopyStatus = ""
		return m, nil, true
	case settingspanel.SlippageChangedMsg:
		m.slippage = msg.Value
		m.settings.SetSlippage(msg.Value)
		m.persistSlippage(msg.Value)
		return m, nil, true
	case settingspanel.NetworkChangeMsg:
		next, cmd := m.onNetworkChange(msg)
		return next, cmd, true
	case chainSwitchedMsg:
		next, cmd := m.onChainSwitched(msg)
		return next, cmd, true
	}
	return m, nil, false
}

func (m Model) onNetworkChange(msg settingspanel.NetworkChangeMsg) (Model, tea.Cmd) {
	if m.balSvc == nil {
		// No RPC services in this session (e.g. tests) — just record it.
		c := chain.Get(msg.ChainKey)
		m.chainKey = msg.ChainKey
		m.settings.SetNetwork(msg.ChainKey, c.Name)
		m.pass.SetAvailable(c.PassAddress != "", c.NativeSymbol)
		m.persistChain(msg.ChainKey)
		return m, nil
	}
	return m, switchChainCmd(msg.ChainKey)
}

func (m Model) onChainSwitched(msg chainSwitchedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = "network switch: " + msg.err.Error()
		return m, nil
	}
	if m.balSvc != nil {
		m.balSvc.Close()
	}
	if m.flowSvc != nil {
		m.flowSvc.Close()
	}
	if m.passSvc != nil {
		m.passSvc.Close()
	}
	m.chainKey = msg.chainKey
	m.balSvc = msg.balSvc
	m.flowSvc = msg.flowSvc
	m.passSvc = msg.passSvc
	c := chain.Get(msg.chainKey)
	m.settings.SetNetwork(msg.chainKey, c.Name)
	m.pass.SetAvailable(c.PassAddress != "", c.NativeSymbol)
	m.persistChain(msg.chainKey)
	m.balanceCache = make(map[string][]balance.TokenBalance)
	m.errMsg = ""
	return m, tea.Batch(m.balancesCmdForCurrent(), m.refreshPassCmd())
}
