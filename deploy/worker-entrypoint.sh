#!/bin/sh
# Arranque del worker (Entrega 4, decisión 2: clamd dentro del contenedor).
#
# Cloud Run solo enruta HTTP, así que clamd (TCP 3310) no puede ser un
# servicio aparte: corre en el mismo contenedor y el worker le habla por
# 127.0.0.1. En docker-compose, CLAMAV_HOST=clamav apunta al contenedor
# clamav y aquí no se inicia clamd.
#
# El worker abre el puerto solo cuando clamd ya responde, así la prueba de
# inicio de Cloud Run no da tráfico a una instancia que todavía carga las
# firmas (tarda unos 20-40 s).
set -eu

CLAMAV_HOST="${CLAMAV_HOST:-127.0.0.1}"
CLAMAV_PORT="${CLAMAV_PORT:-3310}"
CLAMD_ESPERA_S="${CLAMD_ESPERA_S:-180}"

# clamd y clamdscan leen el mismo archivo. Los límites cubren videos de
# prueba grandes: con los valores por defecto (25 MB) clamd rechaza el
# archivo en vez de escanearlo.
cat > /etc/clamav/clamd.conf <<EOF
TCPSocket ${CLAMAV_PORT}
TCPAddr ${CLAMAV_HOST}
Foreground yes
DatabaseDirectory /var/lib/clamav
TemporaryDirectory /tmp
LogTime yes
StreamMaxLength 1000M
MaxFileSize 1000M
MaxScanSize 1000M
EOF

case "${CLAMAV_HOST}" in
  127.0.0.1 | localhost)
    echo '{"severity":"INFO","message":"iniciando clamd local"}'
    clamd &
    ;;
esac

i=0
until clamdscan --ping=1 >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "${i}" -ge "${CLAMD_ESPERA_S}" ]; then
    echo "{\"severity\":\"ERROR\",\"message\":\"clamd no respondió en ${CLAMD_ESPERA_S} s\"}"
    exit 1
  fi
  sleep 1
done
echo "{\"severity\":\"INFO\",\"message\":\"clamd listo tras ${i} s\"}"

exec /app/worker
