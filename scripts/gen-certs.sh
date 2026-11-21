#!/bin/sh
# Genera una CA interna y certificados TLS para cada componente (se ejecuta en el
# contenedor "certs" al levantar el proyecto; los guarda en el volumen /certs).
#   gateway  → HTTPS del API Gateway / WAF (cliente ↔ NGINX)
#   nats     → TLS del Event Bus (servicios ↔ NATS)
#   postgres → TLS de la base de datos (servicios ↔ PostgreSQL)
set -eu
cd /certs

if [ -f ca.crt ] && [ -f gateway.crt ] && [ -f nats.crt ] && [ -f postgres.crt ]; then
  echo "certificados ya existen, nada que hacer"
  exit 0
fi

apk add --no-cache openssl >/dev/null

# 1) Autoridad certificadora (CA) interna de AgroMarket
openssl req -x509 -newkey rsa:2048 -nodes -days 825 \
  -keyout ca.key -out ca.crt -subj "/O=AgroMarket/CN=AgroMarket Internal CA"

# 2) Un certificado por componente, firmado por la CA, con su nombre DNS en el SAN
issue() {
  name=$1; sans=$2
  openssl req -newkey rsa:2048 -nodes -keyout "$name.key" -out "$name.csr" -subj "/O=AgroMarket/CN=$name"
  printf "subjectAltName=%s\nextendedKeyUsage=serverAuth\nkeyUsage=digitalSignature,keyEncipherment\n" "$sans" > "$name.ext"
  openssl x509 -req -in "$name.csr" -CA ca.crt -CAkey ca.key -CAcreateserial -days 825 -sha256 \
    -extfile "$name.ext" -out "$name.crt"
  rm -f "$name.csr" "$name.ext"
}
issue gateway  "DNS:localhost,DNS:gateway,IP:127.0.0.1"
issue nats     "DNS:nats,DNS:localhost"
issue postgres "DNS:postgres,DNS:localhost"

# Permisos: PostgreSQL exige que su llave sea del usuario postgres (uid 70) y 0600
chmod 644 ./*.crt gateway.key nats.key
chmod 600 ca.key
chown 70:70 postgres.key postgres.crt && chmod 600 postgres.key

echo "✔ CA y certificados TLS generados:"
ls -l /certs
