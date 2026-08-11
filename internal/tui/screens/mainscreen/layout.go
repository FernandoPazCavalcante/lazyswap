package mainscreen

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/rightpanel"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/theme"
)

// ─── Tabs ──────────────────────────────────────────────────────────────────────

// availableTabs returns the enabled right-panel tabs in display order. Mirrors
// the FEATURES.tabs gate in src/common/features.ts. Settings (4) and Swap BTC
// (5) are appended as their panels land in later phases.
func (m Model) availableTabs() []rightpanel.Tab {
	return []rightpanel.Tab{
		{Num: int(tabTokens), Label: "Tokens"},
		{Num: int(tabReferral), Label: "Referral"},
		{Num: int(tabSettings), Label: "Settings"},
		{Num: int(tabSwapBTC), Label: "Swap BTC"},
		{Num: int(tabPass), Label: "Lazyswap Pass"},
	}
}

// tabExists reports whether n maps to an enabled tab.
func (m Model) tabExists(n int) bool {
	for _, t := range m.availableTabs() {
		if t.Num == n {
			return true
		}
	}
	return false
}

// capturingInput reports whether the active tab is currently editing a text
// field — when true, number keys are typed into the field rather than treated
// as tab-switch shortcuts.
func (m Model) capturingInput() bool {
	switch m.activeTab {
	case tabSettings:
		return m.settings.Capturing()
	case tabSwapBTC:
		return m.swapbtc.Capturing()
	default:
		return false
	}
}

// rightColumn renders the tab header stacked above the active tab's body.
func (m Model) rightColumn() string {
	header := theme.Text().Render(rightpanel.Header(int(m.activeTab), m.availableTabs()))
	return lipgloss.JoinVertical(lipgloss.Left, header, m.activeTabView())
}

// activeTabView returns the View of whichever tab is currently selected.
func (m Model) activeTabView() string {
	switch m.activeTab {
	case tabReferral:
		return m.referral.View()
	case tabSettings:
		return m.settings.View()
	case tabSwapBTC:
		return m.swapbtc.View()
	case tabPass:
		return m.pass.View()
	default:
		return m.tokens.View()
	}
}

// onTabActivated returns any command to run when a tab becomes active (e.g.
// loading balances for the tokens tab). Later phases hook settings / swapBTC.
func (m *Model) onTabActivated() tea.Cmd {
	switch m.activeTab {
	case tabTokens:
		return m.balancesCmdForCurrent()
	case tabSwapBTC:
		// Seed the source list from cache; fetch if we have nothing yet.
		if m.current != nil {
			if cached, ok := m.balanceCache[m.current.Address]; ok {
				m.swapbtc.SetBalances(cached)
			}
		}
		return m.balancesCmdForCurrent()
	case tabPass:
		return m.refreshPassCmd()
	case tabReferral:
		return m.referralStatsCmd()
	default:
		return nil
	}
}

// ─── View ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.mode == modeImport {
		return m.imp.View()
	}
	if m.mode == modeSwap {
		return m.swap.View()
	}
	if m.mode == modeWalletQR {
		return m.walletQRView()
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, m.panel.View(), m.rightColumn())

	if m.mode == modeConfirmCreate || m.mode == modeConfirmDelete {
		var prompt string
		switch m.mode {
		case modeConfirmCreate:
			prompt = "Create a new wallet? (y/n)"
		case modeConfirmDelete:
			if m.current == nil {
				prompt = ""
			} else {
				prompt = fmt.Sprintf("Delete wallet %s? (y/n)", shortAddr(m.current.Address))
			}
		}
		overlay := theme.Border(true).
			Padding(1, 3).
			Background(theme.YellowSel).
			Render(theme.Text().Bold(true).Render(prompt))
		if m.width > 0 && m.height > 0 {
			return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
		}
		return overlay
	}

	// Always render an error row (empty when no error) so panel heights stay
	// stable as errors come and go. The same row doubles as a transient notice
	// line (e.g. "copied …") when there's no error to show.
	errLine := ""
	if m.errMsg != "" {
		errLine = theme.Error().Render(m.errMsg)
	} else if m.notice != "" {
		errLine = theme.Text().Render(m.notice)
	}
	status := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, body, errLine, status)
}

// walletQRView renders the full-screen, centered overlay showing the active
// wallet's address as scannable QR plus the raw string, with a copy hint.
// The QR is drawn inverse (dark modules on a yellow field) so it keeps the
// conventional dark-on-light orientation that every scanner expects.
func (m Model) walletQRView() string {
	if m.current == nil {
		return ""
	}
	addr := m.current.Address

	var qr string
	if q, err := qrcode.New(addr, qrcode.Medium); err != nil {
		qr = theme.Error().Render("QR error: " + err.Error())
	} else {
		qr = theme.Text().Render(q.ToSmallString(true))
	}

	status := theme.Dim().Render("c: copy   esc: close")
	if m.walletCopyStatus != "" {
		status = theme.Text().Bold(true).Render(m.walletCopyStatus)
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		theme.Text().Bold(true).Render("Wallet Address"),
		"",
		qr,
		theme.Text().Render(addr),
		"",
		status,
	)
	box := theme.Border(true).Padding(1, 3).Render(content)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
}

// renderStatusBar returns the bottom hint line, mirroring TS status-bar.ts.
func (m Model) renderStatusBar() string {
	api := "off"
	if os.Getenv(explorerAPIKeyEnv) != "" {
		api = "on"
	}

	var hint string
	switch m.mode {
	case modeConfirmCreate, modeConfirmDelete:
		hint = "y: confirm  |  n/Esc: cancel"
	case modeImport:
		hint = "Enter: import  |  Esc: cancel"
	case modeSwap:
		hint = "Esc: back"
	case modeWalletQR:
		hint = "c: copy  |  Esc: close"
	default:
		switch m.focus {
		case focusLeft:
			hint = fmt.Sprintf("Tab: focus  |  c/d/i  y:copy  |  j/k  |  ^C  |  API:%s", api)
		case focusRight:
			if m.activeTab == tabPass {
				hint = "Tab: focus  |  enter: buy pass  |  ^C"
			} else {
				hint = fmt.Sprintf("Tab: focus  |  s:swap  r:refresh  y:copy  |  j/k  |  ^C  |  API:%s", api)
			}
		}
	}

	style := theme.Dim()
	if m.width > 0 {
		style = style.Width(m.width)
	}
	return style.Render(" " + hint)
}
