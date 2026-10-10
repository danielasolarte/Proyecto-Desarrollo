<#
Estado del Worker sin SSH (sirve cuando la VM esta saturada): estado de la
instancia, ultimas lineas de la consola serial y salud/login de la API.
Se puede lanzar con Terminal > Run Active File. Registro en deploy/gcp/logs/.
#>
$ErrorActionPreference = "Continue"
Set-Location (Resolve-Path (Join-Path $PSScriptRoot "..\.."))
. .\deploy\gcp\00-variables.example.ps1
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
Start-Transcript -Path "deploy/gcp/logs/11-estado-$stamp.log" | Out-Null
$a = @("--zone=$env:GCP_ZONE", "--project=$env:GCP_PROJECT")
gcloud compute instances describe $env:WORKER_VM @a --format="table(name,status)" 2>&1 | ForEach-Object { "$_" }
gcloud compute instances get-serial-port-output $env:WORKER_VM @a 2>&1 | Select-Object -Last 40 | ForEach-Object { "$_" }
$h = curl.exe -s -o NUL -w "%{http_code} %{time_total}s" "https://35.254.78.215.sslip.io/health"
Write-Host "GET /health -> $h"
$l = curl.exe -s -o NUL -w "%{http_code} %{time_total}s" -H "Content-Type: application/json" --data-binary "{}" "https://35.254.78.215.sslip.io/api/v1/auth/login"
Write-Host "POST /auth/login (cuerpo vacio, se espera 400 o 401 si Redis y la BD responden) -> $l"
Stop-Transcript | Out-Null
