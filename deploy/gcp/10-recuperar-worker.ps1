<#
Recupera el Worker Server cuando queda saturado (SSH sin respuesta, login de la
API en 500 porque Redis no responde). Antes de reiniciar guarda evidencia:
salida de la consola serial y estado de la instancia. Luego hace
'gcloud compute instances reset', espera a que SSH responda y muestra la cola,
los contenedores y la memoria. Redis usa AOF, asi que los trabajos encolados
se conservan; los que estaban en ejecucion vuelven a la cola cuando vence su
lease en asynq.
Se puede lanzar con Terminal > Run Active File. Registro en deploy/gcp/logs/.
#>
$ErrorActionPreference = "Continue"
Set-Location (Resolve-Path (Join-Path $PSScriptRoot "..\.."))
. .\deploy\gcp\00-variables.example.ps1
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
Start-Transcript -Path "deploy/gcp/logs/10-recuperar-worker-$stamp.log" | Out-Null
$args0 = @("--zone=$env:GCP_ZONE", "--project=$env:GCP_PROJECT")

Write-Host "==== Evidencia antes del reinicio ($(Get-Date -Format s)) ====" -ForegroundColor Cyan
gcloud compute instances describe $env:WORKER_VM @args0 --format="table(name,status,machineType.basename())" 2>&1 | ForEach-Object { "$_" }
gcloud compute instances get-serial-port-output $env:WORKER_VM @args0 2>&1 | Select-Object -Last 80 |
    Set-Content -Path "docs/entrega2/evidencia-saturacion-worker-$stamp.txt" -Encoding utf8
Write-Host "Consola serial (ultimas 80 lineas) guardada en docs/entrega2/evidencia-saturacion-worker-$stamp.txt"
Get-Content "docs/entrega2/evidencia-saturacion-worker-$stamp.txt" | Select-Object -Last 25 | ForEach-Object { "$_" }

Write-Host "==== Reiniciando $($env:WORKER_VM) ($(Get-Date -Format s)) ====" -ForegroundColor Cyan
gcloud compute instances reset $env:WORKER_VM @args0 2>&1 | ForEach-Object { "$_" }
Start-Sleep -Seconds 60

$remoto = @'
R=$(sudo docker ps -q --filter label=com.docker.compose.service=redis | head -1)
echo == cola media
for k in pending active; do echo $k: $(sudo docker exec $R redis-cli LLEN asynq:{media}:$k); done
for k in scheduled retry archived; do echo $k: $(sudo docker exec $R redis-cli ZCARD asynq:{media}:$k); done
echo == contenedores
sudo docker ps -a --format 'table {{.Names}}\t{{.Status}}'
echo == memoria
free -m
uptime
'@
$limpio = $remoto -replace "`r", ""
for ($i = 1; $i -le 6; $i++) {
    Write-Host "==== Intento $i de conexion ($(Get-Date -Format s)) ====" -ForegroundColor Cyan
    $salida = gcloud compute ssh $env:WORKER_VM @args0 --tunnel-through-iap --strict-host-key-checking=no --command=$limpio 2>&1 | ForEach-Object { "$_" }
    $salida
    if ($salida -match "== contenedores") { break }
    Start-Sleep -Seconds 30
}
Write-Host "==== Salud de la API ====" -ForegroundColor Cyan
curl.exe -s -o NUL -w "GET /health -> %{http_code} en %{time_total}s`n" "https://35.254.78.215.sslip.io/health"
Stop-Transcript | Out-Null
