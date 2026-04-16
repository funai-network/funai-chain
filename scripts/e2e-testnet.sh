#!/bin/bash
# e2e-testnet.sh — Persistent E2E Testnet: start / test / stop / status
#
# Unlike e2e-real-inference.sh (one-shot), this keeps the testnet running
# so you can send multiple inference requests, inspect logs, debug, etc.
#
# Usage:
#   bash scripts/e2e-testnet.sh start                  # Setup chain + workers + P2P nodes
#   bash scripts/e2e-testnet.sh test                   # Send inference request
#   bash scripts/e2e-testnet.sh test "Explain gravity"  # Custom prompt
#   bash scripts/e2e-testnet.sh status                 # Show running processes
#   bash scripts/e2e-testnet.sh logs [node_idx]        # Tail P2P node logs
#   bash scripts/e2e-testnet.sh stop                   # Kill everything + cleanup
#
# Environment:
#   TGI_ENDPOINT=http://34.143.145.204:8080  (required for start)
#   MODEL_ID=qwen-test                        (default)
#   NODES=4                                   (default)
#   FUNAI_EPSILON=0.01                        (default)
set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────────

BINARY="./build/funaid"
P2P_BINARY="./build/funai-node"
CLIENT_BINARY="./build/e2e-client"
CHAIN_ID="funai_123123123-3"
BASE_DIR="/tmp/funai-e2e-real"
NODES=${NODES:-4}
DENOM="ufai"
KEYRING="test"
GENESIS_BALANCE="200000000000000${DENOM}"
STAKE_AMOUNT="100000000000000${DENOM}"
BLOCK_TIME=2

P2P_PORT_BASE=46656
RPC_PORT_BASE=46657
API_PORT_BASE=21317
GRPC_PORT_BASE=29090
P2P_LIBP2P_PORT_BASE=5001

TGI_ENDPOINT="${TGI_ENDPOINT:-http://localhost:8080}"
MODEL_ID="${MODEL_ID:-qwen-test}"
EPSILON="${FUNAI_EPSILON:-0.01}"

# ── Colors ────────────────────────────────────────────────────────────────────

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
log_pass()  { echo -e "${GREEN}[PASS]${NC} $*"; }
log_fail()  { echo -e "${RED}[FAIL]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_phase() { echo -e "\n${CYAN}══════ $* ══════${NC}\n"; }

# ── Helpers ───────────────────────────────────────────────────────────────────

cli() {
  timeout 15 $BINARY "$@" \
    --home "$BASE_DIR/node0" \
    --node "tcp://127.0.0.1:${RPC_PORT_BASE}" \
    --keyring-backend "$KEYRING" --chain-id "$CHAIN_ID" < /dev/null 2>&1
}

# cli_node runs a command using a specific node's home dir and RPC port.
cli_node() {
  local idx=$1; shift
  local rpc_port=$((RPC_PORT_BASE + idx * 2))
  timeout 15 $BINARY "$@" \
    --home "$BASE_DIR/node${idx}" \
    --node "tcp://127.0.0.1:${rpc_port}" \
    --keyring-backend "$KEYRING" --chain-id "$CHAIN_ID" < /dev/null 2>&1
}

get_addr() {
  local node_idx=$1
  $BINARY keys show "validator${node_idx}" --keyring-backend "$KEYRING" \
    --home "$BASE_DIR/node${node_idx}" -a 2>/dev/null
}

get_block_height() {
  curl -sf "http://127.0.0.1:${RPC_PORT_BASE}/status" 2>/dev/null | \
    python3 -c "import sys,json; print(json.load(sys.stdin)['result']['sync_info']['latest_block_height'])" 2>/dev/null || echo "0"
}

wait_for_blocks() {
  local target=$1 timeout_sec=${2:-60} elapsed=0
  log_info "Waiting for block $target (timeout: ${timeout_sec}s)..."
  while [ $elapsed -lt $timeout_sec ]; do
    local h=$(get_block_height)
    if [ "$h" -ge "$target" ] 2>/dev/null; then
      log_info "Chain at block $h"
      return 0
    fi
    sleep 1; elapsed=$((elapsed + 1))
  done
  log_fail "Timeout waiting for block $target"
  return 1
}

is_running() {
  local pidfile="$1"
  if [ -f "$pidfile" ]; then
    local pid=$(cat "$pidfile")
    kill -0 "$pid" 2>/dev/null && return 0
  fi
  return 1
}

# ── cmd: start ────────────────────────────────────────────────────────────────

cmd_start() {
  log_phase "Starting FunAI E2E Testnet"
  log_info "TGI: $TGI_ENDPOINT | Nodes: $NODES | Model: $MODEL_ID"

  # Check binaries
  for bin in "$BINARY" "$P2P_BINARY"; do
    if [ ! -x "$bin" ]; then
      log_fail "Binary not found: $bin — run 'make build-all'"
      exit 1
    fi
  done
  if [ ! -x "$CLIENT_BINARY" ]; then
    log_info "Building e2e-client..."
    go build -o "$CLIENT_BINARY" ./cmd/e2e-client 2>&1
  fi

  # Check TGI
  if ! curl -sf --connect-timeout 5 "$TGI_ENDPOINT/info" >/dev/null 2>&1; then
    log_fail "TGI unreachable at $TGI_ENDPOINT"
    exit 1
  fi
  log_pass "TGI reachable"

  # Kill any leftover
  cmd_stop 2>/dev/null || true
  sleep 1

  # ── Phase 1: Init chain testnet ──
  log_phase "Phase 1: Init $NODES-node chain"
  rm -rf "$BASE_DIR"; mkdir -p "$BASE_DIR"

  for i in $(seq 0 $((NODES - 1))); do
    local home="$BASE_DIR/node$i"
    $BINARY init "node$i" --chain-id "$CHAIN_ID" --home "$home" --default-denom "$DENOM" 2>/dev/null
    sed -i "s/timeout_commit = \"5s\"/timeout_commit = \"${BLOCK_TIME}s\"/" "$home/config/config.toml" 2>/dev/null || true
    sed -i "s/timeout_commit = \"5000000000\"/timeout_commit = \"${BLOCK_TIME}000000000\"/" "$home/config/config.toml" 2>/dev/null || true
    $BINARY keys add "validator$i" --keyring-backend "$KEYRING" --home "$home" 2>/dev/null
  done

  local genesis_node="$BASE_DIR/node0"
  for i in $(seq 0 $((NODES - 1))); do
    local addr=$(get_addr $i)
    $BINARY genesis add-genesis-account "$addr" "$GENESIS_BALANCE" \
      --home "$genesis_node" --keyring-backend "$KEYRING" 2>/dev/null
  done

  for i in $(seq 0 $((NODES - 1))); do
    [ "$i" -gt 0 ] && cp "$genesis_node/config/genesis.json" "$BASE_DIR/node$i/config/genesis.json"
    $BINARY genesis gentx "validator$i" "$STAKE_AMOUNT" \
      --chain-id "$CHAIN_ID" --keyring-backend "$KEYRING" --home "$BASE_DIR/node$i" 2>/dev/null
    [ "$i" -gt 0 ] && cp "$BASE_DIR/node$i/config/gentx/"*.json "$genesis_node/config/gentx/" 2>/dev/null || true
  done

  $BINARY genesis collect-gentxs --home "$genesis_node" 2>/dev/null

  # EVM: set evm_denom to ufai so EVM Gas uses the same token as native modules
  local genesis_file="$genesis_node/config/genesis.json"
  python3 -c "
import json
with open('$genesis_file') as f:
    g = json.load(f)
if 'evm' in g['app_state']:
    g['app_state']['evm']['params']['evm_denom'] = 'ufai'
    cc = g['app_state']['evm']['params']['chain_config']
    # Set all null blocks to '0' for immediate EVM availability
    for k in cc:
        if k.endswith('_block') and cc[k] is None:
            cc[k] = '0'
    # Critical: set EIP-155 chain ID and denom
    cc['chain_id'] = '9000'
    cc['denom'] = 'ufai'
    cc['decimals'] = '18'
with open('$genesis_file', 'w') as f:
    json.dump(g, f, indent=2)
" 2>/dev/null && log_info "EVM genesis configured (denom=ufai, chain_id=9000)"

  for i in $(seq 1 $((NODES - 1))); do
    cp "$genesis_node/config/genesis.json" "$BASE_DIR/node$i/config/genesis.json"
  done

  # Configure ports & peers
  local peers=""
  for i in $(seq 0 $((NODES - 1))); do
    local home="$BASE_DIR/node$i"
    local cfg="$home/config/config.toml"
    local app_toml="$home/config/app.toml"
    local p2p_port=$((P2P_PORT_BASE + i * 2))
    local rpc_port=$((RPC_PORT_BASE + i * 2))
    local api_port=$((API_PORT_BASE + i))
    local grpc_port=$((GRPC_PORT_BASE + i))

    sed -i "s|laddr = \"tcp://0.0.0.0:26656\"|laddr = \"tcp://0.0.0.0:${p2p_port}\"|" "$cfg"
    sed -i "s|laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:${rpc_port}\"|" "$cfg"
    sed -i "s|pprof_laddr = \"localhost:6060\"|pprof_laddr = \"localhost:$((26060+i))\"|" "$cfg"
    sed -i "s|address = \"tcp://localhost:1317\"|address = \"tcp://0.0.0.0:${api_port}\"|" "$app_toml" 2>/dev/null || true
    sed -i "s|address = \"localhost:9090\"|address = \"localhost:${grpc_port}\"|" "$app_toml" 2>/dev/null || true
    sed -i "s|address = \"localhost:9091\"|address = \"localhost:$((GRPC_PORT_BASE+100+i))\"|" "$app_toml" 2>/dev/null || true
    sed -i '/^\[api\]$/,/^\[/ s|^enable = false|enable = true|' "$app_toml" 2>/dev/null || true
    # Enable CORS for block explorer (Ping.pub) access
    sed -i 's|enabled-unsafe-cors = false|enabled-unsafe-cors = true|' "$app_toml" 2>/dev/null || true
    sed -i 's|cors_allowed_origins = \[\]|cors_allowed_origins = ["*"]|' "$cfg" 2>/dev/null || true

    # EVM: enable JSON-RPC on node0 only (port 8545)
    if [ "$i" -eq 0 ]; then
      cat >> "$app_toml" <<'EVMCFG'

###############################################################################
###                         EVM JSON-RPC                                    ###
###############################################################################

[json-rpc]
enable = true
address = "0.0.0.0:8545"
ws-address = "0.0.0.0:8546"
api = "eth,txpool,personal,net,debug,web3"
gas-cap = 25000000
evm-timeout = "5s"
txfee-cap = 1
filter-cap = 200
enable-indexer = true
EVMCFG
      log_info "EVM JSON-RPC enabled on node0 (:8545)"
    fi

    local client_toml="$home/config/client.toml"
    sed -i "s|node = \"tcp://localhost:26657\"|node = \"tcp://localhost:${rpc_port}\"|" "$client_toml" 2>/dev/null || true

    local node_id
    node_id=$($BINARY comet show-node-id --home "$home" 2>&1 | grep -oP '^[a-f0-9]{40}$' || $BINARY tendermint show-node-id --home "$home" 2>&1 | grep -oP '^[a-f0-9]{40}$')
    local entry="${node_id}@127.0.0.1:${p2p_port}"
    [ -z "$peers" ] && peers="$entry" || peers="${peers},${entry}"
  done

  for i in $(seq 0 $((NODES - 1))); do
    local home="$BASE_DIR/node$i"
    local node_id=$($BINARY comet show-node-id --home "$home" 2>&1 | grep -oP '^[a-f0-9]{40}$' || $BINARY tendermint show-node-id --home "$home" 2>&1 | grep -oP '^[a-f0-9]{40}$')
    local p2p_port=$((P2P_PORT_BASE + i * 2))
    local node_entry="${node_id}@127.0.0.1:${p2p_port}"
    local other_peers=$(echo "$peers" | tr ',' '\n' | grep -v "$node_entry" | tr '\n' ',' | sed 's/,$//')
    sed -i "s|^persistent_peers = .*|persistent_peers = \"${other_peers}\"|" "$home/config/config.toml"
    sed -i "s|addr_book_strict = true|addr_book_strict = false|" "$home/config/config.toml"
    sed -i "s|allow_duplicate_ip = false|allow_duplicate_ip = true|" "$home/config/config.toml"
  done
  log_pass "Chain initialized"

  # ── Start chain nodes ──
  log_phase "Phase 2: Start chain nodes"
  for i in $(seq 0 $((NODES - 1))); do
    $BINARY start --home "$BASE_DIR/node$i" > "$BASE_DIR/chain-node${i}.log" 2>&1 &
    echo $! > "$BASE_DIR/chain-node${i}.pid"
  done
  wait_for_blocks 3 90
  log_pass "Chain running"

  # ── Phase 3: Register workers + deposit ──
  log_phase "Phase 3: Register workers & deposit"

  for i in $(seq 0 $((NODES - 1))); do
    local privkey_hex
    privkey_hex=$(echo "y" | $BINARY keys export "validator${i}" --keyring-backend "$KEYRING" \
      --home "$BASE_DIR/node${i}" --unsafe --unarmored-hex 2>/dev/null | grep -oP '^[0-9a-fA-F]{64}$' || echo "")
    if [ -z "$privkey_hex" ] || [ ${#privkey_hex} -ne 64 ]; then
      privkey_hex=$(echo "y" | $BINARY keys export "validator${i}" --keyring-backend "$KEYRING" \
        --home "$BASE_DIR/node${i}" --unsafe --unarmored-hex 2>&1 | tail -1 | tr -d '[:space:]')
    fi
    echo "$privkey_hex" > "$BASE_DIR/worker${i}.privkey"

    local pubkey_hex
    pubkey_hex=$($BINARY keys show "validator${i}" --keyring-backend "$KEYRING" \
      --home "$BASE_DIR/node${i}" -p 2>/dev/null | \
      python3 -c "import sys,json,base64; d=json.load(sys.stdin); print(base64.b64decode(d.get('key','')).hex())" 2>/dev/null || echo "")
    echo "$pubkey_hex" > "$BASE_DIR/worker${i}.pubkey"

    local addr=$(get_addr $i)
    log_info "Worker $i: $addr"

    cli_node "$i" tx worker register \
      --pubkey "$pubkey_hex" \
      --endpoint "http://localhost:8080" \
      --gpu-model "gpu-t4" --gpu-count 1 --gpu-vram 16 \
      --models "$MODEL_ID" \
      --from "validator${i}" --yes --fees 500ufai 2>/dev/null || true
  done

  wait_for_blocks $(($(get_block_height) + 2)) 30

  for i in $(seq 0 $((NODES - 1))); do
    local addr=$(get_addr $i)
    cli_node "$i" tx worker stake "10000000000${DENOM}" \
      --from "validator${i}" --yes --fees 500ufai 2>/dev/null || true
  done

  wait_for_blocks $(($(get_block_height) + 2)) 30

  # Verify worker registrations
  for i in $(seq 0 $((NODES - 1))); do
    local addr=$(get_addr $i)
    local result=$(cli query worker worker "$addr" 2>/dev/null || echo "not found")
    if echo "$result" | grep -q "address"; then
      log_pass "Worker $i registered: $addr"
    else
      log_warn "Worker $i registration not confirmed: $addr"
    fi
  done

  # Deposit for validator0 (used as SDK user)
  local user_addr=$(get_addr 0)
  cli tx settlement deposit "1000000000${DENOM}" \
    --from validator0 --yes --fees 500ufai 2>/dev/null || true

  wait_for_blocks $(($(get_block_height) + 2)) 30

  # Export SDK user privkey
  local val0_privkey=$(cat "$BASE_DIR/worker0.privkey")
  echo "$val0_privkey" > "$BASE_DIR/sdk-user.privkey"
  log_pass "Workers registered, funds deposited"

  # ── Phase 4: Start P2P nodes ──
  log_phase "Phase 4: Start P2P nodes"
  local first_p2p_addr=""
  for i in $(seq 0 $((NODES - 1))); do
    local addr=$(get_addr $i)
    local privkey_hex=$(cat "$BASE_DIR/worker${i}.privkey")
    local libp2p_port=$((P2P_LIBP2P_PORT_BASE + i))
    local rpc_port=$((RPC_PORT_BASE + i * 2))
    local api_port=$((API_PORT_BASE + i))
    local boot_peers=""
    [ -n "$first_p2p_addr" ] && boot_peers="$first_p2p_addr"

    FUNAI_LISTEN_ADDR="/ip4/0.0.0.0/tcp/${libp2p_port}" \
    FUNAI_CHAIN_RPC="http://127.0.0.1:${rpc_port}" \
    FUNAI_CHAIN_REST="http://127.0.0.1:${api_port}" \
    FUNAI_TGI_ENDPOINT="$TGI_ENDPOINT" \
    FUNAI_WORKER_ADDR="$addr" \
    FUNAI_WORKER_PRIVKEY="$privkey_hex" \
    FUNAI_MODELS="$MODEL_ID" \
    FUNAI_BOOT_PEERS="$boot_peers" \
    FUNAI_METRICS_ADDR=":$((19100+i))" \
    FUNAI_EPSILON="$EPSILON" \
    FUNAI_CHAIN_ID="$CHAIN_ID" \
    FUNAI_BATCH_INTERVAL="3s" \
    $P2P_BINARY > "$BASE_DIR/p2p-node${i}.log" 2>&1 &
    echo $! > "$BASE_DIR/p2p-node${i}.pid"
    log_info "P2P node $i started (pid=$!, port=$libp2p_port)"

    if [ "$i" -eq 0 ]; then
      sleep 3
      local peer_id=$(grep -oP 'peer_id=\K[A-Za-z0-9]+' "$BASE_DIR/p2p-node0.log" 2>/dev/null | head -1 || echo "")
      if [ -n "$peer_id" ]; then
        first_p2p_addr="/ip4/127.0.0.1/tcp/${libp2p_port}/p2p/${peer_id}"
      else
        first_p2p_addr=$(grep -oP 'Listening: \K[^\s]+' "$BASE_DIR/p2p-node0.log" 2>/dev/null | head -1 || echo "")
      fi
      echo "$first_p2p_addr" > "$BASE_DIR/bootstrap-peer"
      log_info "Bootstrap: $first_p2p_addr"
    fi
  done

  log_info "Waiting 15s for P2P mesh formation..."
  sleep 15

  local running=0
  for i in $(seq 0 $((NODES - 1))); do
    is_running "$BASE_DIR/p2p-node${i}.pid" && running=$((running + 1))
  done
  log_pass "$running/$NODES P2P nodes running"

  # ── Save state ──
  echo "$TGI_ENDPOINT" > "$BASE_DIR/tgi-endpoint"
  echo "$MODEL_ID" > "$BASE_DIR/model-id"
  echo "$CHAIN_ID" > "$BASE_DIR/chain-id"

  echo ""
  echo -e "${GREEN}Testnet is running!${NC}"
  echo -e "  Chain RPC:  http://127.0.0.1:${RPC_PORT_BASE}"
  echo -e "  Chain REST: http://127.0.0.1:${API_PORT_BASE}"
  echo -e "  P2P nodes:  ports ${P2P_LIBP2P_PORT_BASE}-$((P2P_LIBP2P_PORT_BASE+NODES-1))"
  echo -e "  TGI:        $TGI_ENDPOINT"
  echo ""
  echo -e "  Send inference:  ${CYAN}bash scripts/e2e-testnet.sh test${NC}"
  echo -e "  Custom prompt:   ${CYAN}bash scripts/e2e-testnet.sh test \"your prompt here\"${NC}"
  echo -e "  View logs:       ${CYAN}bash scripts/e2e-testnet.sh logs [0-3]${NC}"
  echo -e "  Check status:    ${CYAN}bash scripts/e2e-testnet.sh status${NC}"
  echo -e "  Stop:            ${CYAN}bash scripts/e2e-testnet.sh stop${NC}"
}

# ── cmd: test ─────────────────────────────────────────────────────────────────

cmd_test() {
  local prompt="${1:-What is 2+2? Answer with just the number.}"

  if [ ! -f "$BASE_DIR/bootstrap-peer" ]; then
    log_fail "Testnet not running. Run: bash scripts/e2e-testnet.sh start"
    exit 1
  fi

  local boot_peer=$(cat "$BASE_DIR/bootstrap-peer")
  local sdk_privkey=$(cat "$BASE_DIR/sdk-user.privkey")
  local model_id=$(cat "$BASE_DIR/model-id" 2>/dev/null || echo "qwen-test")
  local chain_id=$(cat "$BASE_DIR/chain-id" 2>/dev/null || echo "funai-e2e-real-1")

  if [ ! -x "$CLIENT_BINARY" ]; then
    log_info "Building e2e-client..."
    go build -o "$CLIENT_BINARY" ./cmd/e2e-client 2>&1
  fi

  log_info "Prompt: \"$prompt\""
  log_info "Model: $model_id | Boot: $boot_peer"
  echo ""

  E2E_USER_PRIVKEY="$sdk_privkey" \
  E2E_BOOT_PEERS="$boot_peer" \
  E2E_MODEL="$model_id" \
  E2E_PROMPT="$prompt" \
  E2E_FEE="1000000" \
  E2E_TEMPERATURE="0" \
  E2E_CHAIN_RPC="http://127.0.0.1:${RPC_PORT_BASE}" \
  E2E_CHAIN_ID="$chain_id" \
  timeout 60 $CLIENT_BINARY 2>&1

  echo ""
  log_pass "Inference request completed"
}

# ── cmd: status ───────────────────────────────────────────────────────────────

cmd_status() {
  echo -e "${CYAN}FunAI E2E Testnet Status${NC}"
  echo ""

  # Chain nodes
  local chain_running=0
  for i in $(seq 0 $((NODES - 1))); do
    if is_running "$BASE_DIR/chain-node${i}.pid"; then
      local pid=$(cat "$BASE_DIR/chain-node${i}.pid")
      echo -e "  Chain node $i:  ${GREEN}running${NC} (pid=$pid)"
      chain_running=$((chain_running + 1))
    else
      echo -e "  Chain node $i:  ${RED}stopped${NC}"
    fi
  done

  # P2P nodes
  local p2p_running=0
  for i in $(seq 0 $((NODES - 1))); do
    if is_running "$BASE_DIR/p2p-node${i}.pid"; then
      local pid=$(cat "$BASE_DIR/p2p-node${i}.pid")
      echo -e "  P2P node $i:   ${GREEN}running${NC} (pid=$pid)"
      p2p_running=$((p2p_running + 1))
    else
      echo -e "  P2P node $i:   ${RED}stopped${NC}"
    fi
  done

  echo ""
  if [ "$chain_running" -gt 0 ]; then
    local height=$(get_block_height)
    echo -e "  Block height: $height"
  fi

  local tgi=$(cat "$BASE_DIR/tgi-endpoint" 2>/dev/null || echo "unknown")
  echo -e "  TGI endpoint: $tgi"

  if [ "$chain_running" -eq "$NODES" ] && [ "$p2p_running" -eq "$NODES" ]; then
    echo -e "\n  ${GREEN}All services healthy${NC}"
  elif [ "$chain_running" -eq 0 ] && [ "$p2p_running" -eq 0 ]; then
    echo -e "\n  ${RED}Testnet not running${NC}. Run: bash scripts/e2e-testnet.sh start"
  else
    echo -e "\n  ${YELLOW}Partially running${NC} (chain=$chain_running/$NODES, p2p=$p2p_running/$NODES)"
  fi
}

# ── cmd: logs ─────────────────────────────────────────────────────────────────

cmd_logs() {
  local idx="${1:-0}"
  local logfile="$BASE_DIR/p2p-node${idx}.log"
  if [ ! -f "$logfile" ]; then
    log_fail "Log not found: $logfile"
    exit 1
  fi
  echo -e "${CYAN}=== P2P Node $idx Logs (tail -f) ===${NC}"
  tail -f "$logfile"
}

# ── cmd: stop ─────────────────────────────────────────────────────────────────

cmd_stop() {
  log_info "Stopping testnet..."

  for i in $(seq 0 $((NODES - 1))); do
    for prefix in p2p-node chain-node; do
      local pidfile="$BASE_DIR/${prefix}${i}.pid"
      if [ -f "$pidfile" ]; then
        local pid=$(cat "$pidfile")
        kill "$pid" 2>/dev/null || true
      fi
    done
  done

  # Belt and suspenders
  pkill -f "funaid.*funai-e2e-real" 2>/dev/null || true
  pkill -f "funai-node.*funai-e2e-real" 2>/dev/null || true

  sleep 1
  log_pass "All processes stopped"
  log_info "Data preserved at $BASE_DIR"
  log_info "To clean up: rm -rf $BASE_DIR"
}

# ── Main ──────────────────────────────────────────────────────────────────────

case "${1:-}" in
  start)
    cmd_start
    ;;
  test)
    shift
    cmd_test "$@"
    ;;
  status)
    cmd_status
    ;;
  logs)
    shift
    cmd_logs "${1:-0}"
    ;;
  stop)
    cmd_stop
    ;;
  *)
    echo "Usage: bash scripts/e2e-testnet.sh {start|test|status|logs|stop}"
    echo ""
    echo "  start              Setup and start chain + P2P testnet (stays running)"
    echo "  test [prompt]      Send inference request"
    echo "  status             Show running processes and block height"
    echo "  logs [0-3]         Tail P2P node logs"
    echo "  stop               Kill all processes"
    echo ""
    echo "Environment:"
    echo "  TGI_ENDPOINT       TGI server URL (required for start)"
    echo "  MODEL_ID           Model name (default: qwen-test)"
    echo "  NODES              Number of nodes (default: 4)"
    exit 1
    ;;
esac
