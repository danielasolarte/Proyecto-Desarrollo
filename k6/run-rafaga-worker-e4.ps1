<#
Entrega 4 - Rafaga de cargas para el escalamiento del worker.

Corre k6/rafaga-worker-e4.js con Docker contra el backend de Cloud Run y
guarda el resultado en k6/results/e4/. Mientras corre, abre en Monitoring
las graficas de la seccion 4 de docs/entrega4/evidencias.md (mensajes
pendientes, edad del mas antiguo e instancias del worker).

Antes de correr:
  - Cloud SQL encendida y el worker desplegado.
  - Docker Desktop abierto.
  - La clave del profesor en una variable de entorno (no va en el repo):
      $env:TEACHER_PASSWORD = "<clave del profesor>"

Uso:
  .\k6\run-rafaga-worker-e4.ps1                                  (9 presentaciones)
  .\k6\run-rafaga-worker-e4.ps1 -Tipo video -Rafaga 6
  .\k6\run-rafaga-worker-e4.ps1 -Tipo presentacion -Rafaga 1     (mide c: una sola carga)
#>
param(
    [ValidateSet("presentacion", "video")]
    [string]$Tipo = "presentacion",
    [int]$Rafaga = 9,
    [string]$TeacherEmail = "profesor.prueba@mooc.local",
    [string]$BaseUrl,
    [int]$ReadyTimeoutMin = 15,
    [string]$K6Image = "grafana/k6:0.57.0"
)

$ErrorActionPreference = "Continue"
Set-Location (Resolve-Path (Join-Path $PSScriptRoot ".."))

if (-not $env:TEACHER_PASSWORD) {
    Write-Host 'Falta la clave del profesor. Corre primero:  $env:TEACHER_PASSWORD = "<clave>"' -ForegroundColor Red
    return
}
if (-not $BaseUrl) {
    $backend = terraform "-chdir=infra/terraform" output -raw url_backend 2>$null
    if (-not $backend) { Write-Host "No se pudo leer url_backend de Terraform; pasa -BaseUrl https://.../api/v1" -ForegroundColor Red; return }
    $BaseUrl = "$($backend.Trim())/api/v1"
}

docker version --format "Docker {{.Server.Version}}" 2>$null
if ($LASTEXITCODE -ne 0) { Write-Host "Docker Desktop no esta corriendo." -ForegroundColor Red; return }

# El video corto de prueba (15 s, 360p) se genera con ffmpeg si no existe.
if ($Tipo -eq "video" -and -not (Test-Path "k6/media/corto_360p_15s.mp4")) {
    New-Item -ItemType Directory -Force "k6/media" | Out-Null
    Write-Host "Generando k6/media/corto_360p_15s.mp4 con ffmpeg (Docker)..."
    docker run --rm -v "${PWD}/k6/media:/out" jrottenberg/ffmpeg:6.1-alpine -hide_banner -loglevel error `
        -f lavfi -i "testsrc2=size=640x360:rate=30" -f lavfi -i "sine=frequency=440:sample_rate=44100" `
        -t 15 -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -b:a 128k -shortest /out/corto_360p_15s.mp4
    if (-not (Test-Path "k6/media/corto_360p_15s.mp4")) { Write-Host "No se pudo generar el video." -ForegroundColor Red; return }
}

New-Item -ItemType Directory -Force "k6/results/e4" | Out-Null
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$prefix = "k6/results/e4/rafaga-$Tipo-$Rafaga-$stamp"

Write-Host ""
Write-Host "=== Rafaga: $Rafaga cargas de $Tipo contra $BaseUrl ===" -ForegroundColor Cyan
Write-Host "Inicio (UTC): $((Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')). Anota esta hora para las graficas de Monitoring."

# TEACHER_PASSWORD pasa del entorno al contenedor sin aparecer en el comando.
docker run --rm `
    -v "${PWD}:/project" -w /project/k6 `
    -e BASE_URL=$BaseUrl -e TEACHER_EMAIL=$TeacherEmail -e TEACHER_PASSWORD `
    -e TIPO=$Tipo -e RAFAGA=$Rafaga -e READY_TIMEOUT_MS=$($ReadyTimeoutMin * 60000) `
    $K6Image run --summary-export "/project/$prefix.json" rafaga-worker-e4.js 2>&1 |
    Tee-Object -FilePath "$prefix.log"

# k6 copia en el resumen lo que devuelve setup(), incluido el token de sesion
# del profesor. Se quita antes de que el archivo pueda llegar al repositorio.
if (Test-Path "$prefix.json") {
    $resumen = Get-Content "$prefix.json" -Raw | ConvertFrom-Json
    $resumen.PSObject.Properties.Remove("setup_data")
    $resumen | ConvertTo-Json -Depth 32 | Set-Content "$prefix.json" -Encoding utf8
}

Write-Host ""
Write-Host "Fin (UTC): $((Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ'))"
Write-Host "Resultados: $prefix.log y $prefix.json" -ForegroundColor Green
Write-Host "En el resumen: upload_request_ms (p95 < 2 s), time_to_ready_ms (p95) y ready_timeouts (debe ser 0)."
