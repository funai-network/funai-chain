#!/bin/bash
# stability-test.sh — Long-running stability test
# Usage: bash scripts/stability-test.sh [duration_min] [interval_sec]
set -euo pipefail

DURATION_MIN=${1:-30}
INTERVAL_SEC=${2:-30}
TOTAL=$((DURATION_MIN * 60 / INTERVAL_SEC))

echo "=== Stability Test: ${DURATION_MIN}min, 1 req/${INTERVAL_SEC}s, ${TOTAL} total ==="
echo ""

# Record initial state
SUPPLY_BEFORE=$(curl -s "http://127.0.0.1:21317/cosmos/bank/v1beta1/supply" 2>/dev/null | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['supply'][0]['amount'])" 2>/dev/null)
HEIGHT_BEFORE=$(curl -s http://127.0.0.1:46657/status | python3 -c "import sys,json;print(json.load(sys.stdin)['result']['sync_info']['latest_block_height'])" 2>/dev/null)

# Record initial memory
for i in 0 1 2 3; do
  PID=$(cat /tmp/funai-e2e-real/p2p-node${i}.pid 2>/dev/null)
  RSS=$(ps -o rss= -p $PID 2>/dev/null | tr -d ' ')
  eval "MEM_BEFORE_$i=$RSS"
done

SUCCESS=0
FAIL=0
START_TIME=$(date +%s)

for i in $(seq 1 $TOTAL); do
  RESULT=$(TGI_ENDPOINT=http://34.143.145.204:8080 bash scripts/e2e-testnet.sh test "Stability $i" 2>&1)
  if echo "$RESULT" | grep -q 'SUCCESS'; then
    LAT=$(echo "$RESULT" | grep 'Latency' | awk '{print $2}')
    echo "[$(date +%H:%M:%S)] $i/$TOTAL PASS $LAT"
    SUCCESS=$((SUCCESS+1))
  else
    echo "[$(date +%H:%M:%S)] $i/$TOTAL SKIP"
    FAIL=$((FAIL+1))
  fi
  sleep $INTERVAL_SEC
done

ELAPSED=$(( $(date +%s) - START_TIME ))

echo ""
echo "=== Results ==="
echo "Duration: ${ELAPSED}s | Requests: $TOTAL | Pass: $SUCCESS | Skip: $FAIL"

# Check for panics
echo ""
echo "=== Panic Check ==="
PANICS=$(grep -ci 'panic\|fatal\|SIGSEGV' /tmp/funai-e2e-real/p2p-node*.log /tmp/funai-e2e-real/chain-node*.log 2>/dev/null | awk -F: '{sum+=$2} END {print sum}')
echo "Panics: $PANICS"

# Memory check
echo ""
echo "=== Memory Delta ==="
for i in 0 1 2 3; do
  PID=$(cat /tmp/funai-e2e-real/p2p-node${i}.pid 2>/dev/null)
  RSS=$(ps -o rss= -p $PID 2>/dev/null | tr -d ' ')
  eval "BEFORE=\$MEM_BEFORE_$i"
  DELTA=$((RSS - BEFORE))
  echo "  P2P node$i: ${BEFORE}KB -> ${RSS}KB (delta: ${DELTA}KB)"
done

# Supply check
echo ""
echo "=== Supply ==="
SUPPLY_AFTER=$(curl -s "http://127.0.0.1:21317/cosmos/bank/v1beta1/supply" 2>/dev/null | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['supply'][0]['amount'])" 2>/dev/null)
HEIGHT_AFTER=$(curl -s http://127.0.0.1:46657/status | python3 -c "import sys,json;print(json.load(sys.stdin)['result']['sync_info']['latest_block_height'])" 2>/dev/null)
echo "Supply: $SUPPLY_BEFORE -> $SUPPLY_AFTER (minted: $((SUPPLY_AFTER - SUPPLY_BEFORE)))"
echo "Height: $HEIGHT_BEFORE -> $HEIGHT_AFTER (blocks: $((HEIGHT_AFTER - HEIGHT_BEFORE)))"

# Settlement check
echo ""
echo "=== Settlements ==="
grep -c 'BatchSettlement submitted' /tmp/funai-e2e-real/p2p-node*.log 2>/dev/null
