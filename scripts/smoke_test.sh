#!/usr/bin/env bash
# End-to-end smoke test against a running MicroHub deployment (local
# `go run`, docker-compose, or `kubectl port-forward`'d from a cluster).
#
# Usage:
#   USER_URL=http://localhost:8081 MATCH_URL=http://localhost:8080 \
#     NOTIF_URL=http://localhost:8082 ./scripts/smoke_test.sh
set -euo pipefail

USER_URL="${USER_URL:-http://localhost:8081}"
MATCH_URL="${MATCH_URL:-http://localhost:8080}"
NOTIF_URL="${NOTIF_URL:-http://localhost:8082}"

echo "== health checks =="
curl -sS -f "$USER_URL/healthz" && echo
curl -sS -f "$MATCH_URL/healthz" && echo
curl -sS -f "$NOTIF_URL/healthz" && echo

echo "== create two users =="
U1_JSON=$(curl -sS -X POST "$USER_URL/users" -d '{"name":"Maggie","age":24,"location":"SF","interests":["hiking","go","boba"]}')
U1_ID=$(echo "$U1_JSON" | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])")
U2_JSON=$(curl -sS -X POST "$USER_URL/users" -d '{"name":"Alex","age":25,"location":"SF","interests":["climbing","go"]}')
U2_ID=$(echo "$U2_JSON" | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])")
echo "created users $U1_ID ($U1_JSON) and $U2_ID ($U2_JSON)"

echo "== one-sided swipe (expect matched:false) =="
curl -sS -X POST "$MATCH_URL/swipes" -d "{\"userId\":$U1_ID,\"targetId\":$U2_ID,\"liked\":true}"
echo

echo "== reciprocating swipe (expect matched:true) =="
curl -sS -X POST "$MATCH_URL/swipes" -d "{\"userId\":$U2_ID,\"targetId\":$U1_ID,\"liked\":true}"
echo

echo "== matches for user $U1_ID =="
curl -sS "$MATCH_URL/matches/$U1_ID"
echo

echo "== notifications for both users =="
curl -sS "$NOTIF_URL/notifications/$U1_ID"
echo
curl -sS "$NOTIF_URL/notifications/$U2_ID"
echo

echo "== smoke test passed =="
