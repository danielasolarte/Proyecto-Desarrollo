<#
Construye la imagen de la API desde el commit actual, la publica en Artifact
Registry con dos etiquetas (el hash corto del commit y latest) y la despliega
en el Web Server.

Motivo: la imagen mooc-e2-api:latest que corre hoy es anterior al commit
6c879a6 (27 sep) que agrego las cookies seguras y CSRF; por eso el login
responde sin cookies aunque AUTH_COOKIE_ENABLED=true.

Uso (desde la raiz del repo, con Docker Desktop encendido):
  . .\deploy\gcp\00-variables.example.ps1
  .\deploy\gcp\08-publicar-imagen-api.ps1 -SoloRevisarMigraciones
  .\deploy\gcp\08-publicar-imagen-api.ps1 -AplicarMigraciones     (si la BD esta atrasada)
  .\deploy\gcp\08-publicar-imagen-api.ps1
#>
param(
    [switch]$SoloRevisarMigraciones,
    [switch]$AplicarMigraciones,
    [string]$Repositorio = "mooc-e2"
)

$ErrorActionPreference = "Continue"
# Permite lanzarlo con el boton Run de VS Code: se ubica en la raiz del repo,
# carga las variables si faltan y deja el registro en deploy/gcp/logs/.
Set-Location (Resolve-Path (Join-Path $PSScriptRoot "..\.."))
if (-not $env:GCP_PROJECT -or -not $env:WEB_VM) { . (Join-Path $PSScriptRoot "00-variables.example.ps1") }
New-Item -ItemType Directory -Force "deploy/gcp/logs" | Out-Null
$log = "deploy/gcp/logs/08-publicar-$(Get-Date -Format 'yyyyMMdd-HHmmss').log"
Start-Transcript -Path $log | Out-Null

$P = $env:GCP_PROJECT; $Z = $env:GCP_ZONE; $R = $env:GCP_REGION
$registro = "$R-docker.pkg.dev"
$imagen = "$registro/$P/$Repositorio/mooc-e2-api"
$commit = (git rev-parse --short HEAD).Trim()

function Titulo($t) { Write-Host ""; Write-Host "==== $t ====" -ForegroundColor Cyan }
function Ssh($cmd) {
    $limpio = $cmd -replace "`r", ""
    gcloud compute ssh $env:WEB_VM --zone=$Z --tunnel-through-iap --project=$P --strict-host-key-checking=no --command=$limpio 2>&1 | ForEach-Object { "$_" }
}

Titulo "Migraciones: version aplicada en Cloud SQL vs repositorio"
$ultima = (Get-ChildItem migrations -Filter "*.up.sql" | Sort-Object Name | Select-Object -Last 1).Name.Substring(0, 6)
Write-Host "Ultima migracion en el repositorio: $([int]$ultima)"
gcloud compute scp deploy/gcp/remoto/version-migraciones.sh deploy/gcp/remoto/aplicar-migraciones.sh "$($env:WEB_VM):/tmp/" --zone=$Z --tunnel-through-iap --project=$P --strict-host-key-checking=no | Out-Null
Ssh "bash /tmp/version-migraciones.sh"

if ($AplicarMigraciones) {
    Titulo "Aplicando migraciones desde el Web Server (dentro de la VPC)"
    Ssh "rm -rf /tmp/mooc-migrations"
    gcloud compute scp --recurse migrations "$($env:WEB_VM):/tmp/mooc-migrations" --zone=$Z --tunnel-through-iap --project=$P --strict-host-key-checking=no | Out-Null
    Ssh "bash /tmp/aplicar-migraciones.sh"
    Stop-Transcript | Out-Null; return
}
if ($SoloRevisarMigraciones) { Stop-Transcript | Out-Null; return }

Titulo "Construyendo $($imagen):$commit"
if (git status --porcelain -- cmd internal go.mod go.sum deploy/Dockerfile.api) {
    Write-Host "ATENCION: hay cambios sin commit en el codigo de la API; la etiqueta $commit no los representaria." -ForegroundColor Yellow
    Stop-Transcript | Out-Null; return
}
gcloud auth configure-docker $registro --quiet 2>&1 | ForEach-Object { "$_" }
docker build --progress=plain -f deploy/Dockerfile.api -t "$($imagen):$commit" -t "$($imagen):latest" . 2>&1 | ForEach-Object { "$_" }
if ($LASTEXITCODE -ne 0) { Write-Host "Fallo docker build (revisa que Docker Desktop este corriendo)." -ForegroundColor Red; Stop-Transcript | Out-Null; return }
docker push "$($imagen):$commit" 2>&1 | ForEach-Object { "$_" }
docker push "$($imagen):latest" 2>&1 | ForEach-Object { "$_" }

Titulo "Desplegando en $($env:WEB_VM)"
$token = gcloud auth print-access-token
$remoto = @"
echo '$token' | sudo docker login -u oauth2accesstoken --password-stdin https://$registro >/dev/null
cd /opt/mooc/deploy
sudo sed -i 's#^API_IMAGE=.*#API_IMAGE=$($imagen):$commit#' .env
sudo docker compose -f docker-compose.web.yml --env-file .env pull api
sudo docker compose -f docker-compose.web.yml --env-file .env up -d --no-deps --force-recreate api
sleep 5
sudo docker compose -f docker-compose.web.yml ps
sudo docker logs --tail 10 `$(sudo docker compose -f docker-compose.web.yml ps -q api)
"@
Ssh $remoto
Write-Host ""
Write-Host "Imagen desplegada: $($imagen):$commit (API_IMAGE fijada a esa etiqueta en el .env del Web Server)." -ForegroundColor Green
Titulo "Verificacion final"
& (Join-Path $PSScriptRoot "06-correcciones-entrega2.ps1") -Fase Verificar -CrearUsuarioPrueba 2>&1 | ForEach-Object { "$_" }
Stop-Transcript | Out-Null
Write-Host "Registro completo en $log"
