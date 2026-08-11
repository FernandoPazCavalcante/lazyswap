// Package testrpc is a minimal fake Ethereum JSON-RPC endpoint for unit tests.
// It understands just enough of the surface lazyswap uses — getAmountsOut,
// balanceOf, allowance eth_calls plus eth_getBalance — to exercise the dex,
// swap and balance packages without a network.
//
// Test-support only: excluded from the coverage denominator (its coverage is
// its use by other packages' tests). See scripts/coverage-gate.sh.
package testrpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// Selectors of the eth_call methods the app issues.
const (
	selGetAmountsOut = "d06ca61f" // getAmountsOut(uint256,address[])
	selBalanceOf     = "70a08231" // balanceOf(address)
	selAllowance     = "dd62ed3e" // allowance(address,address)
)

// Server fakes an EVM RPC. Configure the exported fields, then point
// ethclient.Dial at URL.
type Server struct {
	*httptest.Server

	mu sync.Mutex
	// RateNum/RateDen scale getAmountsOut: amountOut = amountIn * num / den.
	RateNum, RateDen *big.Int
	// NoLiquidity makes every getAmountsOut call revert.
	NoLiquidity bool
	// NativeBalance answers eth_getBalance for any address.
	NativeBalance *big.Int
	// TokenBalance answers balanceOf for any (token, holder).
	TokenBalance *big.Int
	// Allowance answers allowance for any (owner, spender).
	Allowance *big.Int
	// Calls counts eth_call invocations by selector.
	Calls map[string]int
	// TxCount counts broadcast transactions; RevertTx makes receipts report
	// failure instead of success.
	TxCount    int
	RevertTx   bool
	lastTxHash string
}

// New starts a fake RPC with 1:1 pricing and zero balances. Close it via
// t.Cleanup(s.Close).
func New() *Server {
	s := &Server{
		RateNum: big.NewInt(1), RateDen: big.NewInt(1),
		NativeBalance: big.NewInt(0),
		TokenBalance:  big.NewInt(0),
		Allowance:     big.NewInt(0),
		Calls:         map[string]int{},
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

type rpcRequest struct {
	ID     json.RawMessage   `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, rpcErr := s.dispatch(req)
	resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	if rpcErr != "" {
		resp["error"] = map[string]any{"code": -32000, "message": rpcErr}
	} else {
		resp["result"] = result
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) dispatch(req rpcRequest) (any, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch req.Method {
	case "eth_chainId":
		return "0x38", ""
	case "eth_getBalance":
		return "0x" + s.NativeBalance.Text(16), ""
	case "eth_call":
		return s.handleCall(req)
	case "eth_getTransactionCount":
		return "0x1", ""
	case "eth_getCode":
		return "0x60806040", "" // non-empty: "a contract lives here"
	case "eth_gasPrice", "eth_maxPriorityFeePerGas":
		return "0x3b9aca00", "" // 1 gwei
	case "eth_estimateGas":
		return "0x33450", ""
	case "eth_getBlockByNumber":
		// Minimal header for fee estimation paths.
		return map[string]any{
			"number": "0x1", "hash": "0x" + strings.Repeat("11", 32),
			"parentHash":    "0x" + strings.Repeat("22", 32),
			"baseFeePerGas": "0x3b9aca00", "gasLimit": "0x1c9c380", "gasUsed": "0x0",
			"difficulty": "0x0", "extraData": "0x", "logsBloom": "0x" + strings.Repeat("00", 256),
			"miner": "0x" + strings.Repeat("00", 20), "mixHash": "0x" + strings.Repeat("00", 32),
			"nonce": "0x0000000000000000", "receiptsRoot": "0x" + strings.Repeat("00", 32),
			"sha3Uncles": "0x" + strings.Repeat("00", 32), "size": "0x0",
			"stateRoot": "0x" + strings.Repeat("00", 32), "timestamp": "0x0",
			"transactionsRoot": "0x" + strings.Repeat("00", 32),
			"transactions":     []any{}, "uncles": []any{},
		}, ""
	case "eth_sendRawTransaction":
		s.TxCount++
		s.lastTxHash = fmt.Sprintf("0x%064x", s.TxCount)
		return s.lastTxHash, ""
	case "eth_getTransactionReceipt":
		status := "0x1"
		if s.RevertTx {
			status = "0x0"
		}
		return map[string]any{
			"transactionHash": s.lastTxHash, "status": status,
			"blockNumber": "0x1", "blockHash": "0x" + strings.Repeat("11", 32),
			"cumulativeGasUsed": "0x5208", "gasUsed": "0x5208",
			"transactionIndex": "0x0", "logs": []any{},
			"logsBloom": "0x" + strings.Repeat("00", 256), "type": "0x0",
			"contractAddress": nil, "effectiveGasPrice": "0x3b9aca00",
		}, ""
	default:
		return nil, fmt.Sprintf("method %s not faked", req.Method)
	}
}

func (s *Server) handleCall(req rpcRequest) (any, string) {
	var call struct {
		Input string `json:"input"` // go-ethereum ≥1.11 sends "input"
		Data  string `json:"data"`
		To    string `json:"to"`
	}
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params[0], &call)
	}
	raw := call.Input
	if raw == "" {
		raw = call.Data
	}
	data := strings.TrimPrefix(raw, "0x")
	if len(data) < 8 {
		return nil, "missing calldata"
	}
	sel := data[:8]
	s.Calls[sel]++

	switch sel {
	case selGetAmountsOut:
		if s.NoLiquidity {
			return nil, "execution reverted"
		}
		amountIn, ok := new(big.Int).SetString(data[8:8+64], 16)
		if !ok {
			return nil, "bad amountIn"
		}
		out := new(big.Int).Mul(amountIn, s.RateNum)
		out.Div(out, s.RateDen)
		// ABI: offset ‖ length=2 ‖ [amountIn, amountOut]
		return "0x" + word(32) + word(2) + wordBig(amountIn) + wordBig(out), ""
	case selBalanceOf:
		return "0x" + wordBig(s.TokenBalance), ""
	case selAllowance:
		return "0x" + wordBig(s.Allowance), ""
	default:
		return nil, "selector " + sel + " not faked"
	}
}

func word(v int64) string { return wordBig(big.NewInt(v)) }

func wordBig(v *big.Int) string {
	b := v.Bytes()
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return hex.EncodeToString(padded)
}
