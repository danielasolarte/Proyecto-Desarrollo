#!/usr/bin/env bash
# Se ejecuta en el Web Server, dentro de la VPC, porque Cloud SQL ya no tiene
# IP publica. Aplica las migraciones copiadas en /tmp/mooc-migrations con
# golang-migrate y deja constancia de la version final.
set -euo pipefail
cd /opt/mooc/deploy
sudo docker run --rm --network host --env-file .env -v /tmp/mooc-migrations:/migrations \
  --entrypoint sh migrate/migrate:v4.17.1 \
  -c 'migrate -path=/migrations -database "$DATABASE_URL" up && migrate -path=/migrations -database "$DATABASE_URL" version'
