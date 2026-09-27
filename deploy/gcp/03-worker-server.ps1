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
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker

mkdir -p /opt/mooc/deploy
chmod 750 /opt/mooc
'@

gcloud compute instances create $env:WORKER_VM `
    --zone=$env:GCP_ZONE `
    --machine-type=$env:MACHINE_TYPE `
    --subnet=$env:SUBNET `
    --tags=$env:WORKER_TAG `
    --no-address `
    --boot-disk-size=$env:BOOT_DISK_SIZE `
    --image-family=$env:IMAGE_FAMILY `
    --image-project=$env:IMAGE_PROJECT `
    --metadata=startup-script=$startup

Write-Host "Worker Server VM created without public IP. Copy deploy/docker-compose.worker.yml and deploy/worker.env.example to /opt/mooc."
