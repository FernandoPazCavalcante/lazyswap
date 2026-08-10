package mainscreen

import (
	tea "github.com/charmbracelet/bubbletea"

	walletpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/wallet"
)

// handleWalletMsg handles wallet CRUD results, balance fetches, and wallet
// selection changes. handled=false for any other message.
func (m Model) handleWalletMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case walletsRefreshedMsg:
		next, cmd := m.onWalletsRefreshed(msg)
		return next, cmd, true
	case createdMsg:
		next, cmd := m.onCreated(msg)
		return next, cmd, true
	case importedMsg:
		next, cmd := m.onImported(msg)
		return next, cmd, true
	case deletedMsg:
		next, cmd := m.onDeleted(msg)
		return next, cmd, true
	case balancesFetchedMsg:
		next, cmd := m.onBalancesFetched(msg)
		return next, cmd, true
	case walletpanel.SelectionChangedMsg:
		next, cmd := m.onWalletSelected(msg)
		return next, cmd, true
	}
	return m, nil, false
}

func (m Model) onWalletsRefreshed(msg walletsRefreshedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	m.wallets = msg.wallets
	m.panel.SetWallets(msg.wallets)
	if m.defaultWallet != "" {
		m.panel.SelectByAddress(m.defaultWallet)
	}
	if sel := m.panel.Selected(); sel != nil {
		m.current = sel
	} else if len(msg.wallets) > 0 {
		m.current = &msg.wallets[0]
	} else {
		m.current = nil
	}
	return m, m.balancesCmdForCurrent()
}

func (m Model) onCreated(msg createdMsg) (Model, tea.Cmd) {
	m.mode = modeNormal
	if msg.err != nil {
		m.errMsg = "create: " + msg.err.Error()
		return m, nil
	}
	return m, refreshWalletsCmd(m.svc)
}

func (m Model) onImported(msg importedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.imp.SetErr("import: " + msg.err.Error())
		return m, nil
	}
	m.mode = modeNormal
	return m, refreshWalletsCmd(m.svc)
}

func (m Model) onDeleted(msg deletedMsg) (Model, tea.Cmd) {
	m.mode = modeNormal
	if msg.err != nil {
		m.errMsg = "delete: " + msg.err.Error()
		return m, nil
	}
	return m, refreshWalletsCmd(m.svc)
}

func (m Model) onBalancesFetched(msg balancesFetchedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.tokens.SetError(msg.err.Error())
		return m, nil
	}
	m.balanceCache[msg.walletAddress] = msg.balances
	// Only paint the result if the cached fetch still matches the
	// selected wallet (the user may have moved on while we awaited RPC).
	if m.current != nil && m.current.Address == msg.walletAddress {
		m.tokens.SetBalances(msg.balances)
		m.swapbtc.SetBalances(msg.balances)
	}
	return m, nil
}

func (m Model) onWalletSelected(msg walletpanel.SelectionChangedMsg) (Model, tea.Cmd) {
	m.current = msg.Wallet
	if msg.Wallet != nil {
		m.persistDefaultWallet(msg.Wallet.Address)
	}
	return m, tea.Batch(m.balancesCmdForCurrent(), m.refreshPassCmd())
}
