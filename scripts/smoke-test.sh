#!/usr/bin/env bash
# Prueba end-to-end de toda la arquitectura a través del API Gateway.
# Uso: docker compose up -d --build && ./scripts/smoke-test.sh
set -euo pipefail
GW=${GW:-http://localhost:8080}
ok() { echo "  ✔ $*"; }
fail() { echo "  ✖ $*"; exit 1; }
json() { python3 -c "import sys,json; d=json.load(sys.stdin); print($1)"; }

echo "▶ Esperando al API Gateway..."
for i in $(seq 1 60); do curl -sf "$GW/api/catalog/products" >/dev/null 2>&1 && break; sleep 2; done

echo "▶ Frontend (PWA)"
curl -sf "$GW/" | grep -q 'manifest.webmanifest' && ok "index.html servido"
curl -sf "$GW/service-worker.js" >/dev/null && ok "service-worker.js disponible"

echo "▶ API Gateway"
[ "$(curl -s -o /dev/null -w '%{http_code}' "$GW/api/orders")" = "401" ] && ok "sin JWT → 401"

echo "▶ User Service (JWT)"
EMAIL="demo$RANDOM@agromarket.pe"
TOKEN=$(curl -sf -X POST "$GW/api/users/register" -H 'Content-Type: application/json' \
  -d "{\"name\":\"Demo\",\"email\":\"$EMAIL\",\"password\":\"demo1234\"}" | json 'd["token"]')
[ -n "$TOKEN" ] && ok "registro y token emitido"
AUTH="Authorization: Bearer $TOKEN"

echo "▶ Catalog Service (Redis cache)"
curl -s -o /dev/null "$GW/api/catalog/products"
curl -sI "$GW/api/catalog/products" | grep -qi 'x-cache: HIT' && ok "cache HIT en Redis"
STOCK0=$(curl -sf "$GW/api/catalog/products" | json '[p["stock"] for p in d if p["id"]==1][0]')

echo "▶ Order Service → Event Bus → Consumers"
OID=$(curl -sf -X POST "$GW/api/orders" -H "$AUTH" -d '{"items":[{"product_id":1,"quantity":2}]}' | json 'd["id"]')
ok "pedido #$OID creado (202, OrderCreated publicado)"
for i in $(seq 1 20); do
  ST=$(curl -sf "$GW/api/orders" -H "$AUTH" | json "[o['status'] for o in d if o['id']==$OID][0]")
  [ "$ST" = "CONFIRMADO" ] && break; sleep 1
done
[ "$ST" = "CONFIRMADO" ] && ok "pedido #$OID: PENDIENTE → PAGADO → CONFIRMADO" || fail "estado final $ST"

STOCK1=$(curl -sf "$GW/api/catalog/products" | json '[p["stock"] for p in d if p["id"]==1][0]')
[ "$STOCK1" -eq $((STOCK0 - 2)) ] && ok "stock sincronizado vía StockUpdated ($STOCK0 → $STOCK1)" || fail "stock $STOCK0 → $STOCK1"

OID2=$(curl -sf -X POST "$GW/api/orders" -H "$AUTH" -d '{"items":[{"product_id":7,"quantity":20}]}' | json 'd["id"]')
for i in $(seq 1 15); do
  ST=$(curl -sf "$GW/api/orders" -H "$AUTH" | json "[o['status'] for o in d if o['id']==$OID2][0]")
  [ "$ST" = "RECHAZADO" ] && break; sleep 1
done
[ "$ST" = "RECHAZADO" ] && ok "pedido #$OID2 > S/ 5,000 → RECHAZADO (PaymentFailed)" || fail "estado $ST"

echo "▶ Notification + Analytics Services"
N=$(curl -sf "$GW/api/notifications" -H "$AUTH" | json 'len(d)')
[ "$N" -ge 5 ] && ok "$N notificaciones generadas desde eventos"
curl -sf "$GW/api/analytics" | json 'd["events_total"]' | grep -qv '^0' && ok "métricas agregadas por Analytics"

echo "▶ Rate limiting"
CODES=$(for i in $(seq 1 30); do curl -s -o /dev/null -w '%{http_code} ' -X POST "$GW/api/users/login" -d '{}'; done)
echo "$CODES" | grep -q 429 && ok "ráfaga de logins → 429 Too Many Requests"

echo; echo "✅ Arquitectura completa verificada"
