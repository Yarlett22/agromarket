#!/bin/sh
# Obliga a que toda conexión TCP a PostgreSQL use TLS (hostssl en lugar de host).
set -e
sed -i 's/^host all all all/hostssl all all all/' "$PGDATA/pg_hba.conf"
echo "pg_hba.conf: solo se aceptan conexiones TLS (hostssl)"
