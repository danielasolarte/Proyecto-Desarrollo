$ErrorActionPreference = "Stop"

if (-not $env:GCP_PROJECT) {
    throw "Load deploy/gcp/00-variables.example.ps1 first and adjust values."
}

$startup = @'
#!/usr/bin/env bash
set -euo pipefail

apt-get update
apt-get install -y ca-certificates curl gnupg

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin caddy
systemctl enable --now docker
systemctl enable --now caddy

mkdir -p /opt/mooc/deploy /opt/mooc/caddy
chmod 750 /opt/mooc
'@

gcloud compute instances create $env:WEB_VM `
    --zone=$env:GCP_ZONE `
    --machine-type=$env:MACHINE_TYPE `
    --subnet=$env:SUBNET `
    --tags=$env:WEB_TAG `
    --address=$env:WEB_STATIC_IP `
    --boot-disk-size=$env:BOOT_DISK_SIZE `
    --image-family=$env:IMAGE_FAMILY `
    --image-project=$env:IMAGE_PROJECT `
    --metadata=startup-script=$startup

Write-Host "Web Server VM created. Copy deploy/docker-compose.web.yml, deploy/web.env.example and deploy/caddy/Caddyfile.web to /opt/mooc."
