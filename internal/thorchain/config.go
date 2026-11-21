// Package thorchain implements cross-chain EVM → BTC swaps via THORchain.
//
// Mirrors src/blockchain/thorchain/{thorchain-config,thorchain-provider,
// thorchain-executor}.ts. The provider talks to the public THORnode API for
// quotes and inbound vault addresses; the executor signs and broadcasts the
// EVM transaction that initiates the swap.
package thorchain

// Public THORchain endpoints. Never hardcode these elsewhere — import from here.
const (
	// NodeURL is the THORnode base URL — serves swap quotes
	// (/thorchain/quote/swap) and inbound vault addresses (/thorchain/inbound_addresses).
	// Liquify gateway: the old *.ninerealms.com endpoints were retired 2025-04-20.
	// Quotes are a THORnode endpoint, NOT a Midgard one (Midgard is read-only analytics).
	NodeURL = "https://gateway.liquify.com/chain/thorchain_api"

	// BTCAsset is the canonical THORchain asset string for Bitcoin.
	BTCAsset = "BTC.BTC"

	// MemoSlippageBPS is the requested price tolerance (300 bps = 3%).
	MemoSlippageBPS = 300
	// MemoSlippage expresses the tolerance as a fraction for fallback memos.
	MemoSlippage = float64(MemoSlippageBPS) / 10000

	defaultEVMGasLimit = 300000

	// AffiliateBPS is the THORchain affiliate fee in basis points (0 = none).
	AffiliateBPS = 0
)

// EVMChainToThor maps a LazySwap chain key to its THORchain chain identifier.
// Only chains THORchain supports for inbound swaps appear here.
var EVMChainToThor = map[string]string{
	"ethereum": "ETH",
	"bsc":      "BSC",
}
