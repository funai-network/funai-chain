#!/bin/bash
# wiki-lint.sh — Automated wiki health check
# Usage: bash scripts/wiki-lint.sh

WIKI_DIR="wiki"
DOCS_DIR="docs"

echo "=== FunAI Wiki Lint Report ==="
echo "Date: $(date +%Y-%m-%d)"
echo ""

# 1. Check for orphan pages (no inbound links from other pages)
echo "--- Orphan Check ---"
for f in "$WIKI_DIR"/*.md; do
  BASENAME=$(basename "$f")
  [ "$BASENAME" = "index.md" ] && continue
  [ "$BASENAME" = "log.md" ] && continue
  [ "$BASENAME" = "schema.md" ] && continue
  REFS=$(grep -rl "$BASENAME" "$WIKI_DIR"/*.md 2>/dev/null | grep -v "$f" | wc -l)
  if [ "$REFS" -eq 0 ]; then
    echo "  ORPHAN: $BASENAME (no inbound links)"
  fi
done

# 2. Check for broken internal links
echo ""
echo "--- Broken Links ---"
BROKEN=0
for f in "$WIKI_DIR"/*.md; do
  while IFS= read -r link; do
    TARGET="$WIKI_DIR/$link"
    if [ ! -f "$TARGET" ]; then
      echo "  BROKEN: $(basename "$f") -> $link"
      BROKEN=$((BROKEN + 1))
    fi
  done < <(grep -oP '\]\(\K[a-z0-9_-]+\.md(?=\))' "$f" 2>/dev/null)
done
[ "$BROKEN" -eq 0 ] && echo "  None found"

# 3. Check for docs not in wiki log
echo ""
echo "--- Uningested Sources ---"
UNINGESTED=0
for f in "$DOCS_DIR"/*.md; do
  [ -f "$f" ] || continue
  BASENAME=$(basename "$f")
  if ! grep -q "$BASENAME" "$WIKI_DIR/log.md" 2>/dev/null; then
    echo "  NEW: $BASENAME"
    UNINGESTED=$((UNINGESTED + 1))
  fi
done
[ "$UNINGESTED" -eq 0 ] && echo "  All docs ingested"

# 4. Stats
echo ""
echo "--- Stats ---"
PAGES=$(ls "$WIKI_DIR"/*.md 2>/dev/null | wc -l)
LINES=$(wc -l "$WIKI_DIR"/*.md 2>/dev/null | tail -1 | awk '{print $1}')
LINKS=$(grep -c '\[.*\](.*\.md)' "$WIKI_DIR"/*.md 2>/dev/null | awk -F: '{sum+=$2} END {print sum}')
SOURCES=$(ls "$DOCS_DIR"/*.md 2>/dev/null | wc -l)
echo "  Pages: $PAGES | Lines: $LINES | Links: $LINKS | Sources: $SOURCES"
