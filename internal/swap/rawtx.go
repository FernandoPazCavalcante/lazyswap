package swap

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
)

// RawTx is an unsigned transaction built by an external router (the lazyswap
// backend / OpenOcean). Numeric fields are decimal strings in base units.
type RawTx struct {
	To       string
	Data     string // 0x-prefixed calldata
	Value    string
	GasPrice string
	GasLimit string
	ChainID  int64
}

// ExecuteRawTx signs and broadcasts a backend-built swap transaction with the
// user's key, via the user's own RPC. When the input is an ERC-20 it first
// ensures the tx target (the aggregator router) has sufficient allowance.
// Refuses a tx whose chainId does not match the connected chain.
func (f *Flow) ExecuteRawTx(
	ctx context.Context,
	privateKeyHex string,
	raw RawTx,
	fromToken TokenInfo,
	amountBase *big.Int,
) (txHash, gasUsed string, err error) {
	if raw.ChainID != 0 && uint64(raw.ChainID) != f.chain.ChainID {
		return "", "", fmt.Errorf("refusing tx for chainId %d on %s (chainId %d)", raw.ChainID, f.chain.Name, f.chain.ChainID)
	}
	pk, err := ethcrypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", "", fmt.Errorf("parse private key: %w", err)
	}
	pub, ok := pk.Public().(*ecdsa.PublicKey)
	if !ok {
		return "", "", errors.New("derive public key: unexpected type")
	}
	from := ethcrypto.PubkeyToAddress(*pub)
	to := common.HexToAddress(raw.To)

	if fromToken.Address != NativeSentinel {
		if err := f.ensureAllowance(ctx, pk, from, fromToken.Address, to, amountBase); err != nil {
			return "", "", err
		}
	}

	p, err := f.parseRawTx(ctx, raw)
	if err != nil {
		return "", "", err
	}
	nonce, err := f.client.PendingNonceAt(ctx, from)
	if err != nil {
		return "", "", fmt.Errorf("nonce: %w", err)
	}

	chainID := new(big.Int).SetUint64(f.chain.ChainID)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    p.value,
		Gas:      p.gasLimit,
		GasPrice: p.gasPrice,
		Data:     p.data,
	})
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), pk)
	if err != nil {
		return "", "", fmt.Errorf("sign: %w", err)
	}
	if err := f.client.SendTransaction(ctx, signed); err != nil {
		return "", "", fmt.Errorf("broadcast: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, f.client, signed)
	if err != nil {
		return "", "", fmt.Errorf("wait: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return receipt.TxHash.Hex(), "", errors.New("transaction reverted")
	}
	return receipt.TxHash.Hex(), strconv.FormatUint(receipt.GasUsed, 10), nil
}

// rawTxParams holds RawTx's numeric and calldata fields decoded into typed
// values ready for a LegacyTx.
type rawTxParams struct {
	value    *big.Int
	gasPrice *big.Int
	gasLimit uint64
	data     []byte
}

// parseRawTx decodes RawTx's string fields, falling back to the RPC's
// suggested gas price when none is set.
func (f *Flow) parseRawTx(ctx context.Context, raw RawTx) (rawTxParams, error) {
	value, ok := new(big.Int).SetString(zeroIfEmpty(raw.Value), 10)
	if !ok {
		return rawTxParams{}, fmt.Errorf("bad tx value %q", raw.Value)
	}
	gasPrice, ok := new(big.Int).SetString(zeroIfEmpty(raw.GasPrice), 10)
	if !ok || gasPrice.Sign() == 0 {
		var err error
		if gasPrice, err = f.client.SuggestGasPrice(ctx); err != nil {
			return rawTxParams{}, fmt.Errorf("gas price: %w", err)
		}
	}
	gasLimit, err := strconv.ParseUint(zeroIfEmpty(raw.GasLimit), 10, 64)
	if err != nil {
		return rawTxParams{}, fmt.Errorf("bad gas limit %q", raw.GasLimit)
	}
	// Aggregator gas estimates run tight; pad 25% — unused gas is refunded.
	gasLimit += gasLimit / 4

	data, err := hex.DecodeString(strings.TrimPrefix(raw.Data, "0x"))
	if err != nil {
		return rawTxParams{}, fmt.Errorf("bad calldata: %w", err)
	}
	return rawTxParams{value: value, gasPrice: gasPrice, gasLimit: gasLimit, data: data}, nil
}

// ensureAllowance approves spender for exactly amount when the current
// allowance is insufficient, blocking until the approval is mined.
func (f *Flow) ensureAllowance(
	ctx context.Context,
	pk *ecdsa.PrivateKey,
	owner common.Address,
	tokenAddr string,
	spender common.Address,
	amount *big.Int,
) error {
	token := common.HexToAddress(tokenAddr)
	contract := bind.NewBoundContract(token, chain.ERC20ABI, f.client, f.client, f.client)

	var out []any
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &out, "allowance", owner, spender); err != nil {
		return fmt.Errorf("allowance: %w", err)
	}
	if len(out) == 1 {
		if cur, ok := out[0].(*big.Int); ok && cur.Cmp(amount) >= 0 {
			return nil
		}
	}

	auth, err := bind.NewKeyedTransactorWithChainID(pk, new(big.Int).SetUint64(f.chain.ChainID))
	if err != nil {
		return fmt.Errorf("transactor: %w", err)
	}
	auth.Context = ctx
	tx, err := contract.Transact(auth, "approve", spender, amount)
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	if _, err := bind.WaitMined(ctx, f.client, tx); err != nil {
		return fmt.Errorf("approve wait: %w", err)
	}
	return nil
}

func zeroIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
}
