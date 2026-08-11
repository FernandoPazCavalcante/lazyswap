// Package mainscreen owns the post-login layout: wallet panel (left) +
// tokens panel (right), plus overlay routing for create / delete / import.
package mainscreen

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/api"
	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	passpkg "github.com/FernandoPazCavalcante/lazyswap/internal/pass"
	"github.com/FernandoPazCavalcante/lazyswap/internal/safety"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/importoverlay"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/swapoverlay"
	passpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/lazyswappass"
	referralpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/referral"
	settingspanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/settings"
	swapbtcpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/swapbtc"
	tokenspanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/tokens"
	walletpanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/wallet"
	walletpkg "github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

type mode int

const (
	modeNormal mode = iota
	modeConfirmCreate
	modeConfirmDelete
	modeImport
	modeSwap
	modeWalletQR
)

type focused int

const (
	focusLeft focused = iota
	focusRight
)

// tab identifies a right-panel tab. Values match the number key that selects
// the tab and the labels in src/tui/panels/right-panel.ts.
type tab int

const (
	tabTokens   tab = 1
	tabReferral tab = 2
	tabSettings tab = 4
	tabSwapBTC  tab = 5
	tabPass     tab = 6
)

// Model owns the main-screen state machine.
type Model struct {
	dao       *walletpkg.DAO
	svc       *walletpkg.Service
	balSvc    *balance.Service
	flowSvc   *swap.Flow
	passSvc   *passpkg.Service
	safetySvc *safety.Service
	apiClient *api.Client

	// swapMode is the persisted route preference ("", "direct", "api"); the
	// swap overlay's 't' key toggles it. lastQuoteMode records which route the
	// latest quote actually used, so execution follows the quoted route.
	swapMode      string
	lastQuoteMode string

	panel    walletpanel.Model
	tokens   tokenspanel.Model
	referral referralpanel.Model
	settings settingspanel.Model
	swapbtc  swapbtcpanel.Model
	pass     passpanel.Model
	imp      importoverlay.Model
	swap     swapoverlay.Model

	mode      mode
	focus     focused
	activeTab tab
	errMsg    string
	// notice is a transient confirmation (e.g. "address copied") shown in the
	// footer in place of an error. walletCopyStatus is the feedback line inside
	// the wallet QR overlay.
	notice           string
	walletCopyStatus string

	// chainKey + slippage are the authoritative swap parameters; the settings
	// tab edits them via messages, the swap flow consumes them.
	chainKey string
	slippage float64

	wallets []walletpkg.Wallet
	current *walletpkg.Wallet

	// defaultWallet is the persisted preferred wallet address (settings); the
	// wallet list selects it on load and updates it when the user switches.
	defaultWallet string

	// Cache balances per wallet address so flipping wallets doesn't re-fetch.
	balanceCache map[string][]balance.TokenBalance

	width, height int
}

// New constructs a main-screen model. balSvc / flowSvc may be nil — when
// either is missing the corresponding feature is disabled but the panel
// remains usable (useful for tests). dao may be nil in tests (persistence is
// then skipped). st seeds the chain, slippage, and default wallet from the
// shared, persisted settings.
func New(svc *walletpkg.Service, balSvc *balance.Service, flowSvc *swap.Flow, passSvc *passpkg.Service, dao *walletpkg.DAO, st settings.Settings) Model {
	chainKey := st.ChainKey
	if chainKey == "" {
		chainKey = chain.DefaultKey
	}
	c := chain.Get(chainKey)
	passPanel := passpanel.New()
	passPanel.SetAvailable(c.PassAddress != "", c.NativeSymbol)
	return Model{
		dao:           dao,
		svc:           svc,
		balSvc:        balSvc,
		flowSvc:       flowSvc,
		passSvc:       passSvc,
		safetySvc:     safety.New(),
		apiClient:     api.New(""),
		swapMode:      st.SwapMode,
		panel:         walletpanel.New(),
		tokens:        tokenspanel.New(),
		referral:      referralpanel.New(),
		settings:      settingspanel.New(st.Slippage, chainKey, c.Name),
		swapbtc:       swapbtcpanel.New(),
		pass:          passPanel,
		imp:           importoverlay.New(),
		focus:         focusLeft,
		activeTab:     tabTokens,
		chainKey:      chainKey,
		slippage:      st.Slippage,
		defaultWallet: st.DefaultWallet,
		balanceCache:  make(map[string][]balance.TokenBalance),
	}
}

// ─── settings persistence ────────────────────────────────────────────────────
// Write changes through to the shared settings store so the CLI reads the same
// values. Best-effort: a failure is logged, never surfaced to the user. A nil
// dao (tests) skips persistence.

func (m Model) persistSlippage(v float64) {
	if m.dao == nil {
		return
	}
	if err := settings.SetSlippage(m.dao, v); err != nil {
		applog.Error("persist slippage", err)
	}
}

func (m Model) persistChain(key string) {
	if m.dao == nil {
		return
	}
	if err := settings.SetChain(m.dao, key); err != nil {
		applog.Error("persist chain", err)
	}
}

func (m Model) persistDefaultWallet(addr string) {
	if m.dao == nil {
		return
	}
	if err := settings.SetDefaultWallet(m.dao, addr); err != nil {
		applog.Error("persist default wallet", err)
	}
}

func (m Model) persistSwapMode(mode string) {
	if m.dao == nil {
		return
	}
	if err := settings.SetSwapMode(m.dao, mode); err != nil {
		applog.Error("persist swap mode", err)
	}
}

// Init kicks off the initial wallet load.
func (m Model) Init() tea.Cmd { return refreshWalletsCmd(m.svc) }

// footerRows is the number of rows reserved below the panels: one for the
// optional error line + one for the status bar. Kept constant so panel sizes
// don't shift when an error appears.
const footerRows = 2

// tabHeaderRows is the single row reserved at the top of the right column for
// the tab bar (mirrors right-panel.ts tabHeader).
const tabHeaderRows = 1

// SetSize lays out the inner components against the outer terminal size.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	leftW := w / 3
	if leftW < 24 {
		leftW = 24
	}
	rightW := w - leftW
	if rightW < 1 {
		rightW = 1
	}
	bodyH := h - footerRows
	if bodyH < 1 {
		bodyH = 1
	}
	rightH := bodyH - tabHeaderRows
	if rightH < 1 {
		rightH = 1
	}
	m.panel.SetSize(leftW, bodyH)
	m.tokens.SetSize(rightW, rightH)
	m.referral.SetSize(rightW, rightH)
	m.settings.SetSize(rightW, rightH)
	m.swapbtc.SetSize(rightW, rightH)
	m.pass.SetSize(rightW, rightH)
	m.imp.SetSize(w, h)
	if m.mode == modeSwap {
		m.swap.SetSize(w, h)
	}
	m.applyFocusStyles()
}

func (m *Model) applyFocusStyles() {
	m.panel.SetFocused(m.focus == focusLeft)
	rightFocused := m.focus == focusRight
	m.tokens.SetFocused(rightFocused && m.activeTab == tabTokens)
	m.referral.SetFocused(rightFocused && m.activeTab == tabReferral)
	m.settings.SetFocused(rightFocused && m.activeTab == tabSettings)
	m.swapbtc.SetFocused(rightFocused && m.activeTab == tabSwapBTC)
	m.pass.SetFocused(rightFocused && m.activeTab == tabPass)
}

// useAPIRoute decides whether a quote should go through the backend: explicit
// "api" preference, or auto ("") once authenticated — never on chains without
// OpenOcean coverage.
func (m Model) useAPIRoute() bool {
	if chain.Get(m.chainKey).OpenOceanKey == "" {
		return false
	}
	return m.swapMode == settings.SwapModeAPI ||
		(m.swapMode == "" && m.apiClient.Authenticated())
}

func (m Model) chainName() string {
	if m.balSvc == nil {
		return chain.Get(chain.DefaultKey).Name
	}
	return m.balSvc.Chain().Name
}

func shortAddr(a string) string {
	if len(a) <= 12 {
		return a
	}
	return fmt.Sprintf("%s…%s", a[:6], a[len(a)-4:])
}
