#!/usr/bin/env bash
# Paperclip API preflight (canonical copy — TES-21).
# Verifies AUTHENTICATED control-plane reachability BEFORE a managed run consumes
# provider work. Strict 2xx gate: 401/404/5xx and connection failures all fail
# the gate. Never mutates anything.
#
# Usage: paperclip-api-preflight.sh [timeout_per_try_sec] [tries]
# Env:   PAPERCLIP_API_URL, PAPERCLIP_API_KEY (required), PAPERCLIP_RUN_ID (optional audit)
# Exit:  0 reachable (2xx); 1 unreachable/not-2xx after all tries
#
# The adapter preflight patch invokes the equivalent logic inline; this script is
# the standalone/operator-facing form. On failure: abort the run WITHOUT consuming
# work; the adapter/runtime status channel is the sanctioned fallback for
# reporting; no issue state is stranded because nothing was written yet.
set -u
: "${PAPERCLIP_API_URL:?PAPERCLIP_API_URL not set}"
: "${PAPERCLIP_API_KEY:?PAPERCLIP_API_KEY not set}"
TIMEOUT="${1:-5}"
TRIES="${2:-3}"
# Normalize the API base: strip trailing slashes, then a trailing /api (and its
# trailing slashes), then any new trailing slash — so both http://host:3100 and
# http://host:3100/api resolve to <base>/api/health.
BASE="${PAPERCLIP_API_URL%/}"
BASE="${BASE%/api}"
BASE="${BASE%/}"
for i in $(seq 1 "$TRIES"); do
  HTTP_CODE=$(curl -sS -m "$TIMEOUT" -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer $PAPERCLIP_API_KEY" \
    "$BASE/api/health" 2>/dev/null)
  CURL_RC=$?
  if [ "$CURL_RC" -eq 0 ] && [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
    echo "preflight: ok (http $HTTP_CODE, try $i/$TRIES)"
    exit 0
  fi
  echo "preflight: attempt $i/$TRIES failed (curl rc=$CURL_RC, http=$HTTP_CODE)" >&2
  [ "$i" -lt "$TRIES" ] && sleep 1
done
echo "preflight: API not reachable (2xx) at $BASE after $TRIES tries — abort run before consuming work; bounded reschedule follows" >&2
exit 1
