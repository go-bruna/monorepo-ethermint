#!/bin/bash

export BINARY=${BINARY:-"./build/sgx"}
export HOME_DIR=$(eval echo "${HOME_DIR:-"~/.inco"}")
export FHEVM_GO_KEYS_DIR="$HOME_DIR/keys/network-fhe-keys"

# if which binary does not exist, exit
if [ -z `which $BINARY` ]; then
  echo "Ensure $BINARY is installed and in your PATH"
  exit 1
fi

command -v $BINARY > /dev/null 2>&1 || { echo >&2 "$BINARY command not found. Ensure this is setup / properly installed in your GOPATH (make install)."; exit 1; }

$BINARY