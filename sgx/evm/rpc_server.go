package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"time"

	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	evmtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/evmos/ethermint/x/evm/sgx"
	"github.com/evmos/ethermint/x/evm/statedb"
	"github.com/zama-ai/fhevm-go/fhevm"
)

// SgxRpcServer is an RPC server on the SGX binary that listens for RPC calls
// from the Cosmos node.
type SgxRpcServer struct {
	logger log.Logger

	// evms is a map of unique handler id to EVM instance
	evms map[uint64]*vm.EVM

	// handler id to specify evms per session
	lastHandlerId uint64
	handlerMutex  *sync.Mutex
}

func RunSgxRpcServer(logger log.Logger, port uint) error {
	logger = logger.With("module", "rpcserver")
	rpcSrv := &SgxRpcServer{
		logger:        logger,
		evms:          make(map[uint64]*vm.EVM),
		lastHandlerId: uint64(time.Now().UnixMilli()),
		handlerMutex:  &sync.Mutex{},
	}

	rpc.Register(rpcSrv)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("RPC server listening on port %d", port))
	http.Serve(l, nil)
	// defer l.Close()
	return nil
}

func (s *SgxRpcServer) StartEVM(req *sgx.StartEVMRequest) (*sgx.StartEVMResponse, error) {
	s.logger.Info("<< Serving StartEVM method", "txhash", common.Bytes2Hex(req.EvmConfig.TxConfig.TxHash))

	s.handlerMutex.Lock()
	defer s.handlerMutex.Unlock()

	// Increase handler id to generate a unique one for each EVM instance
	s.lastHandlerId++
	sdkCtx := sdk.NewContext(nil, *req.Header, false, s.logger)
	ethmRpcCl, err := newEthmRpcClient(s.logger, s.lastHandlerId)
	if err != nil {
		return nil, err
	}

	msg := core.Message{}
	err = json.Unmarshal(req.MsgJson, &msg)
	if err != nil {
		return nil, err
	}

	evm, err := newEVM(sdkCtx, msg, req.EvmConfig, ethmRpcCl)
	if err != nil {
		return nil, err
	}

	// Store evms so that each request can be mapped to its own EVM instance
	s.evms[s.lastHandlerId] = evm

	resp := &sgx.StartEVMResponse{
		HandlerId: s.lastHandlerId,
	}

	s.logger.Info("StartEVM done", "handlerid", resp.HandlerId)
	return resp, nil
}

func (s *SgxRpcServer) InitFhevm(_ context.Context, req *sgx.InitFhevmRequest) (*sgx.InitFhevmResponse, error) {
	s.logger.Info("<< Serving InitFhevm method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	// We unfortunately can't call InitFhevm on the EVM instance during the
	// StartEVM method (inside newEVM), because InitFhevm needs access to the
	// stateDB (via a gRPC calls), but the stateDB (i.e. its associated
	// sdk.Context) is not yet initialized at that point.
	//
	// To solve this, we separate the fhEVM initialization in two phases:
	// 1. StartEVM: Initialize the EVM instance and store it in the evms map
	// (doesn't need access to stateDB).
	// 2. InitFhevm: Initialize the fhEVM instance (needs access to stateDB).
	// ref: https://github.com/Inco-fhevm/zbc-go-ethereum/pull/2
	fhevm.InitFhevm(evm.FhevmEnvironment())

	return &sgx.InitFhevmResponse{}, nil
}

func (s *SgxRpcServer) Create(req *sgx.CreateRequest) (*sgx.CreateResponse, error) {
	s.logger.Info("<< Serving Create method", "handlerid", req.HandlerId, "sender", common.Bytes2Hex(req.Caller))

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	caller := vm.AccountRef(common.BytesToAddress(req.Caller))
	value := new(big.Int).SetUint64(req.Value)
	ret, contractAddr, leftoverGas, err := evm.Create(caller, req.Code, req.Gas, value)
	if err != nil {
		s.logger.Error("EVM create failed", "error", err, "handlerid", req.HandlerId)
		return nil, err
	}

	resp := &sgx.CreateResponse{
		Ret:          ret,
		ContractAddr: contractAddr.Bytes(),
		LeftOverGas:  leftoverGas,
	}

	s.logger.Debug("Create done", "reply", resp)
	return resp, nil
}

func (s *SgxRpcServer) Call(req *sgx.CallRequest) (*sgx.CallResponse, error) {
	s.logger.Info("<< Serving Call method", "handlerid", req.HandlerId, "sender", common.Bytes2Hex(req.Caller))

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	caller := vm.AccountRef(common.BytesToAddress(req.Caller))
	addr := common.BytesToAddress(req.Addr)
	value := new(big.Int).SetUint64(req.Value)
	ret, leftoverGas, err := evm.Call(caller, addr, req.Input, req.Gas, value)
	if err != nil {
		s.logger.Error("EVM call failed", "error", err, "handlerid", req.HandlerId)
		return nil, err
	}

	// Set result
	resp := &sgx.CallResponse{
		Ret:         ret,
		LeftOverGas: leftoverGas,
	}

	s.logger.Debug("Call done", "reply", resp)
	return resp, nil
}

func (s *SgxRpcServer) Commit(req *sgx.CommitRequest) (*sgx.CommitResponse, error) {
	s.logger.Info("<< Serving Commit method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	err := evm.StateDB.(*statedb.StateDB).Commit()

	resp := &sgx.CommitResponse{}
	s.logger.Debug("Commit done", "reply", resp)

	return resp, err
}

func (s *SgxRpcServer) StateDBAddBalance(req *sgx.StateDBAddBalanceRequest) (*sgx.StateDBAddBalanceResponse, error) {
	s.logger.Info("<< Serving StateDB Add balance method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	caller := vm.AccountRef(common.BytesToAddress(req.Caller))
	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse amount %s", req.Amount)
	}
	evm.StateDB.AddBalance(caller.Address(), amount)

	resp := &sgx.StateDBAddBalanceResponse{}
	s.logger.Debug("StateDB Add balance done", "reply", resp)

	return resp, nil
}

func (s *SgxRpcServer) StateDBSubBalance(req *sgx.StateDBSubBalanceRequest) (*sgx.StateDBSubBalanceResponse, error) {
	s.logger.Info("<< Serving StateDB sub balance method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	caller := vm.AccountRef(common.BytesToAddress(req.Caller))
	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse amount %s", req.Amount)
	}

	evm.StateDB.SubBalance(caller.Address(), amount)

	resp := &sgx.StateDBSubBalanceResponse{}
	s.logger.Debug("StateDB SubBalance done", "reply", resp)

	return resp, nil
}

func (s *SgxRpcServer) StateDBSetNonce(req *sgx.StateDBSetNonceRequest) (*sgx.StateDBSetNonceResponse, error) {
	s.logger.Info("<< Serving StateDB SetNonce method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	caller := vm.AccountRef(common.BytesToAddress(req.Caller))
	evm.StateDB.SetNonce(caller.Address(), req.Nonce)

	resp := &sgx.StateDBSetNonceResponse{}
	s.logger.Debug("StateDB SetNonce done", "reply", resp)

	return resp, nil
}

func (s *SgxRpcServer) StateDBIncreaseNonce(req *sgx.StateDBIncreaseNonceRequest) (*sgx.StateDBIncreaseNonceResponse, error) {
	s.logger.Info("<< Serving StateDB IncreaseNonce method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	caller := vm.AccountRef(common.BytesToAddress(req.Caller))
	evm.StateDB.SetNonce(caller.Address(), evm.StateDB.GetNonce(caller.Address())+1)
	resp := &sgx.StateDBIncreaseNonceResponse{}
	s.logger.Debug("StateDB IncreaseNonce IncreaseNonce done", "reply", resp)

	return resp, nil
}

func (s *SgxRpcServer) StateDBPrepare(req *sgx.StateDBPrepareRequest) (*sgx.StateDBPrepareResponse, error) {
	s.logger.Info("<< Serving StateDB Prepare method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	sender := common.BytesToAddress(req.Sender)
	dest := common.BytesToAddress(req.Dest)

	rules := params.Rules{}
	err := json.Unmarshal(req.RulesJson, &rules)
	if err != nil {
		return nil, err
	}

	accessList := evmtypes.AccessList{}
	for _, accList := range req.AccessList {
		storageKeys := make([]common.Hash, 0)
		for _, storageKey := range accList.StorageKeys {
			storageKeys = append(storageKeys, common.BytesToHash(storageKey))
		}
		accTuple := evmtypes.AccessTuple{
			Address:     common.BytesToAddress(accList.Address),
			StorageKeys: storageKeys,
		}

		accessList = append(accessList, accTuple)
	}
	evm.StateDB.Prepare(rules, sender, common.Address(req.Coinbase), &dest, vm.ActivePrecompiles(rules), accessList)

	resp := &sgx.StateDBPrepareResponse{}
	s.logger.Debug("StateDB Add Prepare done", "reply", resp)

	return resp, nil
}

func (s *SgxRpcServer) StateDBGetRefund(req *sgx.StateDBGetRefundRequest) (*sgx.StateDBGetRefundResponse, error) {
	s.logger.Info("<< Serving StateDB GetRefund method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	resp := &sgx.StateDBGetRefundResponse{
		Refund: evm.StateDB.GetRefund(),
	}
	s.logger.Debug("StateDB GetRefund done", "reply", resp)

	return resp, nil
}

func (s *SgxRpcServer) StateDBGetLogs(req *sgx.StateDBGetLogsRequest) (*sgx.StateDBGetLogsResponse, error) {
	s.logger.Info("<< Serving StateDB GetLogs method", "handlerid", req.HandlerId)

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	logs := evm.StateDB.(*statedb.StateDB).Logs()

	logsJSON := make([][]byte, len(logs))
	for i, log := range logs {
		logJSON, err := json.Marshal(log)
		if err != nil {
			return nil, err
		}
		logsJSON[i] = logJSON
	}

	resp := &sgx.StateDBGetLogsResponse{
		Logs: logsJSON,
	}
	s.logger.Debug("StateDB GetLogs done", "reply", resp)

	return resp, nil
}

func (s *SgxRpcServer) StopEVM(req *sgx.StopEVMRequest) (*sgx.StopEVMResponse, error) {
	s.logger.Info("<< Serving StopEVM method", "handlerid", req.HandlerId)

	s.handlerMutex.Lock()
	defer s.handlerMutex.Unlock()

	evm := s.evms[req.HandlerId]
	if evm == nil {
		panic("tx context not initialized")
	}

	delete(s.evms, req.HandlerId)

	resp := &sgx.StopEVMResponse{}
	s.logger.Debug("StopEVM done", "reply", resp)

	return resp, nil
}
