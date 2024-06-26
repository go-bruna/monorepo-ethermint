package evm

import (
	"fmt"
	"math/big"

	"context"
	"net"

	"cosmossdk.io/errors"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/ethermint/x/evm/statedb"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"google.golang.org/grpc"
)

type ethmGrpcClient struct {
	logger    log.Logger
	querier   evmtypes.QueryClient
	handlerId uint64
}

var _ statedb.Keeper = (*ethmGrpcClient)(nil)

func newEthmGrpcClient(logger log.Logger, handlerId uint64) (*ethmGrpcClient, error) {
	// Create a new InterfaceRegistry
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	evmtypes.RegisterInterfaces(interfaceRegistry)

	// Set the node
	rpcConn, err := grpc.Dial("localhost:9090",
		grpc.WithInsecure(),
		grpc.WithContextDialer(func(ctx context.Context, url string) (net.Conn, error) {
			return net.Dial("tcp", url)
		}),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to inco chain grpc")
	}
	querier := evmtypes.NewQueryClient(rpcConn)

	return &ethmGrpcClient{
		querier:   querier,
		logger:    logger.With("module", "grpcclient", "handlerid", handlerId),
		handlerId: handlerId,
	}, nil
}

func (c ethmGrpcClient) GetHashFn(uint64) common.Hash {
	panic("implement me")
}

func (c ethmGrpcClient) StoreKeys() map[string]storetypes.StoreKey {
	return map[string]storetypes.StoreKey{}
}

// GetParams is here to satisfy the interface, but should never be called by
// the SGX binary to the Cosmos node.
func (c ethmGrpcClient) GetParams(sdk.Context) evmtypes.Params {
	panic("dead code")
}

func (c ethmGrpcClient) AddBalance(ctx sdk.Context, addr sdk.AccAddress, coins sdk.Coins) error {
	req := &evmtypes.AddBalanceRequest{HandlerId: c.handlerId, Address: addr.Bytes(), Amount: coins}

	_, err := c.querier.StateDBAddBalance(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed to add balance")
	}

	c.logger.Info(">> Called grpc AddBalance")

	return nil
}

func (c ethmGrpcClient) SubBalance(ctx sdk.Context, addr sdk.AccAddress, coins sdk.Coins) error {
	req := &evmtypes.SubBalanceRequest{HandlerId: c.handlerId, Address: addr.Bytes(), Amount: coins}

	_, err := c.querier.StateDBSubBalance(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed to sub balance")
	}

	c.logger.Info(">> Called grpc SubBalance")

	return nil
}

func (c ethmGrpcClient) SetBalance(_ sdk.Context, addr common.Address, amount *big.Int, denom string) error {
	req := &evmtypes.SetBalanceRequest{
		HandlerId: c.handlerId,
		Address:   addr.Bytes(),
		Amount:    amount.String(),
		Denom:     denom,
	}

	_, err := c.querier.StateDBSetBalance(context.Background(), req)
	if err != nil {
		return errors.Wrap(err, "failed to set balance")
	}

	c.logger.Info(">> Called grpc SetBalance")

	return nil
}

func (c ethmGrpcClient) GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) *big.Int {
	req := &evmtypes.GetBalanceRequest{HandlerId: c.handlerId, Address: addr.Bytes(), Denom: denom}

	resp, err := c.querier.StateDBGetBalance(ctx, req)
	if err != nil {
		panic(err)
	}

	c.logger.Info(">> Called grpc GetBalance")
	bal, ok := new(big.Int).SetString(resp.Balance, 10)
	if !ok {
		panic(fmt.Sprintf("failed to parse balance %s", resp.Balance))
	}

	return bal
}

func (c ethmGrpcClient) GetAccount(ctx sdk.Context, addr common.Address) *statedb.Account {
	req := &evmtypes.GetAccountRequest{HandlerId: c.handlerId, Address: addr.Bytes()}

	resp, err := c.querier.StateDBGetAccount(ctx, req)
	if err != nil {
		panic(err)
	}

	c.logger.Info(">> Called grpc GetAccount", "acc", addr.Hex())

	if resp.Account == nil {
		return nil
	}
	return &statedb.Account{
		CodeHash: resp.Account.CodeHash,
		Nonce:    resp.Account.Nonce,
	}
}

func (c ethmGrpcClient) GetState(ctx sdk.Context, addr common.Address, key common.Hash) common.Hash {
	req := &evmtypes.GetStateRequest{HandlerId: c.handlerId, Address: addr.Bytes(), Key: key.Bytes()}

	resp, err := c.querier.StateDBGetState(ctx, req)
	if err != nil {
		panic(err)
	}

	c.logger.Info(">> Called grpc GetState")

	return common.BytesToHash(resp.Hash)
}

func (c ethmGrpcClient) GetCode(ctx sdk.Context, codeHash common.Hash) []byte {
	req := &evmtypes.GetCodeRequest{HandlerId: c.handlerId, CodeHash: codeHash.Bytes()}

	resp, err := c.querier.StateDBGetCode(ctx, req)
	if err != nil {
		return nil
	}

	c.logger.Info(">> Called grpc GetCode")

	return resp.Code
}

// ForEachStorage is here to satisfy the interface, but should never be called by
// the SGX binary to the Cosmos node.
func (c ethmGrpcClient) ForEachStorage(_ sdk.Context, addr common.Address, cb func(key, value common.Hash) bool) {
	panic("dead code")
}

func (c ethmGrpcClient) SetAccount(ctx sdk.Context, addr common.Address, account statedb.Account) error {
	req := &evmtypes.SetAccountRequest{
		HandlerId: c.handlerId,
		Address:   addr.Bytes(),
		Account:   &evmtypes.Account{Nonce: account.Nonce, CodeHash: account.CodeHash},
	}
	_, err := c.querier.StateDBSetAccount(ctx, req)
	if err != nil {
		return err
	}

	c.logger.Info(">> Called grpc SetAccount")

	return nil
}

func (c ethmGrpcClient) SetState(ctx sdk.Context, addr common.Address, key common.Hash, value []byte) {
	req := &evmtypes.SetStateRequest{HandlerId: c.handlerId, Address: addr.Bytes(), Key: key.Bytes(), Value: value}
	_, err := c.querier.StateDBSetState(ctx, req)
	if err != nil {
		panic(err)
	}

	c.logger.Info(">> Called grpc SetState")
}

func (c ethmGrpcClient) SetCode(ctx sdk.Context, codeHash []byte, code []byte) {
	req := &evmtypes.SetCodeRequest{HandlerId: c.handlerId, CodeHash: codeHash, Code: code}
	c.querier.StateDBSetCode(ctx, req)

	c.logger.Info(">> Called grpc SetCode")
}

func (c ethmGrpcClient) DeleteAccount(ctx sdk.Context, addr common.Address) error {
	req := &evmtypes.DeleteAccountRequest{Address: addr.Bytes()}
	_, err := c.querier.StateDBDeleteAccount(ctx, req)
	if err != nil {
		return err
	}

	c.logger.Info(">> Called grpc DeleteAccount")

	return err
}
