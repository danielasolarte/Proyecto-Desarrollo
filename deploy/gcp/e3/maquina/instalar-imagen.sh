#!/usr/bin/env bash
# Script de arranque de la VM TEMPORAL que construye la imagen de maquina de
# un rol (api o worker). Lo lanza 02-imagen-maquina.ps1; no corre en las VM
# de los grupos. Al final escribe MOOC_IMAGEN_LISTA en la consola serial y el
# script de Windows apaga la VM y crea la imagen a partir de su disco.
#
# Deja instalado: Docker Engine, la imagen del contenedor ya descargada,
# /usr/local/bin/mooc-preparar.sh y mooc.service habilitado. No deja
# secretos ni credenciales: el login a Artifact Registry se borra.
set -euo pipefail
trap 'echo "MOOC_IMAGEN_ERROR linea $LINENO" > /dev/ttyS0' ERR

MD=http://metadata.google.internal/computeMetadata/v1
H='Metadata-Flavor: Google'
attr() { curl -sf -H "$H" "$MD/instance/attributes/$1"; }

# Evita repetir la instalacion si la VM se reinicia a mitad de camino.
if [ -f /var/lib/mooc/imagen-lista ]; then echo MOOC_IMAGEN_LISTA > /dev/ttyS0; exit 0; fi

IMAGEN=$(attr mooc-imagen)
export DEBIAN_FRONTEND=noninteractive

apt-get update -q
apt-get install -y -q ca-certificates curl gnupg python3
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list
apt-get update -q
apt-get install -y -q docker-ce docker-ce-cli containerd.io
systemctl enable docker
systemctl start docker

# Archivos de arranque de los grupos (vienen como metadatos de esta VM).
attr mooc-preparar-sh | tr -d '\r' > /usr/local/bin/mooc-preparar.sh
chmod 755 /usr/local/bin/mooc-preparar.sh
attr mooc-service | tr -d '\r' > /etc/systemd/system/mooc.service
install -d /etc/mooc
echo "$IMAGEN" > /etc/mooc/imagen

# Descarga previa del contenedor (el motivo de la imagen de maquina: las VM
# nuevas no esperan este paso).
TOKEN=$(curl -sf -H "$H" "$MD/instance/service-accounts/default/token" | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')
REG=${IMAGEN%%/*}
echo "$TOKEN" | docker login -u oauth2accesstoken --password-stdin "https://$REG" >/dev/null
docker pull -q "$IMAGEN"
docker logout "https://$REG" >/dev/null || true
rm -rf /root/.docker

systemctl daemon-reload
systemctl enable mooc.service   # se inicia en las VM de los grupos, no aqui

# Limpieza para que la imagen sea pequena y no arrastre estado de esta VM.
apt-get clean
rm -rf /var/lib/apt/lists/*
journalctl --rotate >/dev/null 2>&1 || true
journalctl --vacuum-time=1s >/dev/null 2>&1 || true

install -d /var/lib/mooc
date -u +%FT%TZ > /var/lib/mooc/imagen-lista
echo "$IMAGEN" >> /var/lib/mooc/imagen-lista
sync
echo "MOOC_IMAGEN_LISTA $IMAGEN" > /dev/ttyS0
