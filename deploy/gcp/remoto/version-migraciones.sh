#!/usr/bin/env bash
# Se ejecuta en el Web Server. Imprime la version de migraciones aplicada en
# Cloud SQL (tabla schema_migrations de golang-migrate) sin mostrar secretos.
set -euo pipefail
cd /opt/mooc/deploy
sudo docker run --rm --network host --env-file .env postgres:16 \
  sh -c 'psql "$DATABASE_URL" -X -q -At -F " dirty=" -c "SELECT version, dirty FROM schema_migrations"'
