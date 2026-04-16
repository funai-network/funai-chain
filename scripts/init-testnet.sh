#!/bin/bash
# init-testnet.sh — Initialize a multi-node local testnet for FunAI Chain.
# Creates separate home directories under ~/.funai-testnet/{node0..nodeN}.
# Usage: bash scripts/init-testnet.sh [--clean]
#
# Environment variables:
#   TESTNET_DIR   — data directory (default: ~/.funai-testnet)
#   EXTERNAL_IP   — advertised IP for persistent_peers (default: auto-detect)
#   NODES         — number of validator nodes (default: 4)
set -euo pipefail

BINARY=${BINARY:-funaid}
CHAIN_ID="funai-testnet-1"
BASE_DIR=${TESTNET_DIR:-$HOME/.funai-testnet}
NODES=${NODES:-4}
DENOM="ufai"
# Each validator gets 200M FAI genesis balance; stakes 100M
GENESIS_BALANCE="200000000000000${DENOM}"
STAKE_AMOUNT="100000000000000${DENOM}"
# Start ports — each node offsets by 2
P2P_PORT_BASE=26656
RPC_PORT_BASE=26657
REST_PORT_BASE=1317

# Resolve advertised IP: prefer EXTERNAL_IP env, then auto-detect, fallback 127.0.0.1
if [[ -n "${EXTERNAL_IP:-}" ]]; then
  ADVERTISE_IP="$EXTERNAL_IP"
else
  ADVERTISE_IP=$(curl -s --max-time 3 https://ifconfig.me 2>/dev/null || \
                 hostname -I 2>/dev/null | awk '{print $1}' || \
                 echo "127.0.0.1")
fi
echo "==> Advertised IP: $ADVERTISE_IP"

if [[ "${1:-}" == "--clean" ]]; then
  echo "Cleaning $BASE_DIR..."
  rm -rf "$BASE_DIR"
fi

mkdir -p "$BASE_DIR"

echo "==> Initializing $NODES nodes in $BASE_DIR"

# Step 1: Init each node
for i in $(seq 0 $((NODES - 1))); do
  NODE_HOME="$BASE_DIR/node$i"
  "$BINARY" init "node$i" --chain-id "$CHAIN_ID" --home "$NODE_HOME" --default-denom "$DENOM" 2>/dev/null
  "$BINARY" keys add "validator$i" --keyring-backend test --home "$NODE_HOME" 2>/dev/null
  echo "  Node $i initialized: $NODE_HOME"
done

# Step 2: Collect genesis accounts — use node0 as the canonical genesis
GENESIS_NODE="$BASE_DIR/node0"

for i in $(seq 0 $((NODES - 1))); do
  NODE_HOME="$BASE_DIR/node$i"
  ADDR=$("$BINARY" keys show "validator$i" --keyring-backend test --home "$NODE_HOME" -a)
  "$BINARY" genesis add-genesis-account "$ADDR" "$GENESIS_BALANCE" \
    --home "$GENESIS_NODE" --keyring-backend test 2>/dev/null
  echo "  Added genesis account for node$i: $ADDR"
done

# Step 3: Each node creates a gentx
for i in $(seq 0 $((NODES - 1))); do
  NODE_HOME="$BASE_DIR/node$i"
  # Copy genesis to each node so gentx can find accounts (skip node0 — it's the source)
  [[ "$i" -gt 0 ]] && cp "$GENESIS_NODE/config/genesis.json" "$NODE_HOME/config/genesis.json"
  "$BINARY" genesis gentx "validator$i" "$STAKE_AMOUNT" \
    --chain-id "$CHAIN_ID" \
    --keyring-backend test \
    --home "$NODE_HOME" 2>/dev/null
  # Copy gentx to node0 for collection
  cp "$NODE_HOME/config/gentx/"*.json "$GENESIS_NODE/config/gentx/" 2>/dev/null || true
  echo "  Node $i gentx created"
done

# Step 4: Collect all gentxs into node0's genesis
"$BINARY" genesis collect-gentxs --home "$GENESIS_NODE" 2>/dev/null

FINAL_GENESIS="$GENESIS_NODE/config/genesis.json"

# Step 5: Distribute final genesis to all other nodes
for i in $(seq 1 $((NODES - 1))); do
  cp "$FINAL_GENESIS" "$BASE_DIR/node$i/config/genesis.json"
done
echo "  Final genesis distributed to all nodes"

# Step 6: Collect node IDs and configure ports
declare -a NODE_IDS
for i in $(seq 0 $((NODES - 1))); do
  NODE_HOME="$BASE_DIR/node$i"
  CFG="$NODE_HOME/config/config.toml"

  NODE_IDS[$i]=$("$BINARY" comet show-node-id --home "$NODE_HOME" 2>&1 | grep -v WARNING || \
                  "$BINARY" tendermint show-node-id --home "$NODE_HOME" 2>&1 | grep -v WARNING)

  P2P_PORT=$((P2P_PORT_BASE + i * 2))
  RPC_PORT=$((RPC_PORT_BASE + i * 2))

  REST_PORT=$((REST_PORT_BASE + i))
  GRPC_PORT=$((9090 + i))

  # Patch config.toml — P2P, RPC ports
  sed -i "s|laddr = \"tcp://0.0.0.0:26656\"|laddr = \"tcp://0.0.0.0:${P2P_PORT}\"|" "$CFG"
  sed -i "s|laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:${RPC_PORT}\"|" "$CFG"
  # Advertise external address so remote peers can connect back
  sed -i "s|external_address = \"\"|external_address = \"tcp://${ADVERTISE_IP}:${P2P_PORT}\"|" "$CFG"
  # CORS for browser-based explorer
  sed -i 's|cors_allowed_origins = \[\]|cors_allowed_origins = ["*"]|' "$CFG"
  # Local testnet: allow same-IP peers, disable PEX (avoid external_address pollution)
  sed -i "s|addr_book_strict = true|addr_book_strict = false|" "$CFG"
  sed -i "s|allow_duplicate_ip = false|allow_duplicate_ip = true|" "$CFG"
  sed -i "s|^pex = true|pex = false|" "$CFG"

  # Patch app.toml — REST API, gRPC ports (avoid conflicts between nodes)
  APP="$NODE_HOME/config/app.toml"
  sed -i '/^\[api\]$/,/^\[/ { s|^enable = false|enable = true| }' "$APP"
  sed -i "s|address = \"tcp://localhost:1317\"|address = \"tcp://0.0.0.0:${REST_PORT}\"|" "$APP"
  sed -i 's|enabled-unsafe-cors = false|enabled-unsafe-cors = true|' "$APP"
  sed -i "s|address = \"localhost:9090\"|address = \"localhost:${GRPC_PORT}\"|" "$APP"

  echo "  Node $i: node_id=${NODE_IDS[$i]} p2p=:${P2P_PORT} rpc=:${RPC_PORT} rest=:${REST_PORT} grpc=:${GRPC_PORT}"
done

# Step 7: Build and write persistent_peers (each node gets all others, excluding itself)
for i in $(seq 0 $((NODES - 1))); do
  NODE_HOME="$BASE_DIR/node$i"
  OTHER_PEERS=""
  for j in $(seq 0 $((NODES - 1))); do
    [[ "$i" -eq "$j" ]] && continue
    P2P_PORT=$((P2P_PORT_BASE + j * 2))
    ENTRY="${NODE_IDS[$j]}@127.0.0.1:${P2P_PORT}"
    if [[ -z "$OTHER_PEERS" ]]; then
      OTHER_PEERS="$ENTRY"
    else
      OTHER_PEERS="${OTHER_PEERS},${ENTRY}"
    fi
  done
  sed -i "s|^persistent_peers = .*|persistent_peers = \"${OTHER_PEERS}\"|" "$NODE_HOME/config/config.toml"
done

# Build full peers string for external nodes
ALL_PEERS=""
for i in $(seq 0 $((NODES - 1))); do
  P2P_PORT=$((P2P_PORT_BASE + i * 2))
  ENTRY="${NODE_IDS[$i]}@${ADVERTISE_IP}:${P2P_PORT}"
  if [[ -z "$ALL_PEERS" ]]; then
    ALL_PEERS="$ENTRY"
  else
    ALL_PEERS="${ALL_PEERS},${ENTRY}"
  fi
done

echo ""
echo "==> Testnet initialized successfully!"
echo ""
echo "Start nodes with:"
for i in $(seq 0 $((NODES - 1))); do
  RPC_PORT=$((RPC_PORT_BASE + i * 2))
  echo "  $BINARY start --home $BASE_DIR/node$i  # RPC :${RPC_PORT}"
done
echo ""
echo "External nodes can join with:"
echo "  persistent_peers=\"$ALL_PEERS\""
echo "  Genesis: $FINAL_GENESIS"
