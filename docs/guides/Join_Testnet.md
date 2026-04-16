# FunAI Testnet — External Node Joining Guide

> Seed node: 34.87.21.99
>
> Chain ID: funai_333-1
>
> TGI inference backend: 34.143.145.204:8080 (Qwen2.5-0.5B-Instruct, TGI 3.3.6)

---

## Network Architecture

```
                          ┌─────────────────────┐
                          │  TGI Inference Server │
                          │  34.143.145.204:8080 │
                          └──────────┬──────────┘
                                     │ HTTP
         ┌───────────────────────────┼───────────────────────────┐
         │                           │                           │
  ┌──────┴──────┐            ┌──────┴──────┐            ┌──────┴──────┐
  │ Seed Node    │            │ New Node A  │            │ New Node B  │
  │ 34.87.21.99 │◄──────────►│ your-ip     │◄──────────►│ your-ip-2   │
  │ Chain + P2P │  CometBFT  │ Chain + P2P │  libp2p    │ Chain + P2P │
  │ 4 Workers   │  + libp2p  │ 1 Worker    │            │ 1 Worker    │
  └─────────────┘            └─────────────┘            └─────────────┘
```

Each node runs two processes:
- **funaid** — Cosmos chain node (block production, state sync)
- **funai-node** — P2P inference node (Leader/Worker/Verifier/Proposer)

---

## Prerequisites

- Go 1.25+
- Network access to seed node 34.87.21.99 on ports 46656 (CometBFT P2P), 46657 (RPC), 21317 (REST API), 5001 (libp2p)
- If running inference, network access to TGI server 34.143.145.204:8080

---

## Step 1: Build

```bash
git clone https://github.com/funai-wiki/funai-chain.git
cd funai-chain
make build-all
# Output: ./build/funaid  ./build/funai-node  ./build/e2e-client
```

---

## Step 2: Initialize the Chain Node

```bash
CHAIN_ID="funai_333-1"
NODE_HOME="$HOME/.funai-testnet"

# Initialize the node
./build/funaid init my-node --chain-id $CHAIN_ID --home $NODE_HOME --default-denom ufai
```

---

## Step 3: Obtain the Genesis File

Download the genesis file from the seed node:

```bash
curl -s http://34.87.21.99:21317/cosmos/base/tendermint/v1beta1/node_info > /dev/null  # Verify connectivity

# Fetch genesis via RPC
curl -s http://34.87.21.99:46657/genesis | python3 -c \
  "import sys,json; json.dump(json.load(sys.stdin)['result']['genesis'], open('$NODE_HOME/config/genesis.json','w'), indent=2)"
```

Or copy it directly from the seed node:

```bash
scp seed-node:/tmp/funai-e2e-real/node0/config/genesis.json $NODE_HOME/config/genesis.json
```

---

## Step 4: Configure the Seed Node

```bash
CFG="$NODE_HOME/config/config.toml"

# Set seed node
SEED="ea774f06157b0b61d5f87c6ba6467689af3adb81@34.87.21.99:46656"
sed -i "s|^seeds = .*|seeds = \"$SEED\"|" $CFG

# Allow multiple nodes on the same IP (required for testnet)
sed -i "s|addr_book_strict = true|addr_book_strict = false|" $CFG
sed -i "s|allow_duplicate_ip = false|allow_duplicate_ip = true|" $CFG

# Enable REST API
APP_TOML="$NODE_HOME/config/app.toml"
sed -i '/^\[api\]$/,/^\[/ s|^enable = false|enable = true|' $APP_TOML
```

---

## Step 5: Start the Chain Node and Sync

```bash
./build/funaid start --home $NODE_HOME > chain.log 2>&1 &

# Wait for sync to complete (check block height)
sleep 10
curl -s http://127.0.0.1:26657/status | python3 -c \
  "import sys,json; d=json.load(sys.stdin)['result']['sync_info']; print(f'height={d[\"latest_block_height\"]} catching_up={d[\"catching_up\"]}')"
```

Sync is complete when `catching_up=false`. Since the testnet block height is low, sync should finish within seconds.

---

## Step 6: Create Worker Keys

```bash
KEYRING="test"

# Generate a new Worker key
./build/funaid keys add my-worker --keyring-backend $KEYRING --home $NODE_HOME

# Record the address
WORKER_ADDR=$(./build/funaid keys show my-worker -a --keyring-backend $KEYRING --home $NODE_HOME)
echo "Worker address: $WORKER_ADDR"

# Export the private key (used for P2P node signing)
WORKER_PRIVKEY=$(echo "y" | ./build/funaid keys export my-worker --keyring-backend $KEYRING \
  --home $NODE_HOME --unsafe --unarmored-hex 2>&1 | tail -1 | tr -d '[:space:]')
echo "Worker privkey: $WORKER_PRIVKEY"

# Export the public key (hex format)
WORKER_PUBKEY=$(./build/funaid keys show my-worker -p --keyring-backend $KEYRING --home $NODE_HOME | \
  python3 -c "import sys,json,base64; print(base64.b64decode(json.load(sys.stdin)['key']).hex())")
echo "Worker pubkey: $WORKER_PUBKEY"
```

---

## Step 7: Obtain Test Tokens

New addresses have no balance and require a transfer from an existing account. Contact the seed node administrator, or use one of the following methods:

**Method A: Contact the Administrator**

Send your `$WORKER_ADDR` to the administrator and ask them to execute the following on the seed node:

```bash
# Execute on the seed node
./build/funaid tx bank send validator0 <YOUR_WORKER_ADDR> 50000000000ufai \
  --home /tmp/funai-e2e-real/node0 --keyring-backend test \
  --chain-id funai_333-1 --yes --fees 500ufai
```

**Method B: Use the Faucet (if deployed)**

```bash
curl -X POST http://34.87.21.99:8000/faucet -d '{"address":"'$WORKER_ADDR'"}'
```

---

## Step 8: Register the Worker and Stake

After confirming the balance has arrived:

```bash
# Check balance
./build/funaid query bank balances $WORKER_ADDR --home $NODE_HOME

# Register the Worker
./build/funaid tx worker register \
  --pubkey "$WORKER_PUBKEY" \
  --endpoint "http://34.143.145.204:8080" \
  --gpu-model "gpu-t4" --gpu-count 1 --gpu-vram 16 \
  --models "qwen-test" \
  --from my-worker --yes --fees 500ufai \
  --home $NODE_HOME --keyring-backend $KEYRING --chain-id funai_333-1

# Wait 1 block
sleep 3

# Stake
./build/funaid tx worker stake 10000000000ufai \
  --from my-worker --yes --fees 500ufai \
  --home $NODE_HOME --keyring-backend $KEYRING --chain-id funai_333-1

# Wait 1 block then verify
sleep 3
./build/funaid query worker worker $WORKER_ADDR --home $NODE_HOME
```

---

## Step 9: Deposit Inference Balance

If you also want to send inference requests as a user, you need to deposit into your inference account:

```bash
./build/funaid tx settlement deposit 1000000000ufai \
  --from my-worker --yes --fees 500ufai \
  --home $NODE_HOME --keyring-backend $KEYRING --chain-id funai_333-1
```

---

## Step 10: Start the P2P Inference Node

```bash
FUNAI_LISTEN_ADDR="/ip4/0.0.0.0/tcp/5001" \
FUNAI_CHAIN_RPC="http://127.0.0.1:26657" \
FUNAI_CHAIN_REST="http://127.0.0.1:1317" \
FUNAI_TGI_ENDPOINT="http://34.143.145.204:8080" \
FUNAI_WORKER_ADDR="$WORKER_ADDR" \
FUNAI_WORKER_PRIVKEY="$WORKER_PRIVKEY" \
FUNAI_MODELS="qwen-test" \
FUNAI_BOOT_PEERS="/ip4/34.87.21.99/tcp/5001/p2p/12D3KooWB6vEj2Cc7SMRK1GG5p5b2pBp8cwtdFaF6uot55nLH8rb" \
FUNAI_CHAIN_ID="funai_333-1" \
FUNAI_BATCH_INTERVAL="3s" \
FUNAI_EPSILON="0.01" \
./build/funai-node > p2p-node.log 2>&1 &

echo "P2P node started, pid=$!"
```

Verify the node has joined the network:

```bash
# Check logs
grep -E 'dispatch|worker|connected' p2p-node.log | tail -5

# After about 30 seconds you should see worker list refresh
grep 'refreshWorkerList' p2p-node.log | tail -3
```

---

## Step 11: Send an Inference Request (Optional)

```bash
E2E_USER_PRIVKEY="$WORKER_PRIVKEY" \
E2E_BOOT_PEERS="/ip4/34.87.21.99/tcp/5001/p2p/12D3KooWB6vEj2Cc7SMRK1GG5p5b2pBp8cwtdFaF6uot55nLH8rb" \
E2E_MODEL="qwen-test" \
E2E_PROMPT="What is the meaning of life?" \
E2E_FEE="1000000" \
E2E_TEMPERATURE="0" \
E2E_CHAIN_RPC="http://127.0.0.1:26657" \
E2E_CHAIN_ID="funai_333-1" \
./build/e2e-client
```

---

## Port Reference

| Port | Protocol | Purpose | Must Be Open |
|------|----------|---------|--------------|
| 26656 | TCP | CometBFT P2P (block sync) | Yes |
| 26657 | TCP | CometBFT RPC (queries/transactions) | Localhost only |
| 1317 | TCP | Cosmos REST API | Localhost only |
| 5001 | TCP | libp2p P2P (inference messages) | Yes |

---

## Common Commands

```bash
# Query block height
curl -s http://127.0.0.1:26657/status | python3 -c \
  "import sys,json; print(json.load(sys.stdin)['result']['sync_info']['latest_block_height'])"

# Query the list of registered Workers
./build/funaid query worker workers --home $NODE_HOME

# Query your own Worker status
./build/funaid query worker worker $WORKER_ADDR --home $NODE_HOME

# Query inference balance
./build/funaid query settlement account $WORKER_ADDR --home $NODE_HOME

# View P2P node logs
tail -f p2p-node.log

# Stop
kill $(cat chain.pid)   # Chain node
kill $(cat p2p.pid)     # P2P node
```

---

## Troubleshooting

| Symptom | Cause | Solution |
|---------|-------|----------|
| `catching_up=true` never finishes | Incorrect genesis file | Re-download the genesis file |
| `connection refused 46656` | Seed node firewall | Verify that 34.87.21.99:46656 is reachable |
| `worker not found` query fails | Registration tx not included in a block | Check balance and re-register |
| P2P node logs show no dispatch | Worker list not refreshed | Wait 30s, check if registration + staking succeeded |
| Inference timeout | TGI unreachable | Test with `curl http://34.143.145.204:8080/info` |
| `insufficient balance` | Insufficient inference balance | Deposit with `tx settlement deposit` |

---

## Private Key Security (Important)

The Worker private key is used to sign P2P messages such as InferReceipt, AcceptTask, and VerifyResult. If leaked, an attacker can impersonate your Worker and send forged messages, causing your Worker to be jailed and slashed.

### Must Do

1. **Do not commit the private key to code repositories, Discord, GitHub Issues, or any public channel**
2. **Do not share the same private key across multiple machines** — use an independent key for each Worker machine
3. **In production, use the Cosmos keyring OS backend** instead of passing plaintext via environment variables:
   ```bash
   # Recommended: use OS key storage (macOS Keychain / Linux secret-service / Windows Credential Manager)
   ./build/funaid keys add my-worker --keyring-backend os

   # Not recommended (for testing only):
   export FUNAI_WORKER_PRIVKEY="plaintext-private-key"  # Testnet only
   ```
4. **Rotate Worker keys regularly** — deregister the old Worker and re-register with a new key
5. **Harden the server**:
   - Disable root SSH login
   - Use SSH key authentication, disable password login
   - Regularly apply system patches
   - Use a firewall to restrict unnecessary ports

### Impact of a Leak

| Scenario | Impact | Severity |
|----------|--------|----------|
| Attacker sends fake InferReceipt with your private key | Verifier validation fails -> your Worker gets jailed + slashed 5% stake | Your funds at risk |
| Attacker sends normal inference with your private key | Worker is occupied, you cannot accept tasks | Income loss |
| What an attacker cannot do | Cannot withdraw your stake (requires on-chain tx signature, not P2P signature) | — |

**A private key leak will not result in stake theft** (the P2P signing key and the on-chain transaction signing key can be different), but it will cause the Worker to be maliciously jailed. Upon discovering a leak, immediately:
1. `funaid tx worker exit` to deregister the old Worker
2. Generate a new key and re-register

---

*Document version: 2026-04-03*
*Seed node: 34.87.21.99*
*Chain ID: funai_333-1*
*EVM Chain ID: 333*
