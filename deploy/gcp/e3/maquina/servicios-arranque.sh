#!/usr/bin/env bash
# Script de arranque de la VM de servicios (corre como root en cada arranque).
# Idempotente: instala Docker solo si falta y "docker compose up -d" no
# duplica contenedores. El compose llega en el metadato mooc-compose.
set -euo pipefail
log() { echo "servicios-arranque: $*"; }

if ! command -v docker >/dev/null 2>&1; then
    log "instalando Docker"
    apt-get update
    apt-get install -y ca-certificates curl
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
    systemctl enable --now docker
fi

mkdir -p /opt/mooc
curl -fsS -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/attributes/mooc-compose" \
    -o /opt/mooc/docker-compose.yml

cd /opt/mooc
docker compose -p mooc up -d
log "contenedores arriba: $(docker ps --format '{{.Names}}' | tr '\n' ' ')"
