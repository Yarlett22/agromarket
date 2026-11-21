#!/usr/bin/env bash
# Prueba end-to-end de toda la arquitectura SEGURA a través del API Gateway:
#   TLS · WAF (ModSecurity + OWASP CRS) · Keycloak (JWT RS256) · NATS TLS · PostgreSQL TLS · flujo de eventos
# Uso: docker compose up -d --build && ./scripts/smoke-test.sh
set -euo pipefail
GW=${GW:-https://localhost:8443}
HTTP=${HTTP:-http://localhost:8080}
C="curl -sk"              # -k: certificado firmado por la CA interna (no por una CA pública)
ok()   { echo "  ✔ $*"; }
fail() { echo "  ✖ $*"; exit 1; }
json() { python3 -c "import sys,json; d=json.load(sys.stdin); print($1)"; }

echo "▶ Esperando al gateway y a Keycloak..."
for i in $(seq 1 90); do
  $C -f "$GW/auth/realms/agromarket/.well-known/openid-configuration" >/dev/null 2>&1 && \
  $C -f "$GW/api/catalog/products" >/dev/null 2>&1 && break
  sleep 3
done

echo "▶ TLS"
[ "$(curl -s -o /dev/null -w '%{http_code}' "$HTTP/")" = "301" ] && ok "HTTP :8080 → 301 redirección a HTTPS" || fail "falló: HTTP :8080 → 301 redirección a HTTPS"
docker compose exec -T gateway cat /certs/ca.crt > /tmp/agromarket-ca.crt
curl -s --cacert /tmp/agromarket-ca.crt -o /dev/null "$GW/api/health" && ok "certificado del gateway válido según la CA interna" || fail "falló: certificado del gateway válido según la CA interna"
PROTO=$($C "$GW/api/health" | json 'd["tls"]'); ok "protocolo negociado: $PROTO"
$C -I "$GW/" | grep -qi 'strict-transport-security' && ok "cabecera HSTS presente" || fail "falló: cabecera HSTS presente"

echo "▶ Frontend (PWA)"
$C -f "$GW/" | grep -q 'manifest.webmanifest' && ok "index.html servido por HTTPS" || fail "falló: index.html servido por HTTPS"
$C -f "$GW/service-worker.js" >/dev/null && ok "service-worker.js disponible" || fail "falló: service-worker.js disponible"

echo "▶ Keycloak (OIDC, tokens RS256)"
TOKEN=$($C -X POST "$GW/auth/realms/agromarket/protocol/openid-connect/token" \
  -d grant_type=password -d client_id=agromarket-web -d username=demo -d password=demo1234 | json 'd["access_token"]')
[ -n "$TOKEN" ] && ok "Keycloak emitió un access token para el usuario demo" || fail "falló: Keycloak emitió un access token para el usuario demo"
ALG=$(echo "$TOKEN" | cut -d. -f1 | python3 -c "import sys,base64,json; s=sys.stdin.read().strip(); print(json.loads(base64.urlsafe_b64decode(s+'='*(-len(s)%4)))['alg'])")
[ "$ALG" = "RS256" ] && ok "token firmado con $ALG" || fail "falló: token firmado con $ALG"
AUTH="Authorization: Bearer $TOKEN"

echo "▶ API Gateway (validación del JWT)"
[ "$($C -o /dev/null -w '%{http_code}' "$GW/api/orders")" = "401" ] && ok "sin token → 401" || fail "falló: sin token → 401"
FAKE="eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIiwiYXpwIjoiYWdyb21hcmtldC13ZWIifQ.x"
[ "$($C -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $FAKE" "$GW/api/orders")" = "401" ] && ok "token falsificado (HS256) → 401" || fail "falló: token falsificado (HS256) → 401"
ME=$($C -H "$AUTH" "$GW/api/users/me" | json 'd["username"]+" (roles: "+",".join(d["roles"])+")"'); ok "perfil vía User Service: $ME"

echo "▶ WAF (ModSecurity + OWASP CRS)"
[ "$($C -o /dev/null -w '%{http_code}' "$GW/api/catalog/products?q=1%27%20OR%20%271%27%3D%271")" = "403" ] && ok "inyección SQL → 403 bloqueado" || fail "falló: inyección SQL → 403 bloqueado"
[ "$($C -o /dev/null -w '%{http_code}' "$GW/api/catalog/products?q=%3Cscript%3Ealert(1)%3C%2Fscript%3E")" = "403" ] && ok "XSS → 403 bloqueado" || fail "falló: XSS → 403 bloqueado"
[ "$($C -o /dev/null -w '%{http_code}' "$GW/api/catalog/products?file=../../etc/passwd")" = "403" ] && ok "path traversal → 403 bloqueado" || fail "falló: path traversal → 403 bloqueado"
[ "$($C -o /dev/null -w '%{http_code}' -A 'nikto' "$GW/api/catalog/products")" = "403" ] && ok "escáner de vulnerabilidades (nikto) → 403 bloqueado" || fail "falló: escáner de vulnerabilidades (nikto) → 403 bloqueado"

echo "▶ Catalog Service (Redis cache)"
$C -o /dev/null "$GW/api/catalog/products"
$C -I "$GW/api/catalog/products" | grep -qi 'x-cache: HIT' && ok "cache HIT en Redis" || fail "falló: cache HIT en Redis"
STOCK0=$($C -f "$GW/api/catalog/products" | json '[p["stock"] for p in d if p["id"]==1][0]')

echo "▶ Order Service → Event Bus (TLS) → Consumers"
JSONH="Content-Type: application/json"
OID=$($C -f -X POST "$GW/api/orders" -H "$AUTH" -H "$JSONH" -d '{"items":[{"product_id":1,"quantity":2}]}' | json 'd["id"]')
ok "pedido #$OID creado (202, OrderCreated publicado)"
for i in $(seq 1 20); do
  ST=$($C -f "$GW/api/orders" -H "$AUTH" | json "[o['status'] for o in d if o['id']==$OID][0]")
  [ "$ST" = "CONFIRMADO" ] && break; sleep 1
done
[ "$ST" = "CONFIRMADO" ] && ok "pedido #$OID: PENDIENTE → PAGADO → CONFIRMADO" || fail "estado final $ST"
STOCK1=$($C -f "$GW/api/catalog/products" | json '[p["stock"] for p in d if p["id"]==1][0]')
[ "$STOCK1" -eq $((STOCK0 - 2)) ] && ok "stock sincronizado vía StockUpdated ($STOCK0 → $STOCK1)" || fail "stock $STOCK0 → $STOCK1"

OID2=$($C -f -X POST "$GW/api/orders" -H "$AUTH" -H "$JSONH" -d '{"items":[{"product_id":7,"quantity":20}]}' | json 'd["id"]')
for i in $(seq 1 15); do
  ST=$($C -f "$GW/api/orders" -H "$AUTH" | json "[o['status'] for o in d if o['id']==$OID2][0]")
  [ "$ST" = "RECHAZADO" ] && break; sleep 1
done
[ "$ST" = "RECHAZADO" ] && ok "pedido #$OID2 > S/ 5,000 → RECHAZADO (PaymentFailed)" || fail "estado $ST"

echo "▶ TLS interno"
TLSN=$(curl -s http://localhost:8222/varz | json 'd.get("tls_required")')
[ "$TLSN" = "True" ] && ok "NATS exige TLS (tls_required: true)" || fail "falló: NATS exige TLS (tls_required: true)"
SSLC=$(docker compose exec -T postgres psql -U agro -d agromarket -tAc "select count(*) from pg_stat_ssl where ssl")
[ "$SSLC" -ge 1 ] && ok "PostgreSQL: $SSLC conexiones activas cifradas con TLS" || fail "falló: PostgreSQL: $SSLC conexiones activas cifradas con TLS"
docker compose exec -T postgres psql -U agro -d agromarket -tAc "select version, cipher from pg_stat_ssl where ssl limit 1" | sed 's/^/    /'

echo "▶ Notification + Analytics Services"
N=$($C -f "$GW/api/notifications" -H "$AUTH" | json 'len(d)')
[ "$N" -ge 5 ] && ok "$N notificaciones generadas desde eventos" || fail "falló: $N notificaciones generadas desde eventos"
$C -f "$GW/api/analytics" | json 'd["events_total"]' | grep -qv '^0' && ok "métricas agregadas por Analytics" || fail "falló: métricas agregadas por Analytics"

echo "▶ Rate limiting"
CODES=$(for i in $(seq 1 30); do $C -o /dev/null -w '%{http_code} ' -H "$AUTH" "$GW/api/users/me"; done)
echo "$CODES" | grep -q 429 && ok "ráfaga de peticiones → 429 Too Many Requests" || fail "falló: ráfaga de peticiones → 429 Too Many Requests"

echo; echo "✅ Arquitectura segura verificada (TLS + WAF + Keycloak + eventos)"
