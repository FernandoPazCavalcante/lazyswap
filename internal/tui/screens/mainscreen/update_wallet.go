package mainscreen

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/seedoverlay"
	walletpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/wallet"
	walletpkg "github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
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
	case seedoverlay.ConfirmedMsg:
		return m, saveConfirmedCmd(m.svc, msg.Phrase), true
	case seedoverlay.CloseMsg:
		m.mode = modeNormal
		return m, nil, true
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
	return m, m.balancesCmdForCurrent()
}

// openBackupOverlay generates a phrase (not yet saved) and asks the user to
// prove they wrote it down before the wallet exists.
func (m Model) openBackupOverlay() (Model, tea.Cmd) {
	phrase, err := walletpkg.GenerateMnemonic()
	if err == nil {
		var picks [2]int
		if picks, err = seedoverlay.RandomPicks(len(strings.Fields(phrase))); err == nil {
			m.seed = seedoverlay.NewBackup(phrase, picks)
			m.seed.SetSize(m.width, m.height)
			m.mode = modeSeed
			return m, m.seed.Init()
		}
	}
	m.mode = modeNormal
	m.errMsg = "create: " + err.Error()
	return m, nil
}

// openRevealOverlay shows the selected wallet's phrase after re-checking the
// password against the store.
func (m Model) openRevealOverlay() (Model, tea.Cmd) {
	if m.current == nil || m.dao == nil {
		return m, nil
	}
	dao, addr := m.dao, m.current.Address
	m.seed = seedoverlay.NewReveal(addr, func(pw string) (string, error) {
		c, err := walletpkg.Unlock(dao, pw)
		if err != nil {
			return "", err
		}
		return walletpkg.NewService(dao, c).Reveal(addr)
	})
	m.seed.SetSize(m.width, m.height)
	m.mode = modeSeed
	return m, m.seed.Init()
}
