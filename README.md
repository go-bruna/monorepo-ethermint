<h1 align="center">Inco Monorepo</h1>

<p align="center">
    <strong>Monorepo for Inco node, SGX enclave and FHE oracle.</strong>
</p>

## Get Started

Each node currently needs to run two binaries:

1. The SGX binary exposing a RPC server.
2. The Cosmos SDK node.

For development, we will build the first binary and run it as-is. For production usage, we will run it inside [Gramine](https://gramine.readthedocs.io/en/stable/) on a SGX-enabled machine.

Running a node locally currently requires building several different components. From a common parent folder, run the following commands:

### 1. Compile fhevm-tfhe-cli

```bash
git clone https://github.com/zama-ai/fhevm-tfhe-cli
cd fhevm-tfhe-cli
cargo build --features tfhe/x86_64-unix --release
sudo cp ./target/release/fhevm-tfhe-cli /usr/local/bin/
```

### 2. Build fhevm-go

```bash
git clone --recurse-submodules  https://github.com/Inco-fhevm/fhevm-go
cd fhevm-go
make build
```

### 3. Clone zbc-go-ethereum

```bash
git clone https://github.com/Inco-fhevm/zbc-go-ethereum
```

### 4. Run local node

In a first terminal, run the following commands:

```bash
git clone https://github.com/Inco-fhevm/inco-monorepo
cd inco-monorepo/cosmos
make build
./scripts/run_local_node.sh

```

In a second terminal, run the following commands:

```bash
cd inco-monorepo/sgx
make build
./scripts/run.sh
```

### 5. Fund a test account

In a third terminal, run the following command to fund your Ethereum account:

```bash
cd inco-monorepo/cosmos
./scripts/local_faucet.sh <ETH addr>
```

If you use Remix for test, replace `TFHE.sol` and `Impl.sol` using [Zama's latest version](https://github.com/zama-ai/fhevm/tree/main/lib).

## Directory Structure

<pre>
├── <a href="./cosmos">cosmos</a>: The Inco node built on Cosmos SDK.
└── <a href="./sgx/">sgx</a>: Binary that will run inside the SGX enclave using Gramine.
</pre>
