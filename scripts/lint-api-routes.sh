#!/usr/bin/env bash
# lint-api-routes.sh — Verify that every frontend api.get/post/put/patch/delete
# call matches a backend route definition. Catches the class of bugs where a
# mismatched path falls through to the SPA fallback and returns HTML instead of
# JSON, producing "JSON.parse: unexpected character" errors.
#
# Usage:
#   ./scripts/lint-api-routes.sh          # run manually
#   make lint-routes                      # via Makefile
#   (also runs as a pre-commit git hook)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Temp files
BACKEND_FILE=$(mktemp)
FRONTEND_FILE=$(mktemp)
trap 'rm -f "$BACKEND_FILE" "$FRONTEND_FILE"' EXIT

# ── 1. Build backend route table ─────────────────────────────────────────────
# Handler package → mount prefix (from router.go api.Mount calls).
# If you add a new handler, add an entry here.
extract_routes() {
  local pkg="$1" pfx="$2"
  local handler="$ROOT/internal/$pkg/handler.go"
  [ -f "$handler" ] || return 0

  grep -E '\.(Get|Post|Put|Patch|Delete)\("/' "$handler" 2>/dev/null | while IFS= read -r line; do
    method=$(echo "$line" | sed -E 's/.*\.(Get|Post|Put|Patch|Delete)\(.*/\1/' | tr '[:upper:]' '[:lower:]')
    route=$(echo "$line" | sed -E 's/.*\.(Get|Post|Put|Patch|Delete)\("([^"]+)".*/\2/')
    [ -z "$method" ] || [ -z "$route" ] && continue
    full="${pfx}${route}"
    norm=$(echo "$full" | sed -E 's/\{[^}]+\}/:p/g')
    echo "${method} ${norm}"
  done
}

{
  extract_routes auth      /auth
  extract_routes event     /events
  extract_routes rsvp      /rsvp
  extract_routes invite    /invite
  extract_routes message   /messages
  extract_routes scheduler /reminders
  extract_routes feedback  /feedback

  # Inline routes defined directly in router.go (not via handler.go).
  grep -E 'api\.(Get|Post|Put|Patch|Delete)\("/' "$ROOT/internal/server/router.go" 2>/dev/null | while IFS= read -r line; do
    method=$(echo "$line" | sed -E 's/.*api\.(Get|Post|Put|Patch|Delete)\(.*/\1/' | tr '[:upper:]' '[:lower:]')
    route=$(echo "$line" | sed -E 's/.*api\.(Get|Post|Put|Patch|Delete)\("([^"]+)".*/\2/')
    [ -z "$method" ] || [ -z "$route" ] && continue
    norm=$(echo "$route" | sed -E 's/\{[^}]+\}/:p/g')
    echo "${method} ${norm}"
  done
} | sort -u > "$BACKEND_FILE"

backend_count=$(wc -l < "$BACKEND_FILE" | tr -d ' ')
if [ "$backend_count" -eq 0 ]; then
  echo "WARNING: No backend routes extracted. Is the project structure correct?"
  exit 1
fi

# ── 2. Extract frontend API calls ────────────────────────────────────────────
grep -rn -E 'api\.(get|post|put|patch|delete|upload)\b' "$ROOT/web/src/" \
  --include='*.svelte' --include='*.ts' 2>/dev/null \
  | grep -v 'api\.getToken' \
  | while IFS= read -r match; do

  file="${match%%:*}"
  lineinfo="${match#*:}"
  lineno="${lineinfo%%:*}"
  content="${lineinfo#*:}"

  # Extract method
  method=$(echo "$content" | sed -E 's/.*api\.(get|post|put|patch|delete|upload).*/\1/')
  # Map upload to post (upload uses POST under the hood)
  [ "$method" = "upload" ] && method="post"

  # Remove TypeScript generic type parameters: <{ data: Foo }>
  clean=$(echo "$content" | sed -E 's/<[^>]*>//g')

  path=""

  # Try backtick template: api.method(`/path/${var}`)
  if echo "$clean" | grep -q '`'; then
    candidate=$(echo "$clean" | sed -E 's/.*api\.[a-z]+\(`([^`]+)`.*/\1/')
    if [ "$candidate" != "$clean" ] && echo "$candidate" | grep -q '^/'; then
      path="$candidate"
    fi
  fi

  # Try single-quoted: api.method('/path')
  if [ -z "$path" ]; then
    candidate=$(echo "$clean" | sed -E "s/.*api\.[a-z]+\('([^']+)'.*/\1/")
    if [ "$candidate" != "$clean" ] && echo "$candidate" | grep -q '^/'; then
      path="$candidate"
    fi
  fi

  # Try double-quoted: api.method("/path")
  if [ -z "$path" ]; then
    candidate=$(echo "$clean" | sed -E 's/.*api\.[a-z]+\("([^"]+)".*/\1/')
    if [ "$candidate" != "$clean" ] && echo "$candidate" | grep -q '^/'; then
      path="$candidate"
    fi
  fi

  [ -z "$path" ] && continue

  # Normalize ${expr} → :p
  norm=$(echo "$path" | sed -E 's/\$\{[^}]+\}/:p/g')

  rel="${file#"$ROOT"/}"
  echo "${method} ${norm} ${rel}:${lineno}"
done > "$FRONTEND_FILE"

# ── 3. Compare ───────────────────────────────────────────────────────────────
ERRORS=0

while IFS= read -r line; do
  [ -z "$line" ] && continue
  method=$(echo "$line" | awk '{print $1}')
  path=$(echo "$line" | awk '{print $2}')
  file_loc=$(echo "$line" | awk '{print $3}')
  key="${method} ${path}"

  if ! grep -qF "$key" "$BACKEND_FILE"; then
    echo "  MISMATCH  ${file_loc}  $(echo "$method" | tr '[:lower:]' '[:upper:]') ${path}"
    ERRORS=$((ERRORS + 1))
  fi
done < "$FRONTEND_FILE"

# ── 4. Report ────────────────────────────────────────────────────────────────
frontend_count=$(wc -l < "$FRONTEND_FILE" | tr -d ' ')

if [ "$ERRORS" -gt 0 ]; then
  echo ""
  echo "FAIL: $ERRORS of $frontend_count frontend API call(s) have no matching backend route."
  echo ""
  echo "This usually means the frontend path doesn't match the backend mount prefix."
  echo "Check internal/server/router.go for the api.Mount() calls."
  echo ""
  echo "All registered backend routes:"
  sed 's/^/  /' "$BACKEND_FILE"
  exit 1
else
  echo "api-route-lint: OK ($frontend_count frontend calls, $backend_count backend routes)"
  exit 0
fi
