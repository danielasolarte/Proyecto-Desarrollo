$ErrorActionPreference = "Stop"

if (-not $env:WORKER_VM) {
    throw "Load deploy/gcp/00-variables.example.ps1 first and adjust values."
}

gcloud compute scp deploy/docker-compose.worker.yml "${env:WORKER_VM}:/tmp/docker-compose.worker.yml" --zone=$env:GCP_ZONE --tunnel-through-iap
gcloud compute scp deploy/worker.env.example "${env:WORKER_VM}:/tmp/worker.env.example" --zone=$env:GCP_ZONE --tunnel-through-iap

# Token de acceso de quien ejecuta este script. Se usa para autenticar el
# pull de Artifact Registry sin depender de los permisos de la cuenta de
# servicio adjunta a la VM: en la practica esa cuenta no siempre trae el
# scope/rol necesario y "docker pull" falla con "Unauthenticated request"
# aunque "gcloud auth configure-docker" se haya corrido en la VM.
$accessToken = gcloud auth print-access-token
$registryHost = "$($env:GCP_REGION)-docker.pkg.dev"

$remote = @"
sudo mkdir -p /opt/mooc/deploy
sudo mv /tmp/docker-compose.worker.yml /opt/mooc/deploy/docker-compose.worker.yml
sudo mv /tmp/worker.env.example /opt/mooc/deploy/.env.example
cd /opt/mooc/deploy
if [ ! -f .env ]; then sudo cp .env.example .env; fi
sudo chmod 600 .env
echo '$accessToken' | sudo docker login -u oauth2accesstoken --password-stdin https://$registryHost
sudo docker compose -f docker-compose.worker.yml --env-file .env pull
sudo docker compose -f docker-compose.worker.yml --env-file .env up -d
sudo docker compose -f docker-compose.worker.yml ps
"@

gcloud compute ssh $env:WORKER_VM --zone=$env:GCP_ZONE --tunnel-through-iap --command=$remote

Write-Host "Worker compose (worker + redis + clamav) instalado y levantado."
Write-Host "El token usado para el login de Docker expira en poco tiempo: si necesitas volver a hacer 'docker pull' mas tarde (por ejemplo tras publicar una imagen nueva), vuelve a correr este script para renovarlo."
Write-Host "Edita /opt/mooc/deploy/.env en la VM con los secretos reales (DATABASE_URL, S3_*, etc.) si aun tiene los valores de ejemplo, y reinicia con: sudo docker compose -f docker-compose.worker.yml restart"
