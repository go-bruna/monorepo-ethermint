# Inco Node

## Run a local node

1. Build the node

```bash
make build
```

The binary will be in `./build/incod`.

2. Run the node

```bash
./scripts/run_local_node.sh
```

You should see the node's logs running in your terminal. If you stop the node, and wish to re-run from the same height (not create a new chain), you can pass in `CLEAN=false ./scripts/run_local_node.sh`.

## Faucet

You can give yourself some INCO tokens using the local faucet:

```bash
./scripts/local_faucet.sh <eth address>
```
