#!/usr/bin/env bash
set -euo pipefail

# Generate test emails by exercising the running app so Mailpit captures them.
#
# Prerequisites:
#   docker compose up -d   (starts openrsvp + mailpit)
#
# Usage:
#   ./tests/visual/generate-test-emails.sh
#
# After running, open http://localhost:8025 to inspect the emails,
# then run the Playwright visual tests:
#   npm run test:visual

BASE_URL="${BASE_URL:-http://localhost:8091}"
MAILPIT_URL="${MAILPIT_URL:-http://localhost:8025}"

echo "=== OpenRSVP email test generator ==="
echo ""
echo "App URL:     $BASE_URL"
echo "Mailpit URL: $MAILPIT_URL"
echo ""

# --- Preflight checks ---

if ! curl -sf "$BASE_URL/health" > /dev/null 2>&1; then
  echo "ERROR: App is not responding at $BASE_URL/health"
  echo "       Run: docker compose up -d"
  exit 1
fi

if ! curl -sf "$MAILPIT_URL/api/v1/messages" > /dev/null 2>&1; then
  echo "ERROR: Mailpit is not responding at $MAILPIT_URL"
  echo "       Make sure mailpit is running on the openrsvp-net network."
  exit 1
fi

# --- Clear existing emails for a clean slate ---

echo "Clearing existing Mailpit messages..."
curl -sf -X DELETE "$MAILPIT_URL/api/v1/messages" > /dev/null 2>&1 || true
echo ""

# --- Trigger a magic link email ---

echo "1. Triggering magic link email (test@example.com)..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com"}')
echo "   Response: HTTP $HTTP_CODE"

# --- Trigger a second magic link for variety ---

echo "2. Triggering magic link email (demo@example.com)..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@example.com"}')
echo "   Response: HTTP $HTTP_CODE"

# --- Wait a moment for async delivery ---

sleep 2

# --- Verify ---

TOTAL=$(curl -sf "$MAILPIT_URL/api/v1/messages" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "?")
echo ""
echo "=== Done ==="
echo "Emails in Mailpit: $TOTAL"
echo ""
echo "Inspect emails:    $MAILPIT_URL"
echo "Run visual tests:  cd web && npm run test:visual"
