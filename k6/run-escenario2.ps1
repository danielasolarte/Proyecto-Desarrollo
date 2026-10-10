# Escenario 2 - carga, procesamiento y consumo multimedia, un nivel por corrida.
# Correccion C4 de la Entrega 2: repetir contra el Web Server real (por HTTPS),
# no contra una API temporal dentro del Worker.
#
# Uso desde la raiz del repo (los videos de prueba deben estar en k6/media):
#   .\k6\run-escenario2.ps1 -BaseUrl https://35.254.78.215.sslip.io/api/v1 -Levels baseline,l1,l2,l3 -Tag ws
#   .\k6\run-escenario2.ps1 -BaseUrl https://35.254.78.215.sslip.io/api/v1 -Levels peak -Tag ws-rep
#
# En otra terminal, durante cada corrida, deja capacity-planning/monitor-queue.ps1
# registrando profundidad y antiguedad de la cola, y docker stats en el Worker.
param(
    [Parameter(Mandatory = $true)]
    [string]$BaseUrl,
    [string[]]$Levels = @("baseline", "l1", "l2", "l3"),
    [int]$PauseBetweenS = 120,
    [string]$Tag = "ws",
    [string]$K6Image = "grafana/k6:latest",
    [string]$AdminEmail = $env:K6_ADMIN_EMAIL,
    [string]$AdminPassword = $env:K6_ADMIN_PASSWORD,
    [int]$SetupReadyTimeoutMs = 600000,
    [int]$ReadyTimeoutMs = 300000
)

$ErrorActionPreference = "Continue"
New-Item -ItemType Directory -Force "k6/results" | Out-Null
foreach ($f in @("k6/media/corto_360p.mp4", "k6/media/medio_720p.mp4", "k6/media/largo_1080p.mp4")) {
    if (-not (Test-Path $f)) { throw "Falta $f. Genera los perfiles con ffmpeg (ver k6/README.md)." }
}

Write-Host "Version de k6 usada:"
docker run --rm $K6Image version

$extraEnv = @()
if ($AdminEmail) { $extraEnv += @("-e", "ADMIN_EMAIL=$AdminEmail") }
if ($AdminPassword) { $extraEnv += @("-e", "ADMIN_PASSWORD=$AdminPassword") }

for ($i = 0; $i -lt $Levels.Count; $i++) {
    $level = $Levels[$i]
    $prefix = "k6/results/escenario2-$Tag-$level"
    Write-Host ""
    Write-Host "=== Escenario 2 | nivel $level | destino $BaseUrl | inicio $(Get-Date -Format s) ==="

    docker run --rm `
        -v "${PWD}:/project" `
        -w /project/k6 `
        $K6Image run `
        -e LEVEL=$level `
        -e BASE_URL=$BaseUrl `
        -e VIDEO_SMALL=./media/corto_360p.mp4 `
        -e VIDEO_MEDIUM=./media/medio_720p.mp4 `
        -e VIDEO_LARGE=./media/largo_1080p.mp4 `
        -e SETUP_READY_TIMEOUT_MS=$SetupReadyTimeoutMs `
        -e READY_TIMEOUT_MS=$ReadyTimeoutMs `
        @extraEnv `
        --summary-export "/project/$prefix.json" `
        --summary-trend-stats "avg,min,med,p(90),p(95),p(99),max" `
        escenario2.js 2>&1 | Tee-Object -FilePath "$prefix.log"

    Write-Host "=== fin nivel $level $(Get-Date -Format s) ==="
    if ($i -lt $Levels.Count - 1) {
        Write-Host "Pausa de $PauseBetweenS s para observar el drenaje de la cola..."
        Start-Sleep -Seconds $PauseBetweenS
    }
}
