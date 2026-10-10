<#
Diagnostico del Worker Server: cola asynq, estado de contenedores, memoria,
ultimas lineas del worker y de clamav, y ultimas muestras del monitor.
Se puede lanzar con Terminal > Run Active File. Registro en deploy/gcp/logs/.
#>
$ErrorActionPreference = "Continue"
Set-Location (Resolve-Path (Join-Path $PSScriptRoot "..\.."))
. .\deploy\gcp\00-variables.example.ps1
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
Start-Transcript -Path "deploy/gcp/logs/09-diagnostico-$stamp.log" | Out-Null
$remoto = @'
R=$(sudo docker ps -q --filter label=com.docker.compose.service=redis | head -1)
W=$(sudo docker ps -aq --filter label=com.docker.compose.service=worker | head -1)
C=$(sudo docker ps -q --filter label=com.docker.compose.service=clamav | head -1)
echo == cola media
for k in pending active; do echo $k: $(sudo docker exec $R redis-cli LLEN asynq:{media}:$k); done
for k in scheduled retry archived completed; do echo $k: $(sudo docker exec $R redis-cli ZCARD asynq:{media}:$k); done
echo == contenedores
sudo docker ps -a --format 'table {{.Names}}\t{{.Status}}'
echo == memoria
free -m
uptime
echo == docker stats
sudo docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}'
echo == worker ultimas 60 lineas
sudo docker logs --tail 60 $W 2>&1
echo == clamav ultimas 15 lineas
sudo docker logs --tail 15 $C 2>&1
echo == monitor
ls -1t /tmp/monitor-escenario2-*.csv | head -1 | xargs tail -n 8
'@
$limpio = $remoto -replace "`r", ""
gcloud compute ssh $env:WORKER_VM --zone=$env:GCP_ZONE --tunnel-through-iap --project=$env:GCP_PROJECT --strict-host-key-checking=no --command=$limpio 2>&1 | ForEach-Object { "$_" }
Stop-Transcript | Out-Null
