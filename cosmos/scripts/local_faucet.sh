#!/bin/bash
# Run this script to quickly install, setup, and run the current version of the network without docker.
#
# Examples:
# CHAIN_ID="local-1" HOME_DIR="~/.inco" BLOCK_TIME="1000ms" CLEAN=true sh scripts/test_node.sh
# CHAIN_ID="local-2" HOME_DIR="~/.inco" CLEAN=true RPC=36657 REST=2317 PROFF=6061 P2P=36656 GRPC=8090 GRPC_WEB=8091 ROSETTA=8081 BLOCK_TIME="500ms" sh scripts/test_node.sh

set -e

export KEY="user1"
export KEY2="user2"

export CHAIN_ID=${CHAIN_ID:-"incolocal_9091-1"}
export MONIKER="localvalidator"
export KEYALGO="secp256k1"
export KEYRING=${KEYRING:-"test"}
export HOME_DIR=$(eval echo "${HOME_DIR:-"~/.inco"}")
export BINARY=${BINARY:-"./build/incod"}
export DENOM=${DENOM:-ainco}

export FHEVM_GO_KEYS_DIR="$HOME_DIR/keys/network-fhe-keys"

# if which binary does not exist, exit
if [ -z `which $BINARY` ]; then
  echo "Ensure $BINARY is installed and in your PATH"
  exit 1
fi

alias BINARY="$BINARY --home=$HOME_DIR"

command -v $BINARY > /dev/null 2>&1 || { echo >&2 "$BINARY command not found. Ensure this is setup / properly installed in your GOPATH (make install)."; exit 1; }

set_config() {
  $BINARY config set client chain-id $CHAIN_ID
  $BINARY config set client keyring-backend $KEYRING
}
set_config

# Get first argument, which is the destination address
RECIPIENT=$1
if [ -z "$RECIPIENT" ]; then
  echo "Please provide a recipient address"
  echo "./local_faucet.sh <recipient>"
  exit 1
fi
RECIPIENT=$(echo $RECIPIENT | sed 's/^0x//')
AMOUNT="1000000000000000000$DENOM" # 10**18, so 1INCO

# Get bech32 addr from Ethereum address
# The `$BINARY debug addr` outputs 4 lines, the 3rd one is:
# Bech32 Acc: inco1n7g8ek2znyua9dqua554pjvkh8vysxejlsfmcp
# We extract the inco1... part
echo "BECH32_ADDR $($BINARY debug addr $RECIPIENT)"
BECH32_ADDR=$($BINARY debug addr $RECIPIENT | sed -n '5 p' | sed 's/Bech32 Acc: //')
echo "Sending $AMOUNT to $BECH32_ADDR"

$BINARY tx bank send $KEY $BECH32_ADDR $AMOUNT --gas-prices 1000000000$DENOM --yes