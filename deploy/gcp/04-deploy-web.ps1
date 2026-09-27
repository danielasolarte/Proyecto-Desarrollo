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

# Token de acceso de quien ejecuta este script. Se usa para autenticar el
# pull de Artifact Registry sin depender de los permisos de la cuenta de
# servicio adjunta a la VM: en la practica esa cuenta no siempre trae el
# scope/rol necesario y "docker pull" falla con "Unauthenticated request"
# aunque "gcloud auth configure-docker" se haya corrido en la VM.
$accessToken = gcloud auth print-access-token
$registryHost = "$($env:GCP_REGION)-docker.pkg.dev"

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
echo '$accessToken' | sudo docker login -u oauth2accesstoken --password-stdin https://$registryHost
sudo docker compose -f docker-compose.web.yml --env-file .env pull
sudo docker compose -f docker-compose.web.yml --env-file .env up -d
sudo docker compose -f docker-compose.web.yml ps
"@

gcloud compute ssh $env:WEB_VM --zone=$env:GCP_ZONE --tunnel-through-iap --command=$remote

Write-Host "Web compose y Caddy instalados y levantados."
Write-Host "El token usado para el login de Docker expira en poco tiempo: si necesitas volver a hacer 'docker pull' mas tarde (por ejemplo tras publicar una imagen nueva), vuelve a correr este script para renovarlo."
Write-Host "Edita /opt/mooc/deploy/.env en la VM con los secretos reales antes del reinicio final si aun tiene los valores de ejemplo."
