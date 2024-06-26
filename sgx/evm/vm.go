package evm

import (
	"encoding/json"
	"fmt"
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	rpctypes "github.com/evmos/ethermint/rpc/types"
	ethermint "github.com/evmos/ethermint/types"
	"github.com/evmos/ethermint/x/evm/sgx"
	"github.com/evmos/ethermint/x/evm/statedb"
	ethtypes "github.com/evmos/ethermint/x/evm/types"
)

// TRACER_NAME is the tracer name used in the EVM. It matches the defaults flag
// used in Ethermint.
// ref: https://github.com/Inco-fhevm/ethermint/blob/fd6c811a48caf12415f6e87cf3ea89d034117be1/server/flags/flags.go#L79
const TRACER_NAME = "evm.tracer"

// switch block (nil = no fork, 0 = already homestead)
func ChainConfigUint64ToBigInt(val uint64) *big.Int {
	if val == 0 {
		return new(big.Int).SetUint64(val)
	}
	return nil
}

func newEVM(
	ctx sdk.Context,
	msg core.Message,
	cfg *sgx.StartEVMConfig,
	ethmRpcCl *ethmGrpcClient,
) (*vm.EVM, error) {
	baseFee, ok := new(big.Int).SetString(cfg.BaseFee, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse base fee %s", cfg.BaseFee)
	}

	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     ethmRpcCl.GetHashFn,
		Coinbase:    common.BytesToAddress(cfg.CoinBase),
		GasLimit:    ethermint.BlockGasLimit(ctx),
		BlockNumber: big.NewInt(ctx.BlockHeight()),
		Time:        uint64(ctx.BlockHeader().Time.Unix()),
		Difficulty:  big.NewInt(0), // unused. Only required in PoW context
		BaseFee:     baseFee,
		Random:      nil, // not supported
	}

	txConfig := statedb.TxConfig{
		BlockHash: common.BytesToHash(cfg.TxConfig.BlockHash), // hash of current block
		TxHash:    common.BytesToHash(cfg.TxConfig.TxHash),    // hash of current tx
		TxIndex:   uint(cfg.TxConfig.TxIndex),                 // the index of current transaction
		LogIndex:  uint(cfg.TxConfig.LogIndex),                // the index of next log within current block
	}

	txCtx := core.NewEVMTxContext(&msg)
	stateDB := statedb.NewWithParams(ctx, ethmRpcCl, txConfig, cfg.EvmDenom)

	overrides := &rpctypes.StateOverride{}
	if cfg.OverridesJson != nil {
		err := json.Unmarshal(cfg.OverridesJson, overrides)
		if err != nil {
			return nil, errorsmod.Wrap(err, "failed to unmarshal StateOverride")
		}

		if err := overrides.Apply(stateDB); err != nil {
			return nil, errorsmod.Wrap(err, "failed to apply state override")
		}
	}

	chainConfig := &params.ChainConfig{}
	err := json.Unmarshal(cfg.ChainConfigJson, chainConfig)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to unmarshal chain ChainConfig")
	}

	vmCfg := vmConfig(ctx, msg, cfg, chainConfig)
	return vm.NewEVM(blockCtx, txCtx, stateDB, chainConfig, vmCfg), nil
}

// VMConfig creates an EVM configuration from the debug setting and the extra EIPs enabled on the
// module parameters. The config generated uses the default JumpTable from the EVM.
func vmConfig(ctx sdk.Context, msg core.Message, cfg *sgx.StartEVMConfig, chainCfg *params.ChainConfig) vm.Config {
	noBaseFee := true
	if ethtypes.IsLondon(chainCfg, ctx.BlockHeight()) {
		noBaseFee = cfg.NoBaseFee
	}

	t := tracer(ctx, msg, chainCfg)

	extraEips := make([]int, 0)
	for _, extraEip := range extraEips {
		extraEips = append(extraEips, int(extraEip))
	}
	return vm.Config{
		Tracer:    t,
		NoBaseFee: noBaseFee,
		ExtraEips: extraEips,
	}
}

// tracer return a default vm.Tracer based on current keeper state
func tracer(ctx sdk.Context, msg core.Message, ethCfg *params.ChainConfig) vm.EVMLogger {
	return ethtypes.NewTracer(TRACER_NAME, msg, ethCfg, ctx.BlockHeight(), ctx.BlockHeader().Time.Unix())
}
