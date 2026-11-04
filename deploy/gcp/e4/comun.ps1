<#
Variables y funciones compartidas por los scripts de la Entrega 4.
No se ejecuta solo: cada script lo carga con ". (Join-Path $PSScriptRoot 'comun.ps1')".
Nada de este archivo es secreto.
#>

# Raiz del repositorio.
Set-Location (Resolve-Path (Join-Path $PSScriptRoot "..\..\.."))

$P = if ($env:GCP_PROJECT) { $env:GCP_PROJECT } else { "proyecto1-entrega2-desarrollo" }
$R = if ($env:GCP_REGION) { $env:GCP_REGION } else { "us-central1" }

$BUCKET_ESTADO = "mooc-tfstate-$P"
$REPO_AR = "mooc-e2"
$REGISTRO = "$R-docker.pkg.dev"
$TF_DIR = "infra/terraform"

# Imagen y Dockerfile de cada pieza de la E4.
$IMAGENES = [ordered]@{
    backend     = @{ nombre = "mooc-e4-backend";     dockerfile = "deploy/Dockerfile.api" }
    worker      = @{ nombre = "mooc-e4-worker";      dockerfile = "deploy/Dockerfile.worker" }
    frontend    = @{ nombre = "mooc-e4-frontend";    dockerfile = "deploy/Dockerfile.frontend" }
    migraciones = @{ nombre = "mooc-e4-migraciones"; dockerfile = "deploy/Dockerfile.migraciones" }
}

function Titulo($t) { Write-Host ""; Write-Host "==== $t ====" -ForegroundColor Cyan }
function Ok($t) { Write-Host $t -ForegroundColor Green }
function Aviso($t) { Write-Host $t -ForegroundColor Yellow }
function Falla($t) { Write-Host $t -ForegroundColor Red }

function IniciarRegistro($nombre) {
    New-Item -ItemType Directory -Force "deploy/gcp/logs" | Out-Null
    $script:LOG = "deploy/gcp/logs/e4-$nombre-$(Get-Date -Format 'yyyyMMdd-HHmmss').log"
    Start-Transcript -Path $script:LOG | Out-Null
}
function CerrarRegistro {
    Stop-Transcript | Out-Null
    Write-Host "Registro completo en $script:LOG"
}

function Commit {
    (git rev-parse --short HEAD).Trim()
}

function ArbolLimpio {
    # Una imagen debe corresponder exactamente a un commit.
    $cambios = git status --porcelain --untracked-files=no
    return -not $cambios
}
