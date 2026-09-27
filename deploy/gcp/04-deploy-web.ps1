param(
    [Parameter(Mandatory = $true)]
    [string]$Domain,

    [Parameter(Mandatory = $true)]
    [string]$AcmeEmail
)

$ErrorActionPreference = "Stop"

if (-not $env:WEB_VM) {
    throw "Load deploy/gcp/00-variables.example.ps1 first and adjust values."
}

gcloud compute scp deploy/docker-compose.web.yml "${env:WEB_VM}:/tmp/docker-compose.web.yml" --zone=$env:GCP_ZONE --tunnel-through-iap
gcloud compute scp deploy/web.env.example "${env:WEB_VM}:/tmp/web.env.example" --zone=$env:GCP_ZONE --tunnel-through-iap
gcloud compute scp deploy/caddy/Caddyfile.web "${env:WEB_VM}:/tmp/Caddyfile.web" --zone=$env:GCP_ZONE --tunnel-through-iap

$remote = @"
sudo mkdir -p /opt/mooc/deploy /opt/mooc/caddy
sudo mv /tmp/docker-compose.web.yml /opt/mooc/deploy/docker-compose.web.yml
sudo mv /tmp/web.env.example /opt/mooc/deploy/.env.example
sudo mv /tmp/Caddyfile.web /opt/mooc/caddy/Caddyfile
cd /opt/mooc/deploy
if [ ! -f .env ]; then sudo cp .env.example .env; fi
sudo chmod 600 .env
sudo sed -i 's|__MOOC_DOMAIN__|$Domain|g; s|__ACME_EMAIL__|$AcmeEmail|g' /opt/mooc/caddy/Caddyfile
sudo cp /opt/mooc/caddy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy
sudo docker compose -f docker-compose.web.yml --env-file .env up -d
"@

gcloud compute ssh $env:WEB_VM --zone=$env:GCP_ZONE --tunnel-through-iap --command=$remote

Write-Host "Web compose and Caddy installed. Edit /opt/mooc/deploy/.env on the VM with real secrets before final restart."
