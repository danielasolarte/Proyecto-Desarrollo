<#
Entrega 3 - Grupo de instancias administrado (MIG) de la API (Daniela).

Fases, en este orden la primera vez (cada una es idempotente):

  Red          Reglas de firewall: health checks de Google -> API:8080,
               SSH por IAP a las VM de la E3 y Redis temporal (API -> Worker
               de la E2, mientras Mateo termina la VM de servicios).
  Plantilla    Plantilla de instancia mooc-api-<commit>-<fecha> con la imagen
               de maquina mooc-api-<commit>, e2-small, sin IP externa,
               cuenta sa-mooc-api y metadatos (config sin secretos + nombres
               de los secretos).
  Salud        Health checks: mooc-api-hc-listo (/health/ready, para el
               balanceador) y mooc-api-hc-vida (/health, autorreparacion).
  Grupo        Crea mooc-api-mig (1 instancia) con autorreparacion.
  Autoscaling  Politica: 1 a 3 instancias, 60 % de CPU, 120 s de
               inicializacion, reduccion de maximo 1 instancia cada 10 min.
  Probar       Por cada instancia: /health/ready, X-Instance-Id y tiempo de
               arranque (creacion de la VM -> API escuchando).
  Estado       Instancias, salud, plantilla y autoscaler.
  Actualizar   Actualizacion progresiva a la plantilla mas reciente
               (max-surge 1, max-unavailable 0).
  Apagar       Autoscaling en off y 0 instancias (para no pagar entre pruebas).
  Encender     1 instancia y autoscaling de nuevo en on.

Uso:
  .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Red
  .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Plantilla
  .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Salud
  .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Grupo
  .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Probar
  .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Autoscaling
#>
param(
    [ValidateSet("Red", "Plantilla", "Salud", "Grupo", "Autoscaling", "Probar", "Estado", "Actualizar", "Apagar", "Encender")]
    [string]$Fase = "Estado",
    # Redis compartido. Por defecto, el Redis del Worker de la E2; cuando
    # exista la VM de servicios, pasar su IP (y crear una plantilla nueva).
    [string]$RedisAddr,
    [string]$SmtpAddr = "",
    [string]$S3Bucket,
    [string]$Commit,
    [string]$TipoMaquina = "e2-small"
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "03-grupo-api-$Fase"

$MIG = "mooc-api-mig"
$HC_LISTO = "mooc-api-hc-listo"
$HC_VIDA = "mooc-api-hc-vida"
if (-not $Commit) { $Commit = Commit }

function PlantillaMasReciente {
    gcloud compute instance-templates list --project=$P --filter="properties.labels.rol=api AND properties.labels.app=mooc" --sort-by="~creationTimestamp" --limit=1 --format="value(name)"
}

function ReglaFirewall($nombre, [string[]]$parametros) {
    if (Existe "compute firewall-rules" $nombre) { Write-Host "Ya existe la regla $nombre"; return }
    gcloud compute firewall-rules create $nombre --project=$P --network=$NET @parametros --format=none
    if ($LASTEXITCODE -eq 0) { Ok "Creada la regla $nombre" }
}

switch ($Fase) {

"Red" {
    Titulo "Firewall de la Entrega 3"
    ReglaFirewall "mooc-e3-allow-hc-api" @("--direction=INGRESS", "--allow=tcp:8080", "--source-ranges=$RANGOS_HC", "--target-tags=$($TAG.api)", "--description=Health checks y balanceador de Google hacia la API en el puerto 8080")
    ReglaFirewall "mooc-e3-allow-iap-ssh" @("--direction=INGRESS", "--allow=tcp:22", "--source-ranges=35.235.240.0/20", "--target-tags=$($TAG.api),$($TAG.worker),$($TAG.servicios)", "--description=SSH solo por Identity-Aware Proxy")
    ReglaFirewall "mooc-e3-redis-temporal" @("--direction=INGRESS", "--allow=tcp:6379", "--source-tags=$($TAG.api)", "--target-tags=$($env:WORKER_TAG)", "--description=TEMPORAL: API (E3) -> Redis del Worker de la E2. Borrar cuando exista la VM de servicios.")
    Write-Host ""
    Write-Host "La API no recibe trafico de Internet directo: solo del balanceador y de los health checks (130.211.0.0/22, 35.191.0.0/16)."
}

"Plantilla" {
    $imagen = "mooc-api-$Commit"
    if (-not (Existe "compute images" $imagen)) { Falla "No existe la imagen $imagen. Corre primero 02-imagen-maquina.ps1 -Rol api (o pasa -Commit)."; break }

    if (-not $RedisAddr) {
        $ipWorker = gcloud compute instances describe $env:WORKER_VM --zone=$Z --project=$P --format="value(networkInterfaces[0].networkIP)"
        $RedisAddr = "$($ipWorker):6379"
        Aviso "REDIS_ADDR = $RedisAddr (Redis del Worker de la E2, temporal)"
    }
    if (-not $S3Bucket) {
        $S3Bucket = (Ssh $env:WEB_VM "sudo docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' `$(sudo docker ps -q --filter label=com.docker.compose.service=api | head -1) | grep ^S3_BUCKET= | cut -d= -f2" | Where-Object { $_ -match '^[a-z0-9._-]+$' })
        $S3Bucket = @($S3Bucket)[0]
        if (-not $S3Bucket) { Falla "No se pudo leer S3_BUCKET del Web Server; pasalo con -S3Bucket."; break }
        Write-Host "S3_BUCKET = $S3Bucket (leido del Web Server)"
    }

    $config = [System.IO.File]::ReadAllText((Resolve-Path "deploy/gcp/e3/config/api.config"))
    $config = $config -replace "__REDIS_ADDR__", $RedisAddr -replace "__S3_BUCKET__", $S3Bucket -replace "__SMTP_ADDR__", $SmtpAddr
    $config = ($config -split "`r?`n" | Where-Object { $_ -notmatch '^SMTP_ADDR=$' }) -join "`n"
    $textoSecretos = ($SECRETOS.Keys | ForEach-Object { "$_=$($SECRETOS[$_])" }) -join "`n"
    $fConfig = TextoLF $config
    $fSecretos = TextoLF $textoSecretos

    $nombre = "mooc-api-$Commit-$(Get-Date -Format 'MMddHHmm')"
    $contenedor = "$($IMAGEN_CONTENEDOR.api):$Commit"
    Titulo "Plantilla $nombre"
    gcloud compute instance-templates create $nombre `
        --project=$P `
        --machine-type=$TipoMaquina `
        --image=$imagen --image-project=$P `
        --boot-disk-size=20GB --boot-disk-type=pd-balanced `
        --network=$NET --subnet=$SUBNET --region=$R --no-address `
        --tags=$($TAG.api) `
        --service-account=$($SA.api) --scopes=cloud-platform `
        "--metadata=mooc-imagen=$contenedor,mooc-detener-s=40" `
        --metadata-from-file="mooc-config=$fConfig,mooc-secretos=$fSecretos" `
        "--labels=app=mooc,entrega=e3,rol=api,commit=$Commit" --format=none
    $codigo = $LASTEXITCODE
    Remove-Item $fConfig, $fSecretos -Force -ErrorAction SilentlyContinue
    if ($codigo -ne 0) { Falla "No se pudo crear la plantilla."; break }
    Ok "Plantilla $nombre creada (contenedor $contenedor)."
    if (Existe "compute instance-groups managed" $MIG @("--zone=$Z")) {
        Aviso "El grupo ya existe: aplica la plantilla nueva con -Fase Actualizar."
    }
}

"Salud" {
    Titulo "Health checks"
    if (-not (Existe "compute health-checks" $HC_LISTO)) {
        gcloud compute health-checks create http $HC_LISTO --project=$P --port=8080 --request-path=/health/ready `
            --check-interval=5s --timeout=3s --healthy-threshold=2 --unhealthy-threshold=2 `
            --description="Balanceador: la replica atiende si Postgres y Redis responden y no se esta apagando" --format=none
        Ok "Creado $HC_LISTO (/health/ready cada 5 s, timeout 3 s, 2 aciertos / 2 fallos)"
    } else { Write-Host "Ya existe $HC_LISTO" }
    if (-not (Existe "compute health-checks" $HC_VIDA)) {
        gcloud compute health-checks create http $HC_VIDA --project=$P --port=8080 --request-path=/health `
            --check-interval=10s --timeout=5s --healthy-threshold=2 --unhealthy-threshold=3 `
            --description="Autorreparacion del MIG: el proceso de la API sigue vivo" --format=none
        Ok "Creado $HC_VIDA (/health cada 10 s, timeout 5 s, 2 aciertos / 3 fallos)"
    } else { Write-Host "Ya existe $HC_VIDA" }
    Write-Host ""
    Write-Host "Separacion: el balanceador usa $HC_LISTO para dejar de enviar trafico (la VM sigue viva);"
    Write-Host "la autorreparacion usa $HC_VIDA y recrea la VM solo si el proceso deja de responder."
}

"Grupo" {
    $plantilla = PlantillaMasReciente
    if (-not $plantilla) { Falla "No hay plantilla de la API. Corre -Fase Plantilla."; break }
    if (-not (Existe "compute health-checks" $HC_VIDA)) { Falla "Falta $HC_VIDA. Corre -Fase Salud."; break }
    Titulo "Grupo $MIG con $plantilla"
    if (Existe "compute instance-groups managed" $MIG @("--zone=$Z")) {
        Write-Host "El grupo ya existe."
    } else {
        gcloud compute instance-groups managed create $MIG --project=$P --zone=$Z `
            --template=$plantilla --size=1 --base-instance-name=mooc-api `
            --health-check=$HC_VIDA --initial-delay=180 --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear el grupo."; break }
        Ok "Grupo creado con 1 instancia."
    }
    gcloud compute instance-groups managed set-named-ports $MIG --project=$P --zone=$Z --named-ports=http:8080 --format=none
    Write-Host "Puerto con nombre http:8080 (lo usa el servicio de backend del balanceador)."
    Write-Host "Espera 1 a 2 minutos y corre -Fase Probar."
}

"Autoscaling" {
    Titulo "Autoscaling de $MIG"
    gcloud compute instance-groups managed set-autoscaling $MIG --project=$P --zone=$Z `
        --min-num-replicas=1 --max-num-replicas=3 `
        --target-cpu-utilization=0.6 --cool-down-period=120 `
        "--scale-in-control=max-scaled-in-replicas=1,time-window=600" `
        --mode=on --format=none
    if ($LASTEXITCODE -eq 0) { Ok "Politica aplicada." }
    gcloud compute instance-groups managed describe $MIG --project=$P --zone=$Z --format="yaml(autoscaler.autoscalingPolicy)"
}

"Probar" {
    Titulo "Instancias de $MIG"
    $instancias = gcloud compute instance-groups managed list-instances $MIG --project=$P --zone=$Z --format="value(name)"
    if (-not $instancias) { Falla "El grupo no tiene instancias."; break }
    foreach ($vm in $instancias) {
        $vm = ($vm -split "/")[-1]
        Titulo $vm
        $creada = [datetime]::Parse((gcloud compute instances describe $vm --zone=$Z --project=$P --format="value(creationTimestamp)")).ToUniversalTime()
        $remoto = @'
curl -s -i --max-time 5 localhost:8080/health/ready | grep -iE '^(HTTP|X-Instance-Id)|status'
echo
systemctl is-active mooc
sudo journalctl -u mooc -o short-iso --no-pager | grep -E 'mooc-preparar|API iniciando' | tail -3
echo ESCUCHANDO_UTC=$(sudo docker inspect -f '{{.State.StartedAt}}' mooc 2>/dev/null)
'@
        $salida = Ssh $vm $remoto
        $salida | Where-Object { $_ -notmatch '^ESCUCHANDO_UTC=' } | ForEach-Object { Write-Host "  $_" }
        $linea = $salida | Where-Object { $_ -match '^ESCUCHANDO_UTC=\d{4}-' } | Select-Object -First 1
        if ($linea -and ($linea -match '^ESCUCHANDO_UTC=(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)')) {
            # Docker da 9 decimales; .NET solo acepta 7, asi que se corta a segundos.
            $arranque = [datetime]::SpecifyKind([datetime]::Parse($Matches[1]), [DateTimeKind]::Utc)
            Write-Host ("  Tiempo de arranque: {0:N0} s desde la creacion de la VM ({1:u}) hasta que arranco el contenedor de la API ({2:u})" -f ($arranque - $creada).TotalSeconds, $creada, $arranque) -ForegroundColor Green
            Write-Host "  (listo = /health/ready en 200 arriba; el balanceador lo confirma con 2 aciertos seguidos, unos 10 s mas)"
        }
    }
}

"Estado" {
    Titulo "Grupo $MIG"
    gcloud compute instance-groups managed list-instances $MIG --project=$P --zone=$Z --format="table(name.basename(),status,instanceHealth[0].detailedHealthState,version.instanceTemplate.basename(),currentAction)"
    gcloud compute instance-groups managed describe $MIG --project=$P --zone=$Z --format="yaml(instanceTemplate.basename(),targetSize,status,autoscaler.autoscalingPolicy.mode,autoscaler.status)"
    Write-Host "Plantilla mas reciente: $(PlantillaMasReciente)"
}

"Actualizar" {
    $plantilla = PlantillaMasReciente
    Titulo "Actualizacion progresiva de $MIG a $plantilla"
    gcloud compute instance-groups managed rolling-action start-update $MIG --project=$P --zone=$Z `
        --version=template=$plantilla --max-surge=1 --max-unavailable=0 --format=none
    if ($LASTEXITCODE -eq 0) { Ok "Actualizacion iniciada: se crea una VM nueva, pasa el health check y luego se retira una vieja." }
}

"Apagar" {
    Titulo "Apagando $MIG (0 instancias)"
    gcloud compute instance-groups managed set-autoscaling $MIG --project=$P --zone=$Z --min-num-replicas=1 --max-num-replicas=3 --target-cpu-utilization=0.6 --cool-down-period=120 "--scale-in-control=max-scaled-in-replicas=1,time-window=600" --mode=off --format=none
    gcloud compute instance-groups managed resize $MIG --project=$P --zone=$Z --size=0 --format=none
    Ok "Grupo en 0 instancias con el autoscaling en off. Para volver: -Fase Encender."
}

"Encender" {
    Titulo "Encendiendo $MIG"
    gcloud compute instance-groups managed resize $MIG --project=$P --zone=$Z --size=1 --format=none
    gcloud compute instance-groups managed set-autoscaling $MIG --project=$P --zone=$Z --min-num-replicas=1 --max-num-replicas=3 --target-cpu-utilization=0.6 --cool-down-period=120 "--scale-in-control=max-scaled-in-replicas=1,time-window=600" --mode=on --format=none
    Ok "1 instancia y autoscaling en on."
}
}

CerrarRegistro
