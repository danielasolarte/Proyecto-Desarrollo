#!/usr/bin/env bash
# Se ejecuta en cada arranque de una VM de los grupos (API o workers), antes de
# iniciar el contenedor (ExecStartPre de mooc.service). Queda dentro de la
# imagen de maquina; la configuracion llega por metadatos de la plantilla:
#
#   mooc-imagen     imagen del contenedor con etiqueta de commit
#   mooc-config     lineas CLAVE=valor NO sensibles (REDIS_ADDR, PORT, ...)
#   mooc-secretos   lineas VARIABLE=nombre-del-secreto en Secret Manager
#   mooc-detener-s  segundos que se le dan al contenedor para apagarse
#   mooc-docker-args argumentos extra de docker run (opcional), por ejemplo
#                   --add-host=clamav:10.20.0.5 para el worker
#
# Escribe /run/mooc/env (tmpfs: no queda en disco) con permisos 600 y
# /run/mooc/unidad.env con la imagen y el tiempo de apagado. Si algo falla,
# sale con error y systemd reintenta (Restart=always).
set -euo pipefail

MD=${MOOC_MD:-http://metadata.google.internal/computeMetadata/v1}   # MOOC_MD/MOOC_SM solo para pruebas locales
H='Metadata-Flavor: Google'
md() { curl -sf -H "$H" "$MD/$1"; }
attr() { curl -sf -H "$H" "$MD/instance/attributes/$1" || true; }
log() { echo "mooc-preparar: $*" | tee /dev/ttyS0 >&2 || true; }

INICIO=$(date +%s)
PROYECTO=$(md project/project-id)
NOMBRE=$(md instance/name)
TOKEN=$(md instance/service-accounts/default/token | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')

IMAGEN=$(attr mooc-imagen)
if [ -z "$IMAGEN" ] && [ -f /etc/mooc/imagen ]; then IMAGEN=$(cat /etc/mooc/imagen); fi
if [ -z "$IMAGEN" ]; then log "falta el metadato mooc-imagen"; exit 1; fi
DETENER=$(attr mooc-detener-s); DETENER=${DETENER:-45}
EXTRA=$(attr mooc-docker-args | tr -d '\r\n')
if [ -n "$EXTRA" ] && ! [[ "$EXTRA" =~ ^[A-Za-z0-9=:._/,\ -]+$ ]]; then
  log "mooc-docker-args tiene caracteres no permitidos"; exit 1
fi

install -d -m 700 /run/mooc
TMP=$(mktemp /run/mooc/env.XXXXXX)
chmod 600 "$TMP"

# Configuracion no sensible.
attr mooc-config | tr -d '\r' | grep -E '^[A-Z_][A-Z0-9_]*=' >> "$TMP" || true
echo "INSTANCE_ID=$NOMBRE" >> "$TMP"

# Secretos: se leen con la identidad de la VM (cuenta de servicio del rol).
while IFS='=' read -r VAR SECRETO; do
  [ -z "${VAR:-}" ] && continue
  VALOR=$(curl -sf -H "Authorization: Bearer $TOKEN" \
    "${MOOC_SM:-https://secretmanager.googleapis.com}/v1/projects/$PROYECTO/secrets/$SECRETO/versions/latest:access" \
    | python3 -c 'import sys,json,base64; print(base64.b64decode(json.load(sys.stdin)["payload"]["data"]).decode())') \
    || { log "no se pudo leer el secreto $SECRETO (revisar permisos de la cuenta de servicio)"; rm -f "$TMP"; exit 1; }
  printf '%s=%s\n' "$VAR" "$VALOR" >> "$TMP"
done < <(attr mooc-secretos | tr -d '\r' | grep -E '^[A-Z_][A-Z0-9_]*=[a-z0-9-]+$' || true)

mv -f "$TMP" /run/mooc/env
printf 'MOOC_IMAGEN=%s\nMOOC_DETENER_S=%s\nMOOC_DOCKER_ARGS="%s"\n' "$IMAGEN" "$DETENER" "$EXTRA" > /run/mooc/unidad.env

# La imagen ya viene descargada en la imagen de maquina; solo se descarga si
# la plantilla pide otra etiqueta. El login usa el token de la VM y se borra.
if ! docker image inspect "$IMAGEN" >/dev/null 2>&1; then
  REG=${IMAGEN%%/*}
  echo "$TOKEN" | docker login -u oauth2accesstoken --password-stdin "https://$REG" >/dev/null
  docker pull -q "$IMAGEN" >/dev/null
  docker logout "https://$REG" >/dev/null || true
  log "imagen descargada en el arranque: $IMAGEN"
fi

log "configuracion lista en $(( $(date +%s) - INICIO )) s para $IMAGEN (instancia $NOMBRE)"
