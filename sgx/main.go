package main

import (
	"os"

	"cosmossdk.io/log"
	"github.com/Inco-fhevm/inco-monorepo/sgx/evm"
	"github.com/rs/zerolog"
)

const PORT = 9092

func main() {
	logger := log.NewLogger(os.Stdout, log.LevelOption(zerolog.InfoLevel))

	err := evm.RunSgxRpcServer(logger, PORT)
	if err != nil {
		panic(err)
	}
}
