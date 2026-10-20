<#
Entrega 3 - Grupo de instancias administrado (MIG) de workers (Mateo).

Mismo mecanismo de arranque que la API (imagen de maquina mooc-worker-<commit>,
mooc.service y mooc-preparar.sh). Lo que cambia:

  - Senal de autoscaling: la metrica custom.googleapis.com/mooc/cola/demanda
    (pendientes + activos de la cola media), que publica cola-exportador desde
    la VM de servicios. La CPU no sirve: ffmpeg la satura con un solo trabajo.
  - Apagado ordenado: el worker espera hasta 300 s a sus trabajos activos
    (WORKER_SHUTDOWN_TIMEOUT_SECONDS) y la plantilla le da 330 s a docker stop
    (mooc-detener-s). Si no alcanza, asynq devuelve el trabajo a la cola y el
    estado en Postgres evita derivados duplicados.
  - Sin trafico de entrada: solo los health checks de Google a :8081/health
    (autorreparacion) y SSH por IAP.

Fases, en este orden la primera vez (cada una es idempotente):

  Red          Regla mooc-e3-allow-hc-worker: health checks de Google -> :8081.
  Salud        Health check mooc-worker-hc-vida (/health en 8081).
  Exportador   Corre cola-exportador (misma imagen y commit que los workers)
               en mooc-servicios. Una sola copia, fuera de los grupos.
  Plantilla    Plantilla mooc-worker-<commit>-<fecha>: e2-medium, 30 GB, sin
               IP externa, sa-mooc-worker, REDIS_ADDR y ClamAV de la VM de
               servicios, apagado ordenado de Compute Engine si se puede.
  Grupo        Crea mooc-worker-mig (1 instancia) con autorreparacion.
  Autoscaling  1 a 3 instancias segun la demanda de la cola.
  Probar       Por instancia: /health, consumo de la cola y tiempo de
               arranque; ultimas muestras del exportador.
  Estado       Instancias, plantilla, autoscaler y cola.
  Actualizar   Actualizacion progresiva a la plantilla mas reciente.
  Apagar       Autoscaling en off y 0 instancias.
  Encender     1 instancia y autoscaling en on.

Uso:
  .\deploy\gcp\e3\02-imagen-maquina.ps1 -Rol worker
  .\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Red
  .\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Salud
  .\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Exportador
  .\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Plantilla
  .\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Grupo
  .\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Probar
  .\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Autoscaling
#>
param(
    [ValidateSet("Red", "Salud", "Exportador", "Plantilla", "Grupo", "Autoscaling", "Probar", "Estado", "Actualizar", "Apagar", "Encender")]
    [string]$Fase = "Estado",
    [string]$Commit,
    [string]$S3Bucket,
    [string]$TipoMaquina = "e2-medium",
    # Demanda (pendientes + activos) que se espera por VM. Con concurrencia 2:
    # 2 en ejecucion + 2 en espera. Se calibra con las primeras corridas de 3B.
    [int]$ObjetivoPorInstancia = 4
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "07-grupo-workers-$Fase"

$MIG = "mooc-worker-mig"
$HC_VIDA = "mooc-worker-hc-vida"
$VM_SERVICIOS = "mooc-servicios"
$METRICA = "custom.googleapis.com/mooc/cola/demanda"
$DETENER_S = 330
if (-not $Commit) { $Commit = Commit }

function PlantillaMasReciente($rol = "worker") {
    gcloud compute instance-templates list --project=$P --filter="properties.labels.rol=$rol AND properties.labels.app=mooc" --sort-by="~creationTimestamp" --limit=1 --format="value(name)"
}

function IpServicios {
    gcloud compute addresses describe mooc-servicios-ip --region=$R --project=$P --format="value(address)" 2>$null
}

function Politica($modo) {
    # Metrica por grupo: el autoscaler divide la demanda total entre
    # ObjetivoPorInstancia para decidir cuantas VM hacen falta.
    gcloud compute instance-groups managed set-autoscaling $MIG --project=$P --zone=$Z `
        --min-num-replicas=1 --max-num-replicas=3 --cool-down-period=120 `
        --update-stackdriver-metric=$METRICA `
        --stackdriver-metric-single-instance-assignment=$ObjetivoPorInstancia `
        "--stackdriver-metric-filter=resource.type = `"global`" AND metric.labels.cola = `"media`"" `
        "--scale-in-control=max-scaled-in-replicas=1,time-window=600" `
        --mode=$modo --format=none
}

switch ($Fase) {

"Red" {
    Titulo "Firewall del grupo de workers"
    $regla = "mooc-e3-allow-hc-worker"
    if (Existe "compute firewall-rules" $regla) { Write-Host "Ya existe la regla $regla" }
    else {
        gcloud compute firewall-rules create $regla --project=$P --network=$NET --direction=INGRESS `
            --allow=tcp:8081 --source-ranges=$RANGOS_HC --target-tags=$($TAG.worker) `
            --description="Health checks de Google hacia /health del worker (autorreparacion)" --format=none
        if ($LASTEXITCODE -eq 0) { Ok "Creada la regla $regla" }
    }
    Write-Host "Los workers no reciben otro trafico. Salen a Redis y ClamAV por mooc-e3-allow-servicios y a Internet por Cloud NAT."
}

"Salud" {
    Titulo "Health check $HC_VIDA"
    if (Existe "compute health-checks" $HC_VIDA) { Write-Host "Ya existe $HC_VIDA" }
    else {
        gcloud compute health-checks create http $HC_VIDA --project=$P --port=8081 --request-path=/health `
            --check-interval=10s --timeout=5s --healthy-threshold=2 --unhealthy-threshold=3 `
            --description="Autorreparacion del MIG de workers: asynq arriba y Redis alcanzable" --format=none
        Ok "Creado $HC_VIDA (/health:8081 cada 10 s, timeout 5 s, 2 aciertos / 3 fallos)"
    }
}

"Exportador" {
    $contenedor = "$($IMAGEN_CONTENEDOR.worker):$Commit"
    Titulo "cola-exportador en $VM_SERVICIOS ($contenedor)"
    $remoto = @"
set -e
TOKEN=`$(curl -sf -H 'Metadata-Flavor: Google' http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')
echo "`$TOKEN" | sudo docker login -u oauth2accesstoken --password-stdin https://$REGISTRO >/dev/null
sudo docker pull -q $contenedor
sudo docker logout https://$REGISTRO >/dev/null || true
sudo docker rm -f cola-exportador >/dev/null 2>&1 || true
sudo docker run -d --name cola-exportador --restart unless-stopped --network host -e REDIS_ADDR=127.0.0.1:6379 --log-opt max-size=20m --log-opt max-file=3 --entrypoint /app/cola-exportador $contenedor >/dev/null
sleep 8
sudo docker logs --tail 5 cola-exportador 2>&1
"@
    $salida = Ssh $VM_SERVICIOS $remoto
    $salida | ForEach-Object { Write-Host "  $_" }
    if ($salida -match "denied|unauthorized|403") {
        Falla "sa-mooc-servicios no puede leer Artifact Registry o escribir metricas. Falta roles/artifactregistry.reader (repositorio $REPO_AR) o roles/monitoring.metricWriter."
    } elseif ($salida -match "publicando la cola") {
        Ok "Exportador corriendo. --restart unless-stopped lo vuelve a levantar si la VM se reinicia."
    }
}

"Plantilla" {
    $imagen = "mooc-worker-$Commit"
    if (-not (Existe "compute images" $imagen)) { Falla "No existe la imagen $imagen. Corre primero 02-imagen-maquina.ps1 -Rol worker (o pasa -Commit)."; break }
    $ip = IpServicios
    if (-not $ip) { Falla "No existe mooc-servicios-ip. Corre 04-vm-servicios.ps1."; break }

    if (-not $S3Bucket) {
        # El mismo bucket que ya usa la API: se lee de su plantilla mas reciente.
        $api = PlantillaMasReciente "api"
        $cfg = gcloud compute instance-templates describe $api --project=$P --format=json | ConvertFrom-Json
        $texto = ($cfg.properties.metadata.items | Where-Object { $_.key -eq "mooc-config" }).value
        $S3Bucket = ($texto -split "`r?`n" | Where-Object { $_ -match '^S3_BUCKET=' } | Select-Object -First 1) -replace '^S3_BUCKET=', ''
        if (-not $S3Bucket) { Falla "No se pudo leer S3_BUCKET de la plantilla $api; pasalo con -S3Bucket."; break }
        Write-Host "S3_BUCKET = $S3Bucket (de la plantilla $api)"
    }

    $config = [System.IO.File]::ReadAllText((Resolve-Path "deploy/gcp/e3/config/worker.config"))
    $config = $config -replace "__REDIS_ADDR__", "$($ip):6379" -replace "__S3_BUCKET__", $S3Bucket
    $textoSecretos = ($SECRETOS.Keys | ForEach-Object { "$_=$($SECRETOS[$_])" }) -join "`n"
    $fConfig = TextoLF $config
    $fSecretos = TextoLF $textoSecretos

    $nombre = "mooc-worker-$Commit-$(Get-Date -Format 'MMddHHmm')"
    $contenedor = "$($IMAGEN_CONTENEDOR.worker):$Commit"
    $argumentos = @(
        "compute", "instance-templates", "create", $nombre,
        "--project=$P", "--machine-type=$TipoMaquina",
        "--image=$imagen", "--image-project=$P",
        "--boot-disk-size=30GB", "--boot-disk-type=pd-balanced",
        "--network=$NET", "--subnet=$SUBNET", "--region=$R", "--no-address",
        "--tags=$($TAG.worker)",
        "--service-account=$($SA.worker)", "--scopes=cloud-platform",
        "--metadata=mooc-imagen=$contenedor,mooc-detener-s=$DETENER_S,mooc-docker-args=--add-host=clamav:$ip",
        "--metadata-from-file=mooc-config=$fConfig,mooc-secretos=$fSecretos",
        "--labels=app=mooc,entrega=e3,rol=worker,commit=$Commit", "--format=none"
    )
    Titulo "Plantilla $nombre"
    # Apagado ordenado de Compute Engine: al borrar la VM, el sistema tiene
    # hasta 6 min para detener mooc.service (330 s de docker stop + margen).
    # Si el proyecto o la version de gcloud no lo soportan, se crea sin el.
    & gcloud beta @argumentos --graceful-shutdown --graceful-shutdown-max-duration=6m
    if ($LASTEXITCODE -eq 0) {
        Ok "Plantilla con apagado ordenado de Compute Engine (hasta 6 min)."
    } else {
        Aviso "No se pudo usar el apagado ordenado de Compute Engine; se crea sin el. Si la VM se corta antes de 330 s, asynq devuelve el trabajo a la cola."
        & gcloud @argumentos
    }
    $codigo = $LASTEXITCODE
    Remove-Item $fConfig, $fSecretos -Force -ErrorAction SilentlyContinue
    if ($codigo -ne 0) { Falla "No se pudo crear la plantilla."; break }
    Ok "Plantilla $nombre creada (contenedor $contenedor, Redis $($ip):6379, ClamAV $($ip):3310)."
    if (Existe "compute instance-groups managed" $MIG @("--zone=$Z")) {
        Aviso "El grupo ya existe: aplica la plantilla nueva con -Fase Actualizar."
    }
}

"Grupo" {
    $plantilla = PlantillaMasReciente
    if (-not $plantilla) { Falla "No hay plantilla de workers. Corre -Fase Plantilla."; break }
    if (-not (Existe "compute health-checks" $HC_VIDA)) { Falla "Falta $HC_VIDA. Corre -Fase Salud."; break }
    Titulo "Grupo $MIG con $plantilla"
    if (Existe "compute instance-groups managed" $MIG @("--zone=$Z")) { Write-Host "El grupo ya existe." }
    else {
        gcloud compute instance-groups managed create $MIG --project=$P --zone=$Z `
            --template=$plantilla --size=1 --base-instance-name=mooc-worker `
            --health-check=$HC_VIDA --initial-delay=180 --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear el grupo."; break }
        Ok "Grupo creado con 1 instancia. Espera 1 a 2 minutos y corre -Fase Probar."
    }
}

"Autoscaling" {
    Titulo "Autoscaling de $MIG"
    Politica "on"
    if ($LASTEXITCODE -eq 0) { Ok "Politica aplicada: 1 a 3 VM, $ObjetivoPorInstancia trabajos (pendientes + activos) por VM, reduccion de max 1 VM cada 10 min." }
    gcloud compute instance-groups managed describe $MIG --project=$P --zone=$Z --format="yaml(autoscaler.autoscalingPolicy)"
}

"Probar" {
    Titulo "Instancias de $MIG"
    $instancias = gcloud compute instance-groups managed list-instances $MIG --project=$P --zone=$Z --format="value(name)"
    if (-not $instancias) { Falla "El grupo no tiene instancias." }
    foreach ($vm in $instancias) {
        $vm = ($vm -split "/")[-1]
        Titulo $vm
        $creada = [datetime]::Parse((gcloud compute instances describe $vm --zone=$Z --project=$P --format="value(creationTimestamp)")).ToUniversalTime()
        $remoto = @'
echo "health: $(curl -s --max-time 5 localhost:8081/health)"
systemctl is-active mooc
sudo docker logs mooc 2>&1 | grep -E 'concurrencia|escuchando|ERROR|error' | tail -4
echo ARRANCO_UTC=$(sudo docker inspect -f '{{.State.StartedAt}}' mooc 2>/dev/null)
'@
        $salida = Ssh $vm $remoto
        $salida | Where-Object { $_ -notmatch '^ARRANCO_UTC=' } | ForEach-Object { Write-Host "  $_" }
        $linea = $salida | Where-Object { $_ -match '^ARRANCO_UTC=\d{4}-' } | Select-Object -First 1
        if ($linea -and ($linea -match '^ARRANCO_UTC=(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)')) {
            $arranque = [datetime]::SpecifyKind([datetime]::Parse($Matches[1]), [DateTimeKind]::Utc)
            Write-Host ("  Tiempo de arranque: {0:N0} s desde la creacion de la VM ({1:u}) hasta el contenedor del worker ({2:u})" -f ($arranque - $creada).TotalSeconds, $creada, $arranque) -ForegroundColor Green
            Write-Host "  (listo = health ok y la linea 'escuchando en la cola' arriba: el worker ya consume de media)"
        }
    }
    Titulo "Cola (ultimas muestras de cola-exportador en $VM_SERVICIOS)"
    Ssh $VM_SERVICIOS "sudo docker logs --tail 4 cola-exportador 2>&1" | ForEach-Object { Write-Host "  $_" }
}

"Estado" {
    Titulo "Grupo $MIG"
    gcloud compute instance-groups managed list-instances $MIG --project=$P --zone=$Z --format="table(name.basename(),status,instanceHealth[0].detailedHealthState,version.instanceTemplate.basename(),currentAction)"
    gcloud compute instance-groups managed describe $MIG --project=$P --zone=$Z --format="yaml(instanceTemplate.basename(),targetSize,status,autoscaler.autoscalingPolicy.mode,autoscaler.status)"
    Write-Host "Plantilla mas reciente: $(PlantillaMasReciente)"
    Ssh $VM_SERVICIOS "sudo docker logs --tail 1 cola-exportador 2>&1" | ForEach-Object { Write-Host "Cola: $_" }
}

"Actualizar" {
    $plantilla = PlantillaMasReciente
    Titulo "Actualizacion progresiva de $MIG a $plantilla"
    gcloud compute instance-groups managed rolling-action start-update $MIG --project=$P --zone=$Z `
        --version=template=$plantilla --max-surge=1 --max-unavailable=0 --format=none
    if ($LASTEXITCODE -eq 0) { Ok "Actualizacion iniciada: entra una VM nueva y luego sale una vieja (con apagado ordenado)." }
}

"Apagar" {
    Titulo "Apagando $MIG (0 instancias)"
    Politica "off"
    gcloud compute instance-groups managed resize $MIG --project=$P --zone=$Z --size=0 --format=none
    Ok "Grupo en 0 instancias con el autoscaling en off. Los trabajos que lleguen esperan en la cola. Para volver: -Fase Encender."
}

"Encender" {
    Titulo "Encendiendo $MIG"
    gcloud compute instance-groups managed resize $MIG --project=$P --zone=$Z --size=1 --format=none
    Politica "on"
    Ok "1 instancia y autoscaling en on."
}
}

CerrarRegistro
