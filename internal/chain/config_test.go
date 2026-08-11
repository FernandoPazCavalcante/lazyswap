package chain

import (
	"regexp"
	"testing"
)

var hexAddr = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func TestRegistryIntegrity(t *testing.T) {
	if len(OrderedKeys) != len(CHAINS) {
		t.Fatalf("OrderedKeys (%d) and CHAINS (%d) diverge", len(OrderedKeys), len(CHAINS))
	}
	seenIDs := map[uint64]string{}
	for _, key := range OrderedKeys {
		c, ok := CHAINS[key]
		if !ok {
			t.Fatalf("OrderedKeys entry %q missing from CHAINS", key)
		}
		if c.ChainID == 0 {
			t.Errorf("%s: zero ChainID", key)
		}
		if prev, dup := seenIDs[c.ChainID]; dup {
			t.Errorf("%s: ChainID %d duplicates %s", key, c.ChainID, prev)
		}
		seenIDs[c.ChainID] = key

		for name, addr := range map[string]string{
			"RouterAddress":  c.RouterAddress,
			"WrappedNative":  c.WrappedNative,
			"StablecoinAddr": c.StablecoinAddr,
		} {
			if !hexAddr.MatchString(addr) {
				t.Errorf("%s: %s %q is not a hex address", key, name, addr)
			}
		}
		if c.RPCURL == "" || c.Name == "" || c.NativeSymbol == "" || c.NativeDecimals == 0 {
			t.Errorf("%s: incomplete base config: %+v", key, c)
		}
		for sym, tok := range c.Tokens {
			if tok.Symbol != sym {
				t.Errorf("%s: token map key %q != symbol %q", key, sym, tok.Symbol)
			}
			if !hexAddr.MatchString(tok.Address) {
				t.Errorf("%s/%s: bad token address %q", key, sym, tok.Address)
			}
			if tok.Decimals == 0 {
				t.Errorf("%s/%s: zero decimals", key, sym)
			}
		}
	}
}

func TestDefaultAndLookups(t *testing.T) {
	if !Has(DefaultKey) {
		t.Fatalf("DefaultKey %q not in CHAINS", DefaultKey)
	}
	if Has("nopechain") {
		t.Fatal("Has must reject unknown keys")
	}
	if got := Get("bsc").ChainID; got != 56 {
		t.Fatalf("bsc ChainID = %d", got)
	}
}

func TestNextKeyWraps(t *testing.T) {
	// Walking NextKey len(OrderedKeys) times from the first key returns to it.
	key := OrderedKeys[0]
	for range OrderedKeys {
		key = NextKey(key)
	}
	if key != OrderedKeys[0] {
		t.Fatalf("NextKey cycle broken: ended at %q", key)
	}
	if NextKey("unknown") != OrderedKeys[0] {
		t.Fatal("NextKey from unknown key must restart at the first key")
	}
}

func TestOpenOceanKeys(t *testing.T) {
	// API swap mode is mainnet-only by construction.
	want := map[string]string{"ethereum": "eth", "bsc": "bsc", "bsc_testnet": "", "sepolia": ""}
	for key, oo := range want {
		if got := Get(key).OpenOceanKey; got != oo {
			t.Errorf("%s: OpenOceanKey = %q, want %q", key, got, oo)
		}
	}
}
