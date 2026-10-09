<#
Correcciones de la retroalimentacion de la Entrega 2 (C1, C2, C3).

  C1  Login del Web Server devolvia 500: REDIS_ADDR del Web Server apuntaba a
      una IP interna del Worker anterior al cambio a mooc-e2-vpc.
  C2  Cloud SQL debe usar conexion privada (Private Service Access), sin IP
      publica ni redes autorizadas, y exigir cifrado.
  C3  El Worker Server no debe tener IP externa; la salida a Internet pasa por
      Cloud NAT.

Se ejecuta por fases, en este orden, revisando la salida de cada una antes de
seguir. Cada fase es idempotente: si ya estaba hecha, no la repite.

  . .\deploy\gcp\00-variables.example.ps1
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase Diagnostico
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase EncenderSql
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase Verificar -Email "correo@mooc.test" -Password "clave"
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase RedisWeb            (solo si REDIS_ADDR no es la IP del worker)
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase IpPrivadaSql
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase CambiarDatabaseUrl
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase QuitarIpPublicaSql
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase WorkerSinIpPublica

La configuracion de cada VM se ubica a partir del contenedor que esta
corriendo (docker compose ls / labels de compose), no de una ruta fija, porque
el Worker no se levanto desde /opt/mooc/deploy. Los secretos nunca se
imprimen: de DATABASE_URL solo se muestra el host.
#>
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("Diagnostico", "EncenderSql", "RedisWeb", "IpPrivadaSql", "CambiarDatabaseUrl", "QuitarIpPublicaSql", "WorkerSinIpPublica", "Verificar")]
    [string]$Fase,

    [string]$SqlInstance = "mooc-postgres",
    [string]$BaseUrl = "https://35.254.78.215.sslip.io",
    [string]$Email,
    [string]$Password
)

$ErrorActionPreference = "Continue"

if (-not $env:GCP_PROJECT -or -not $env:WEB_VM -or -not $env:WORKER_VM) {
    throw "Carga primero deploy/gcp/00-variables.example.ps1 (o tu copia con valores reales)."
}

$P = $env:GCP_PROJECT
$Z = $env:GCP_ZONE
$R = $env:GCP_REGION
$NET = $env:NETWORK

function Titulo($t) { Write-Host ""; Write-Host "==== $t ====" -ForegroundColor Cyan }

function Ssh($vm, $cmd) {
    # Quita CR (finales CRLF en Windows). Se asigna a una variable porque
    # "--command=(expr)" PowerShell lo parte en dos argumentos.
    $limpio = $cmd -replace "`r", ""
    gcloud compute ssh $vm --zone=$Z --tunnel-through-iap --project=$P --strict-host-key-checking=no --command=$limpio
}

function IpInterna($vm) {
    gcloud compute instances describe $vm --zone=$Z --project=$P --format="value(networkInterfaces[0].networkIP)"
}

function IpPrivadaSql {
    $json = gcloud sql instances describe $SqlInstance --project=$P --format=json | ConvertFrom-Json
    ($json.ipAddresses | Where-Object { $_.type -eq "PRIVATE" } | Select-Object -First 1).ipAddress
}

function RouterDeLaVpc {
    $r = gcloud compute routers list --project=$P --filter="network~$NET AND region~$R" --format="value(name)" | Select-Object -First 1
    if ($r) { return $r } else { return $env:CLOUD_ROUTER }
}

function EncenderSql {
    $estado = gcloud sql instances describe $SqlInstance --project=$P --format="value(state)"
    $politica = gcloud sql instances describe $SqlInstance --project=$P --format="value(settings.activationPolicy)"
    if ($politica -ne "ALWAYS" -or $estado -ne "RUNNABLE") {
        Write-Host "Cloud SQL esta $estado (activationPolicy=$politica). Encendiendo (tarda unos minutos)..." -ForegroundColor Yellow
        gcloud sql instances patch $SqlInstance --project=$P --activation-policy=ALWAYS --quiet
    } else { Write-Host "Cloud SQL ya esta encendida." }
}

# ---------------------------------------------------------------------------
# Fragmentos bash que se ejecutan en las VM. Son plantillas en comillas simples
# (PowerShell no expande nada) y se rellenan con -replace. Sin comillas dobles,
# para que pasen intactas por gcloud.cmd en Windows.
#   __SERVICIO__  api | worker
#   __IP__        IP nueva
# Ubica el compose del servicio en ejecucion: archivo (CF), carpeta (D) y el
# contenedor (C). El .env que se edita es D/.env.
# ---------------------------------------------------------------------------
$ubicar = @'
C=$(sudo docker ps -q --filter label=com.docker.compose.service=__SERVICIO__ | head -1)
if [ -z $C ]; then echo 'No hay un contenedor __SERVICIO__ corriendo en esta VM'; sudo docker ps; exit 1; fi
CF=$(sudo docker inspect -f '{{index .Config.Labels `com.docker.compose.project.config_files`}}' $C | cut -d, -f1)
D=$(sudo docker inspect -f '{{index .Config.Labels `com.docker.compose.project.working_dir`}}' $C)
'@

$mostrar = $ubicar + "`n" + @'
echo compose: $CF
echo carpeta: $D
sudo docker exec $C env | sed -nE 's#^DATABASE_URL=[^@]*@([^:/?]*).*#DATABASE_URL host en el contenedor: \1#p; s#^(REDIS_ADDR=.*)#\1 (contenedor)#p'
ls -l $D/.env 2>/dev/null || echo no existe el archivo .env en $D, revisar env_file del compose
'@

$cambiarDb = $ubicar + "`n" + @'
set -e
sudo cp $D/.env $D/.env.bak-antes-ip-privada
sudo sed -i -E 's#^(DATABASE_URL=[^@]*@)[^:/?]+#\1__IP__#' $D/.env
cd $D && sudo docker compose -f $CF up -d --force-recreate __SERVICIO__
sleep 8
sudo docker exec $(sudo docker ps -q --filter label=com.docker.compose.service=__SERVICIO__ | head -1) env | sed -nE 's#^DATABASE_URL=[^@]*@([^:/?]*).*#DATABASE_URL host ahora: \1#p'
sudo docker logs --tail 15 $(sudo docker ps -q --filter label=com.docker.compose.service=__SERVICIO__ | head -1)
'@

$cambiarRedis = $ubicar + "`n" + @'
set -e
sudo cp $D/.env $D/.env.bak-antes-redis
sudo sed -i 's#^REDIS_ADDR=.*#REDIS_ADDR=__IP__:6379#' $D/.env
timeout 3 bash -c '</dev/tcp/__IP__/6379' && echo 'Redis alcanzable desde el Web Server' || echo 'ATENCION: Redis NO alcanzable (revisar firewall mooc-e2-allow-redis-from-web)'
cd $D && sudo docker compose -f $CF up -d --force-recreate __SERVICIO__
sleep 5
sudo docker logs --tail 15 $(sudo docker ps -q --filter label=com.docker.compose.service=__SERVICIO__ | head -1)
'@

function Plantilla($texto, $servicio, $ip) {
    ($texto -replace "__SERVICIO__", $servicio) -replace "__IP__", $ip
}

switch ($Fase) {

    "Diagnostico" {
        Titulo "Cloud SQL ($SqlInstance)"
        gcloud sql instances describe $SqlInstance --project=$P --format="yaml(state,settings.activationPolicy,settings.tier,settings.ipConfiguration.ipv4Enabled,settings.ipConfiguration.privateNetwork,settings.ipConfiguration.authorizedNetworks,settings.ipConfiguration.requireSsl,settings.ipConfiguration.sslMode,ipAddresses)"

        Titulo "Private Service Access en $NET"
        gcloud compute addresses list --global --project=$P --filter="purpose=VPC_PEERING" --format="table(name,address,prefixLength,network.basename())"
        gcloud services vpc-peerings list --network=$NET --project=$P --quiet

        Titulo "IP externas de las VM"
        foreach ($vm in @($env:WEB_VM, $env:WORKER_VM)) {
            gcloud compute instances describe $vm --zone=$Z --project=$P --format="table(name,status,networkInterfaces[0].networkIP:label=IP_INTERNA,networkInterfaces[0].accessConfigs[0].natIP:label=IP_EXTERNA,networkInterfaces[0].accessConfigs[0].name:label=ACCESS_CONFIG)"
        }

        Titulo "Cloud Router / NAT"
        $router = RouterDeLaVpc
        gcloud compute routers nats list --router=$router --region=$R --project=$P --format="table(name,natIpAllocateOption,sourceSubnetworkIpRangesToNat)"

        Titulo "Configuracion en uso en la VM web (contenedor api)"
        Ssh $env:WEB_VM (Plantilla $mostrar "api" "")
        Titulo "Configuracion en uso en la VM worker (contenedor worker)"
        Ssh $env:WORKER_VM (Plantilla $mostrar "worker" "")

        Titulo "Salud publica"
        curl.exe -s -o NUL -w "GET /health -> %{http_code} en %{time_total}s`n" "$BaseUrl/health"
    }

    "EncenderSql" {
        EncenderSql
        gcloud sql instances describe $SqlInstance --project=$P --format="value(state)"
    }

    "RedisWeb" {
        # C1
        $workerIp = IpInterna $env:WORKER_VM
        Titulo "Apuntando REDIS_ADDR del Web Server a ${workerIp}:6379"
        Ssh $env:WEB_VM (Plantilla $cambiarRedis "api" $workerIp)
    }

    "IpPrivadaSql" {
        # C2, paso 1: activar IP privada (la publica sigue, para no cortar el servicio)
        EncenderSql
        Titulo "Rango reservado para Private Service Access"
        $rango = gcloud compute addresses list --global --project=$P --filter="name=$($env:PRIVATE_SERVICE_RANGE)" --format="value(name)"
        if (-not $rango) {
            gcloud compute addresses create $env:PRIVATE_SERVICE_RANGE --global --purpose=VPC_PEERING --prefix-length=16 --network=$NET --project=$P
        } else { Write-Host "Ya existe $rango" }

        Titulo "Peering con servicenetworking"
        $peer = gcloud services vpc-peerings list --network=$NET --project=$P --quiet --format="value(peering)"
        if (-not $peer) {
            gcloud services vpc-peerings connect --service=servicenetworking.googleapis.com --ranges=$env:PRIVATE_SERVICE_RANGE --network=$NET --project=$P
        } else { Write-Host "Ya existe el peering $peer" }

        Titulo "Activando IP privada en $SqlInstance (puede tardar varios minutos)"
        gcloud sql instances patch $SqlInstance --project=$P --network="projects/$P/global/networks/$NET" --quiet
        $ip = IpPrivadaSql
        Write-Host "IP privada de Cloud SQL: $ip" -ForegroundColor Green
        Write-Host "Siguiente: -Fase CambiarDatabaseUrl"
    }

    "CambiarDatabaseUrl" {
        # C2, paso 2: que ambas VM usen la IP privada
        $ip = IpPrivadaSql
        if (-not $ip) { throw "La instancia no tiene IP privada todavia. Corre -Fase IpPrivadaSql." }
        Titulo "Web Server: DATABASE_URL -> $ip"
        Ssh $env:WEB_VM (Plantilla $cambiarDb "api" $ip)
        Titulo "Worker Server: DATABASE_URL -> $ip"
        Ssh $env:WORKER_VM (Plantilla $cambiarDb "worker" $ip)
        Write-Host "Revisa que los logs no muestren errores de conexion y corre -Fase Verificar ANTES de quitar la IP publica."
    }

    "QuitarIpPublicaSql" {
        # C2, paso 3: sin IP publica ni redes autorizadas, con cifrado obligatorio
        if (-not (IpPrivadaSql)) { throw "Sin IP privada; no se quita la publica." }
        Titulo "Quitando redes autorizadas e IP publica de $SqlInstance"
        gcloud sql instances patch $SqlInstance --project=$P --clear-authorized-networks --quiet
        gcloud sql instances patch $SqlInstance --project=$P --no-assign-ip --quiet
        # ENCRYPTED_ONLY exige cifrado sin certificado de cliente (lo que usa sslmode=require).
        gcloud sql instances patch $SqlInstance --project=$P --ssl-mode=ENCRYPTED_ONLY --quiet
        gcloud sql instances describe $SqlInstance --project=$P --format="yaml(settings.ipConfiguration.ipv4Enabled,settings.ipConfiguration.authorizedNetworks,settings.ipConfiguration.sslMode,ipAddresses)"
        Write-Host "Nota: deploy/run-migrations.ps1 ya no puede correr desde un equipo personal; usa una VM de la VPC o Cloud SQL Auth Proxy por IAP."
    }

    "WorkerSinIpPublica" {
        # C3
        Titulo "Cloud Router y Cloud NAT"
        $router = RouterDeLaVpc
        $existe = gcloud compute routers list --project=$P --filter="name=$router" --format="value(name)"
        if (-not $existe) {
            gcloud compute routers create $router --network=$NET --region=$R --project=$P
        } else { Write-Host "Se usa el router existente $router" }
        $nat = gcloud compute routers nats list --router=$router --region=$R --project=$P --format="value(name)"
        if (-not $nat) {
            gcloud compute routers nats create $env:CLOUD_NAT --router=$router --region=$R --project=$P --nat-all-subnet-ip-ranges --auto-allocate-nat-external-ips
        } else { Write-Host "Ya existe el NAT $nat en $router" }

        Titulo "Private Google Access en la subred"
        gcloud compute networks subnets update $env:SUBNET --region=$R --project=$P --enable-private-ip-google-access

        Titulo "Quitando la IP externa de $($env:WORKER_VM)"
        $ac = gcloud compute instances describe $env:WORKER_VM --zone=$Z --project=$P --format="value(networkInterfaces[0].accessConfigs[0].name)"
        if ($ac) {
            gcloud compute instances delete-access-config $env:WORKER_VM --zone=$Z --project=$P --access-config-name="$ac"
        } else { Write-Host "El worker ya no tiene IP externa." }

        Titulo "Probando salida del worker por NAT"
        $cmd = @'
curl -s -o /dev/null -w 'storage.googleapis.com -> %{http_code}\n' https://storage.googleapis.com
curl -s -o /dev/null -w 'us-central1-docker.pkg.dev -> %{http_code}\n' https://us-central1-docker.pkg.dev/v2/
curl -s -o /dev/null -w 'database.clamav.net -> %{http_code}\n' https://database.clamav.net
sudo docker ps --format 'table {{.Names}}\t{{.Status}}'
'@
        Ssh $env:WORKER_VM $cmd
        Write-Host "Si 34.28.33.182 estaba reservada como IP estatica, liberala en Red de VPC > Direcciones IP."
    }

    "Verificar" {
        Titulo "Infraestructura"
        gcloud sql instances describe $SqlInstance --project=$P --format="table(name,state,settings.ipConfiguration.ipv4Enabled:label=IP_PUBLICA,ipAddresses[].type.join(','):label=TIPOS_IP)"
        foreach ($vm in @($env:WEB_VM, $env:WORKER_VM)) {
            gcloud compute instances describe $vm --zone=$Z --project=$P --format="table(name,status,networkInterfaces[0].networkIP:label=IP_INTERNA,networkInterfaces[0].accessConfigs[0].natIP:label=IP_EXTERNA)"
        }

        Titulo "Login real sobre $BaseUrl"
        curl.exe -s -o NUL -w "GET /health -> %{http_code} en %{time_total}s`n" "$BaseUrl/health"
        if ($Email -and $Password) {
            $jar = Join-Path $env:TEMP "mooc-verificar-cookies.txt"
            $body = @{ email = $Email; password = $Password } | ConvertTo-Json -Compress
            $bodyFile = Join-Path $env:TEMP "mooc-verificar-body.json"
            Set-Content -Path $bodyFile -Value $body -NoNewline -Encoding ascii
            curl.exe -s -c $jar -o NUL -w "POST /api/v1/auth/login -> %{http_code} en %{time_total}s`n" -H "Content-Type: application/json" --data-binary "@$bodyFile" "$BaseUrl/api/v1/auth/login"
            curl.exe -s -b $jar -o NUL -w "GET /api/v1/me (con cookie) -> %{http_code}`n" "$BaseUrl/api/v1/me"
            Select-String -Path $jar -Pattern "mooc_session|mooc_csrf" | ForEach-Object { ($_.Line -split "`t")[5] + " (cookie presente)" }
            Remove-Item $jar, $bodyFile -ErrorAction SilentlyContinue
        } else {
            Write-Host "Pasa -Email y -Password (entre comillas) de un usuario de prueba para verificar el login."
        }
    }
}
