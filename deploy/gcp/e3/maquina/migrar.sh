#!/usr/bin/env bash
# Migraciones controladas (Entrega 3). Corre UNA vez, en la VM de servicios
# (dentro de la VPC, porque Cloud SQL solo tiene IP privada), nunca en el
# arranque de las replicas. Lo lanza 05-migraciones.ps1:
#
#   sudo bash /tmp/mooc-migrar.sh version     version aplicada en Cloud SQL
#   sudo bash /tmp/mooc-migrar.sh up          aplica las pendientes
#
# DATABASE_URL se lee de Secret Manager con la identidad de la VM y solo vive
# en una variable de este proceso: no se escribe en disco ni aparece en la
# linea de comandos de docker (se pasa con "-e DATABASE_URL" sin valor).
# golang-migrate toma un bloqueo en Postgres, asi que dos ejecuciones
# simultaneas no aplican la misma migracion dos veces.
set -euo pipefail

ACCION=${1:-version}
DIR=${MOOC_MIGRACIONES_DIR:-/tmp/mooc-migrations}
SECRETO=${MOOC_SECRETO_DB:-mooc-database-url}
MIGRATE_IMG=migrate/migrate:v4.17.1
MD=${MOOC_MD:-http://metadata.google.internal/computeMetadata/v1}   # MOOC_MD/MOOC_SM solo para pruebas locales
SM=${MOOC_SM:-https://secretmanager.googleapis.com}
H='Metadata-Flavor: Google'

case "$ACCION" in version|up) ;; *) echo "accion invalida: $ACCION (version | up)"; exit 2;; esac

PROYECTO=$(curl -sf -H "$H" "$MD/project/project-id")
TOKEN=$(curl -sf -H "$H" "$MD/instance/service-accounts/default/token" | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')
DATABASE_URL=$(curl -sf -H "Authorization: Bearer $TOKEN" "$SM/v1/projects/$PROYECTO/secrets/$SECRETO/versions/latest:access" \
  | python3 -c 'import sys,json,base64; print(base64.b64decode(json.load(sys.stdin)["payload"]["data"]).decode())' 2>/dev/null) \
  || { echo "ERROR: no se pudo leer el secreto $SECRETO (la cuenta de servicio de esta VM necesita secretAccessor sobre el)"; exit 1; }
export DATABASE_URL

# Host de la base, sin usuario ni clave, solo para dejar constancia.
echo "base de datos: $(echo "$DATABASE_URL" | sed -E 's#^[a-z]+://[^@]*@([^:/?]*).*#\1#')"

# La URL se expande DENTRO del contenedor (comillas simples): en la linea de
# comandos de docker solo aparece el texto literal $DATABASE_URL.
migrar() {
  docker run --rm --network host -e DATABASE_URL -v "$DIR":/migrations:ro \
    --entrypoint sh "$MIGRATE_IMG" -c 'migrate -path=/migrations -database "$DATABASE_URL" "$@"' migrate "$@"
}

if [ "$ACCION" = "up" ]; then
  [ -d "$DIR" ] || { echo "ERROR: no existe $DIR (el script de Windows copia ahi la carpeta migrations)"; exit 1; }
  echo "migraciones en el repositorio: $(ls "$DIR"/*.up.sql | wc -l)"
  echo "antes:"; migrar version 2>&1 | sed 's/^error: no migration$/sin migraciones aplicadas (base vacia)/' || true
  migrar up 2>&1
  echo "despues:"
fi
migrar version 2>&1 | sed 's/^error: no migration$/sin migraciones aplicadas (base vacia)/'
