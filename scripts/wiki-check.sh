#!/bin/bash
# wiki-check.sh — Check if new docs need wiki ingestion
# Exit code 0 = all good, exit code 2 = new docs found
# Usage: bash scripts/wiki-check.sh

DOCS_DIR="docs"
WIKI_LOG="wiki/log.md"

[ -f "$WIKI_LOG" ] || { echo "[wiki] No wiki/log.md found."; exit 1; }

MISSING=()
for f in "$DOCS_DIR"/*.md; do
  [ -f "$f" ] || continue
  BASENAME=$(basename "$f")
  if ! grep -q "$BASENAME" "$WIKI_LOG"; then
    MISSING+=("$f")
  fi
done

if [ ${#MISSING[@]} -gt 0 ]; then
  echo "[wiki] Uningested docs found (${#MISSING[@]}):"
  for f in "${MISSING[@]}"; do
    echo "  $f"
  done
  exit 2
fi

echo "[wiki] All docs ingested."
exit 0
