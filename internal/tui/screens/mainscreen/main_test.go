package mainscreen

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FernandoPazCavalcante/lazyswap/internal/crypto"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/importoverlay"
	"github.com/FernandoPazCavalcante/lazyswap/internal/tui/overlays/seedoverlay"
	settingspanel "github.com/FernandoPazCavalcante/lazyswap/internal/tui/panels/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

const hardhatPhrase = "test test test test test test test test test test test junk"

func newModel(t *testing.T) Model {
	t.Helper()
	dao, err := wallet.OpenAt(filepath.Join(t.TempDir(), "wallets.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = dao.Close() })

	key, _, _ := crypto.DeriveKey("hunter2", make([]byte, crypto.SaltLength))
	c, _ := crypto.New(key)
	svc := wallet.NewService(dao, c)

	m := New(svc, nil, nil, dao, settings.Defaults()) // nil balance/swap/pass services — RPC not exercised in unit tests
	m.SetSize(120, 30)
	return m
}

func TestCKeyEntersConfirmCreate(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modeConfirmCreate {
		t.Fatalf("mode = %v, want modeConfirmCreate", m.mode)
	}
}

func TestNKeyDuringConfirmCreateCancels(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.mode != modeNormal {
		t.Fatalf("after 'n' mode = %v, want modeNormal", m.mode)
	}
}

// 'y' must not save anything: the wallet only exists once the backup quiz is
// passed, so a user can never fund a wallet whose phrase they never saw.
func TestYKeyDuringConfirmCreateOpensBackupWithoutSaving(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.mode != modeSeed || m.seed.Step() != "show" {
		t.Fatalf("mode = %v step = %q, want backup overlay on the phrase", m.mode, m.seed.Step())
	}
	if ws, _ := m.dao.FetchAll(); len(ws) != 0 {
		t.Fatalf("wallet saved before backup was confirmed: %d rows", len(ws))
	}
}

func TestBackupConfirmedSavesWallet(t *testing.T) {
	m := newModel(t)
	m.mode = modeSeed
	_, cmd := m.Update(seedoverlay.ConfirmedMsg{Phrase: hardhatPhrase})
	if cmd == nil {
		t.Fatal("expected a save command")
	}
	cm, ok := cmd().(createdMsg)
	if !ok || cm.err != nil || cm.w == nil {
		t.Fatalf("save failed: %#v", cm)
	}
	if ws, _ := m.dao.FetchAll(); len(ws) != 1 {
		t.Fatalf("rows = %d, want 1", len(ws))
	}
}

func TestBackupAbandonedSavesNothing(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m, _ = m.Update(cmd())
	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want normal", m.mode)
	}
	if ws, _ := m.dao.FetchAll(); len(ws) != 0 {
		t.Fatalf("abandoned backup saved %d wallets", len(ws))
	}
}

func TestBKeyOpensRevealForCurrentWallet(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if m.mode != modeNormal {
		t.Fatalf("b with no wallet should stay normal; got %v", m.mode)
	}
	w, err := m.svc.Import(hardhatPhrase)
	if err != nil {
		t.Fatal(err)
	}
	m.current = w
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if m.mode != modeSeed || m.seed.Step() != "password" {
		t.Fatalf("mode = %v step = %q, want reveal overlay asking for the password", m.mode, m.seed.Step())
	}
}

func TestDKeyWithoutSelectionStaysNormal(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.mode != modeNormal {
		t.Fatalf("d with no current should stay normal; got %v", m.mode)
	}
}

func TestIKeyEntersImportMode(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if m.mode != modeImport {
		t.Fatalf("after 'i' mode = %v, want modeImport", m.mode)
	}
}

func TestImportSubmitTriggersImportCmd(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	_, cmd := m.Update(importoverlay.SubmitMsg{Phrase: hardhatPhrase})
	if cmd == nil {
		t.Fatalf("expected import cmd")
	}
	msg := cmd()
	im, ok := msg.(importedMsg)
	if !ok {
		t.Fatalf("expected importedMsg, got %T", msg)
	}
	if im.err != nil {
		t.Fatalf("import failed: %v", im.err)
	}
}

func TestImportEmptyShowsErrorAndStaysInOverlay(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	m, cmd := m.Update(importoverlay.SubmitMsg{Phrase: ""})
	if cmd != nil {
		t.Fatalf("empty submit should not issue a cmd")
	}
	if m.mode != modeImport {
		t.Fatalf("empty submit should keep modeImport, got %v", m.mode)
	}
}

func TestImportCancelExitsOverlay(t *testing.T) {
	m := newModel(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	m, _ = m.Update(importoverlay.CancelMsg{})
	if m.mode != modeNormal {
		t.Fatalf("after cancel mode = %v, want modeNormal", m.mode)
	}
}

func TestRefreshedMsgPopulatesWallets(t *testing.T) {
	m := newModel(t)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	// Run a create directly via the service so we have data, then feed a
	// refreshedMsg into the model and assert it absorbs it.
	w, err := m.svc.Create()
	if err != nil {
		t.Fatalf("svc.Create: %v", err)
	}

	m, _ = m.Update(walletsRefreshedMsg{wallets: []wallet.Wallet{*w}})
	if len(m.wallets) != 1 {
		t.Fatalf("expected 1 wallet in state, got %d", len(m.wallets))
	}
	if m.current == nil || m.current.Address != w.Address {
		t.Fatalf("current wallet not set to %s", w.Address)
	}
}

// modelWithWallet returns a model with one persisted, selected wallet.
func modelWithWallet(t *testing.T) Model {
	t.Helper()
	m := newModel(t)
	w, err := m.svc.Create()
	if err != nil {
		t.Fatalf("svc.Create: %v", err)
	}
	m, _ = m.Update(walletsRefreshedMsg{wallets: []wallet.Wallet{*w}})
	return m
}

func TestShowWalletMsgEntersQRMode(t *testing.T) {
	m := modelWithWallet(t)
	m, _ = m.Update(settingspanel.ShowWalletMsg{})
	if m.mode != modeWalletQR {
		t.Fatalf("mode = %v, want modeWalletQR", m.mode)
	}
}

func TestShowWalletMsgNoSelectionStaysNormal(t *testing.T) {
	m := newModel(t) // no wallets
	m, _ = m.Update(settingspanel.ShowWalletMsg{})
	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want modeNormal", m.mode)
	}
	if m.errMsg == "" {
		t.Fatal("expected an error message when no wallet is selected")
	}
}

func TestWalletQRModeEscCloses(t *testing.T) {
	m := modelWithWallet(t)
	m, _ = m.Update(settingspanel.ShowWalletMsg{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Fatalf("after esc mode = %v, want modeNormal", m.mode)
	}
}
