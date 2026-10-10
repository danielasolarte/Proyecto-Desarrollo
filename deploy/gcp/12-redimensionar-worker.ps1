<#
Ajuste minimo documentado para el Escenario 2: el Worker e2-small (2 vCPU
compartidas, 2 GiB) se satura con una sola transcodificacion de 720p mientras
corren ClamAV (~1 GiB) y Redis. Este script lo pasa a e2-medium (2 vCPU,
4 GiB): detiene la VM, cambia el tipo de maquina, la arranca y, apenas
responde, fija WORKER_CONCURRENCY y muestra el estado.
La IP interna (10.20.0.2) se conserva al detener y arrancar.
Se puede lanzar con Terminal > Run Active File. Registro en deploy/gcp/logs/.
#>
param(
    [string]$TipoMaquina = "e2-medium",
    [int]$WorkerConcurrency = 2
)
$ErrorActionPreference = "Continue"
Set-Location (Resolve-Path (Join-Path $PSScriptRoot "..\.."))
. .\deploy\gcp\00-variables.example.ps1
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
Start-Transcript -Path "deploy/gcp/logs/12-redimensionar-$stamp.log" | Out-Null
$a = @("--zone=$env:GCP_ZONE", "--project=$env:GCP_PROJECT")
function Titulo($t) { Write-Host ""; Write-Host "==== $t ($(Get-Date -Format 'HH:mm:ss')) ====" -ForegroundColor Cyan }

Titulo "Antes"
gcloud compute instances describe $env:WORKER_VM @a --format="table(name,status,machineType.basename(),networkInterfaces[0].networkIP)" 2>&1 | ForEach-Object { "$_" }
Titulo "Deteniendo $($env:WORKER_VM)"
gcloud compute instances stop $env:WORKER_VM @a 2>&1 | ForEach-Object { "$_" }
Titulo "Cambiando a $TipoMaquina"
gcloud compute instances set-machine-type $env:WORKER_VM @a --machine-type=$TipoMaquina 2>&1 | ForEach-Object { "$_" }
Titulo "Arrancando"
gcloud compute instances start $env:WORKER_VM @a 2>&1 | ForEach-Object { "$_" }
gcloud compute instances describe $env:WORKER_VM @a --format="table(name,status,machineType.basename(),networkInterfaces[0].networkIP)" 2>&1 | ForEach-Object { "$_" }

$remoto = @'
C=$(sudo docker ps -aq --filter label=com.docker.compose.service=worker | head -1)
CF=$(sudo docker inspect -f '{{index .Config.Labels `com.docker.compose.project.config_files`}}' $C | cut -d, -f1)
D=$(sudo docker inspect -f '{{index .Config.Labels `com.docker.compose.project.working_dir`}}' $C)
if sudo grep -q ^WORKER_CONCURRENCY= $D/.env; then sudo sed -i s#^WORKER_CONCURRENCY=.*#WORKER_CONCURRENCY=__CONC__# $D/.env; else echo WORKER_CONCURRENCY=__CONC__ | sudo tee -a $D/.env >/dev/null; fi
cd $D && sudo docker compose -f $CF up -d --no-deps --force-recreate worker
sleep 5
W=$(sudo docker ps -q --filter label=com.docker.compose.service=worker | head -1)
sudo docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' $W | grep ^WORKER_CONCURRENCY
R=$(sudo docker ps -q --filter label=com.docker.compose.service=redis | head -1)
for k in pending active; do echo $k: $(sudo docker exec $R redis-cli LLEN asynq:{media}:$k); done
for k in scheduled retry archived; do echo $k: $(sudo docker exec $R redis-cli ZCARD asynq:{media}:$k); done
sudo docker ps --format 'table {{.Names}}\t{{.Status}}'
free -m
nproc
'@
$limpio = ($remoto -replace "__CONC__", "$WorkerConcurrency") -replace "`r", ""
for ($i = 1; $i -le 8; $i++) {
    Titulo "Configurando el worker, intento $i"
    $salida = gcloud compute ssh $env:WORKER_VM @a --tunnel-through-iap --strict-host-key-checking=no --command=$limpio 2>&1 | ForEach-Object { "$_" }
    $salida
    if ($salida -match "WORKER_CONCURRENCY=") { break }
    Start-Sleep -Seconds 20
}
Stop-Transcript | Out-Null
