#!/usr/bin/env bash
# Se ejecuta en el Web Server. Promueve a admin una cuenta de PRUEBA ya
# registrada y verificada (solo correos @mooc.test), para que k6 pueda crear
# profesores en setup() sin usar ni modificar cuentas reales del equipo.
set -euo pipefail
EMAIL="$1"
case "$EMAIL" in *@mooc.test) ;; *) echo "Solo se permiten cuentas @mooc.test"; exit 1;; esac
cd /opt/mooc/deploy
sudo docker run --rm -i --network host --env-file .env -e EMAIL="$EMAIL" postgres:16 \
  sh -c 'psql "$DATABASE_URL" -X -q -v ON_ERROR_STOP=1 -v email="$EMAIL" -At' <<'SQL'
UPDATE users SET role = 'admin', updated_at = now()
WHERE email = :'email' AND status = 'active'
RETURNING email, role;
SQL
