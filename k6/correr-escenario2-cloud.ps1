<#
Correccion C4 de la Entrega 2: Escenario 2 contra el Web Server real.

Hace todo en orden y deja el registro en deploy/gcp/logs/:
  1. Genera los tres videos de prueba con ffmpeg (Docker) si no existen:
     corto_360p (15 s), medio_720p (45 s), largo_1080p (90 s), sin escalar
     la resolucion del original.
  2. Crea una cuenta admin de PRUEBA (admin.k6.e2.<fecha>@mooc.test) con clave
     aleatoria: registro + verificacion por la API y promocion a admin en
     Cloud SQL desde el Web Server. No toca cuentas del equipo.
  3. Arranca monitores en el Worker (cola asynq, CPU/memoria de worker,
     clamav y redis) y en el Web Server (CPU/memoria de la API).
  4. Corre k6/run-escenario2.ps1 contra https://<web>/api/v1 con los niveles
     indicados (concurrencia del worker fija en 5).
  5. Espera el drenaje de la cola y trae los CSV a capacity-planning/results/.

Se puede lanzar con Terminal > Run Active File en VS Code.
#>
param(
    [string[]]$Levels = @("baseline", "l1", "l2"),
    [int]$WorkerConcurrency = 2,
    [string]$BaseUrl = "https://35.254.78.215.sslip.io/api/v1",
    [int]$PauseBetweenS = 120,
    [int]$DrenajeS = 180
)

$ErrorActionPreference = "Continue"
Set-Location (Resolve-Path (Join-Path $PSScriptRoot ".."))
. .\deploy\gcp\00-variables.example.ps1
$Proyecto = $env:GCP_PROJECT; $Z = $env:GCP_ZONE
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
New-Item -ItemType Directory -Force "deploy/gcp/logs", "k6/media", "capacity-planning/results" | Out-Null
Start-Transcript -Path "deploy/gcp/logs/escenario2-$stamp.log" | Out-Null

function Titulo($t) { Write-Host ""; Write-Host "==== $t ($(Get-Date -Format 'HH:mm:ss')) ====" -ForegroundColor Cyan }
function Ssh($vm, $cmd) {
    $limpio = $cmd -replace "`r", ""
    gcloud compute ssh $vm --zone=$Z --tunnel-through-iap --project=$Proyecto --strict-host-key-checking=no --command=$limpio 2>&1 | ForEach-Object { "$_" }
}
function Scp($origen, $destino) {
    gcloud compute scp $origen $destino --zone=$Z --tunnel-through-iap --project=$Proyecto --strict-host-key-checking=no 2>&1 | ForEach-Object { "$_" }
}

Titulo "Docker"
docker version --format "cliente {{.Client.Version}} / motor {{.Server.Version}}" 2>&1 | ForEach-Object { "$_" }
if ($LASTEXITCODE -ne 0) { Write-Host "Docker Desktop no esta corriendo." -ForegroundColor Red; Stop-Transcript | Out-Null; return }

Titulo "1. Videos de prueba"
$perfiles = @(
    @{ n = "corto_360p";  s = "640x360";   d = 15 },
    @{ n = "medio_720p";  s = "1280x720";  d = 45 },
    @{ n = "largo_1080p"; s = "1920x1080"; d = 90 }
)
foreach ($perfil in $perfiles) {
    $f = "k6/media/$($perfil.n).mp4"
    foreach ($img in @("jrottenberg/ffmpeg:6.1-alpine", "linuxserver/ffmpeg:latest")) {
        if (Test-Path $f) { break }
        Write-Host "Generando $f con $img"
        docker run --rm -v "${PWD}/k6/media:/out" $img -hide_banner -loglevel error `
            -f lavfi -i "testsrc2=size=$($perfil.s):rate=30" -f lavfi -i "sine=frequency=440:sample_rate=44100" `
            -t $perfil.d -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -b:a 128k -shortest "/out/$($perfil.n).mp4" 2>&1 | ForEach-Object { "$_" }
    }
    if (Test-Path $f) { Write-Host ("{0}: {1}, {2} s, {3:N1} MB" -f $perfil.n, $perfil.s, $perfil.d, ((Get-Item $f).Length / 1MB)) }
    else { Write-Host "No se pudo generar $f" -ForegroundColor Red; Stop-Transcript | Out-Null; return }
}

Titulo "2. Cuenta admin de prueba"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$adminEmail = "admin.k6.e2.$stamp@mooc.test"
$adminPass = "K6adm-" + [guid]::NewGuid().ToString("N").Substring(0, 14)
try {
    $reg = Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/register" -ContentType "application/json" `
        -Body (@{ email = $adminEmail; password = $adminPass; full_name = "Admin k6 Escenario 2" } | ConvertTo-Json -Compress)
    Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/verify-email" -ContentType "application/json" `
        -Body (@{ token = $reg.verification_token_dev } | ConvertTo-Json -Compress) | Out-Null
    Write-Host "Registrada y verificada: $adminEmail"
} catch { Write-Host "Fallo el registro del admin de prueba: $($_.Exception.Message)" -ForegroundColor Red; Stop-Transcript | Out-Null; return }
Scp "deploy/gcp/remoto/promover-admin-prueba.sh" "$($env:WEB_VM):/tmp/"
Ssh $env:WEB_VM "bash /tmp/promover-admin-prueba.sh $adminEmail"

Titulo "2b. Concurrencia del worker = $WorkerConcurrency y cola vacia antes de medir"
$prep = @'
C=$(sudo docker ps -aq --filter label=com.docker.compose.service=worker | head -1)
CF=$(sudo docker inspect -f '{{index .Config.Labels `com.docker.compose.project.config_files`}}' $C | cut -d, -f1)
D=$(sudo docker inspect -f '{{index .Config.Labels `com.docker.compose.project.working_dir`}}' $C)
if sudo grep -q ^WORKER_CONCURRENCY= $D/.env; then sudo sed -i s#^WORKER_CONCURRENCY=.*#WORKER_CONCURRENCY=__CONC__# $D/.env; else echo WORKER_CONCURRENCY=__CONC__ | sudo tee -a $D/.env >/dev/null; fi
cd $D && sudo docker compose -f $CF up -d --no-deps --force-recreate worker
sleep 5
W=$(sudo docker ps -q --filter label=com.docker.compose.service=worker | head -1)
sudo docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' $W | grep ^WORKER_CONCURRENCY
sudo docker logs --tail 3 $W
R=$(sudo docker ps -q --filter label=com.docker.compose.service=redis | head -1)
for i in $(seq 1 60); do
  P=$(sudo docker exec $R redis-cli LLEN asynq:{media}:pending); A=$(sudo docker exec $R redis-cli LLEN asynq:{media}:active)
  echo cola: pending=$P active=$A; [ $P = 0 ] && [ $A = 0 ] && break; sleep 10
done
free -m | head -2
'@
Ssh $env:WORKER_VM ($prep -replace "__CONC__", "$WorkerConcurrency")

Titulo "3. Monitores en las VM"
$durMonitor = 600 + ($Levels.Count * (240 + $PauseBetweenS)) + $DrenajeS + 300
foreach ($vm in @($env:WORKER_VM, $env:WEB_VM)) {
    Scp "deploy/gcp/remoto/monitor-vm.sh" "$($vm):/tmp/"
    Ssh $vm "setsid nohup bash /tmp/monitor-vm.sh $durMonitor 5 /tmp/monitor-escenario2-$stamp.csv >/dev/null 2>&1 & echo monitor iniciado en $vm por $durMonitor s"
}

Titulo "4. k6 Escenario 2 contra $BaseUrl, niveles $($Levels -join ', ')"
& .\k6\run-escenario2.ps1 -BaseUrl $BaseUrl -Levels $Levels -Tag "ws-$stamp" -PauseBetweenS $PauseBetweenS -AdminEmail $adminEmail -AdminPassword $adminPass 2>&1 | ForEach-Object { "$_" }

Titulo "5. Drenaje de la cola ($DrenajeS s) y recoleccion de metricas"
Start-Sleep -Seconds $DrenajeS
Ssh $env:WORKER_VM "tail -n 3 /tmp/monitor-escenario2-$stamp.csv"
Scp "$($env:WORKER_VM):/tmp/monitor-escenario2-$stamp.csv" "capacity-planning/results/escenario2-ws-worker-$stamp.csv"
Scp "$($env:WEB_VM):/tmp/monitor-escenario2-$stamp.csv" "capacity-planning/results/escenario2-ws-web-$stamp.csv"
Get-ChildItem k6/results -Filter "escenario2-ws-$stamp-*" | Select-Object Name, Length, LastWriteTime | Format-Table | Out-String | Write-Host

Stop-Transcript | Out-Null
Write-Host "Listo. Registro en deploy/gcp/logs/escenario2-$stamp.log"
