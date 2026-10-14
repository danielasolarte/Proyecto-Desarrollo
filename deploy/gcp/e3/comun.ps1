<#
Variables y funciones compartidas por los scripts de la Entrega 3.
No se ejecuta solo: cada script lo carga con ". (Join-Path $PSScriptRoot 'comun.ps1')".

Nada de este archivo es secreto. Los secretos viven en Secret Manager
(ver 01-secretos-identidades.ps1) y las VM los leen al arrancar.
#>

# Raiz del repositorio y variables de la Entrega 2 (red, zona, proyecto).
Set-Location (Resolve-Path (Join-Path $PSScriptRoot "..\..\.."))
if (-not $env:GCP_PROJECT -or -not $env:WEB_VM) { . (Join-Path $PSScriptRoot "..\00-variables.example.ps1") }

$P = $env:GCP_PROJECT
$R = $env:GCP_REGION
$Z = $env:GCP_ZONE
$NET = $env:NETWORK
$SUBNET = $env:SUBNET

# Artifact Registry (el mismo repositorio de la Entrega 2).
$REPO_AR = "mooc-e2"
$REGISTRO = "$R-docker.pkg.dev"
$IMAGEN_CONTENEDOR = @{
    api    = "$REGISTRO/$P/$REPO_AR/mooc-e2-api"
    worker = "$REGISTRO/$P/$REPO_AR/mooc-e2-worker"
}
$DOCKERFILE = @{ api = "deploy/Dockerfile.api"; worker = "deploy/Dockerfile.worker" }

# Cuentas de servicio por rol (permisos minimos, ver 01-secretos-identidades.ps1).
$SA = @{
    api    = "sa-mooc-api@$P.iam.gserviceaccount.com"
    worker = "sa-mooc-worker@$P.iam.gserviceaccount.com"
    servicios = "sa-mooc-servicios@$P.iam.gserviceaccount.com"
}

# Secretos: variable de entorno de la app => nombre del secreto en Secret Manager.
$SECRETOS = [ordered]@{
    DATABASE_URL  = "mooc-database-url"
    S3_ACCESS_KEY = "mooc-s3-access-key"
    S3_SECRET_KEY = "mooc-s3-secret-key"
}

# Etiquetas de red de la Entrega 3.
$TAG = @{ api = "mooc-api"; worker = "mooc-worker"; servicios = "mooc-servicios" }

# Familias de imagenes de maquina (una por rol).
$FAMILIA = @{ api = "mooc-api"; worker = "mooc-worker" }

# Rangos de Google para health checks y balanceador HTTP(S) externo.
$RANGOS_HC = "130.211.0.0/22,35.191.0.0/16"

function Titulo($t) { Write-Host ""; Write-Host "==== $t ====" -ForegroundColor Cyan }
function Ok($t) { Write-Host $t -ForegroundColor Green }
function Aviso($t) { Write-Host $t -ForegroundColor Yellow }
function Falla($t) { Write-Host $t -ForegroundColor Red }

function IniciarRegistro($nombre) {
    New-Item -ItemType Directory -Force "deploy/gcp/logs" | Out-Null
    $script:LOG = "deploy/gcp/logs/e3-$nombre-$(Get-Date -Format 'yyyyMMdd-HHmmss').log"
    Start-Transcript -Path $script:LOG | Out-Null
}
function CerrarRegistro {
    Stop-Transcript | Out-Null
    Write-Host "Registro completo en $script:LOG"
}

function Ssh($vm, $cmd) {
    # Quita CR (CRLF de Windows). Se asigna a variable porque
    # "--command=(expr)" PowerShell lo parte en dos argumentos.
    $limpio = $cmd -replace "`r", ""
    # En Windows gcloud usa PuTTY (plink): la primera vez pregunta si guarda la
    # llave de la VM nueva y sigue igual; esas lineas se filtran.
    gcloud compute ssh $vm --zone=$Z --tunnel-through-iap --project=$P --strict-host-key-checking=no --command=$limpio 2>&1 |
        ForEach-Object { "$_" } |
        Where-Object { $_ -notmatch "host key is not cached|no guarantee that the server|think it is\.|ssh-ed25519 key fingerprint|ssh-ed25519 255 SHA256|If you trust this host|cache and carry on|If you want to carry on|the key to the cache|If you do not trust|abandon the|connection\.$|Store key in cache|compute\.\d+ \(port 22\)|^\s*$" }
}

function Existe($tipo, $nombre, $extra) {
    # $tipo: por ejemplo "compute instance-templates" o "iam service-accounts".
    $argumentos = @($tipo.Split(" ")) + @("describe", $nombre, "--project=$P", "--format=value(name)")
    if ($extra) { $argumentos += $extra }
    # Los "not found" de gcloud llegan como registros de error; se descartan
    # aqui para que no ensucien la consola ni el registro.
    $r = & gcloud @argumentos 2>&1 | Where-Object { $_ -isnot [System.Management.Automation.ErrorRecord] }
    return [bool]$r
}

function Commit {
    (git rev-parse --short HEAD).Trim()
}

function ArchivoLF($ruta) {
    # Copia temporal con finales LF (bash en la VM no tolera CRLF).
    $tmp = [System.IO.Path]::GetTempFileName()
    $texto = [System.IO.File]::ReadAllText((Resolve-Path $ruta)) -replace "`r`n", "`n"
    [System.IO.File]::WriteAllText($tmp, $texto)
    return $tmp
}

function TextoLF($texto) {
    $tmp = [System.IO.Path]::GetTempFileName()
    [System.IO.File]::WriteAllText($tmp, ($texto -replace "`r`n", "`n"))
    return $tmp
}
