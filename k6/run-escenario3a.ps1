# Escenario 3A - carga en rampa contra el balanceador (k6 nativo en Windows).
# Uso desde la raiz del repo (arrancar ANTES el monitor en otra terminal):
#   .\k6\run-escenario3a.ps1 -BaseUrl https://<dominio>/api/v1 -StudentEmail <correo> -StudentPassword <clave>
# Perfil por defecto: 5 req/s 2 min, sube a 30 req/s en 3 min, 12 min en 30 req/s, baja en 1 min.
param(
    [Parameter(Mandatory = $true)][string]$BaseUrl,
    [string]$StudentEmail,
    [string]$StudentPassword,
    [int]$BaseRate = 5,
    [int]$PeakRate = 30,
    [int]$HoldM = 12,
    [double]$LoginRatio = 0.1,
    [string]$Tag = (Get-Date -Format "yyyyMMdd-HHmmss")
)

$ErrorActionPreference = "Continue"
New-Item -ItemType Directory -Force "k6/results" | Out-Null
$prefix = "k6/results/e3a-$Tag"

Write-Host "Version de k6 usada:"
k6 version
Write-Host ""
Write-Host "=== Escenario 3A | $BaseRate -> $PeakRate req/s, meseta $HoldM min, logins $([math]::Round($LoginRatio*100)) % | $BaseUrl ==="
Write-Host ("Inicio UTC: {0:yyyy-MM-ddTHH:mm:ssZ}" -f (Get-Date).ToUniversalTime())

$argsK6 = @(
    "-e", "BASE_URL=$BaseUrl", "-e", "BASE_RATE=$BaseRate", "-e", "PEAK_RATE=$PeakRate",
    "-e", "HOLD_M=$HoldM", "-e", "LOGIN_RATIO=$LoginRatio")
if ($StudentEmail) { $argsK6 += @("-e", "STUDENT_EMAIL=$StudentEmail", "-e", "STUDENT_PASSWORD=$StudentPassword") }

k6 run @argsK6 --summary-export "$prefix.json" k6/escenario3a.js 2>&1 | Tee-Object -FilePath "$prefix.log"

Write-Host ("Fin UTC: {0:yyyy-MM-ddTHH:mm:ssZ}" -f (Get-Date).ToUniversalTime())
Write-Host "Resultados en $prefix.json y .log. Deja el monitor corriendo para ver la reduccion del grupo."
