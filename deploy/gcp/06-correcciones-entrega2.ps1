<#
Correcciones de la retroalimentacion de la Entrega 2 (C1, C2, C3).

  C1  Login del Web Server devolvia 500: REDIS_ADDR del Web Server apuntaba a
      una IP interna del Worker anterior al cambio a mooc-e2-vpc.
  C2  Cloud SQL debe usar conexion privada (Private Service Access), sin IP
      publica ni redes autorizadas.
  C3  El Worker Server no debe tener IP externa; la salida a Internet pasa por
      Cloud NAT.

Se ejecuta por fases, en este orden, revisando la salida de cada una antes de
seguir. Cada fase es idempotente: si ya estaba hecha, no la repite.

  . .\deploy\gcp\00-variables.ps1              # tus variables reales
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase Diagnostico
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase RedisWeb
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase IpPrivadaSql
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase CambiarDatabaseUrl
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase QuitarIpPublicaSql
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase WorkerSinIpPublica
  .\deploy\gcp\06-correcciones-entrega2.ps1 -Fase Verificar -Email profe@mooc.test -Password ****

Requiere gcloud autenticado con permisos de Compute, Cloud SQL y Service
Networking. Los secretos nunca se imprimen: de DATABASE_URL solo se muestra
el host.
#>
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("Diagnostico", "RedisWeb", "IpPrivadaSql", "CambiarDatabaseUrl", "QuitarIpPublicaSql", "WorkerSinIpPublica", "Verificar")]
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
$ENVFILE = "/opt/mooc/deploy/.env"

function Titulo($t) { Write-Host ""; Write-Host "==== $t ====" -ForegroundColor Cyan }

function Ssh($vm, $cmd) {
    # Quita CR por si el archivo quedo con finales CRLF en Windows. Se asigna
    # antes a una variable: "--command=(expr)" PowerShell lo parte en dos
    # argumentos y gcloud responde "unrecognized arguments".
    $limpio = $cmd -replace "`r", ""
    gcloud compute ssh $vm --zone=$Z --tunnel-through-iap --project=$P --command=$limpio
}

function IpInterna($vm) {
    gcloud compute instances describe $vm --zone=$Z --project=$P --format="value(networkInterfaces[0].networkIP)"
}

# Usa el Cloud Router que ya exista en la VPC (en el proyecto se llama
# mooc-e2-nat-router); si no hay ninguno, el de 00-variables.
function RouterDeLaVpc {
    $r = gcloud compute routers list --project=$P --filter="network~$NET AND region~$R" --format="value(name)" | Select-Object -First 1
    if ($r) { return $r } else { return $env:CLOUD_ROUTER }
}

function EncenderSql {
    $estado = gcloud sql instances describe $SqlInstance --project=$P --format="value(state)"
    $politica = gcloud sql instances describe $SqlInstance --project=$P --format="value(settings.activationPolicy)"
    if ($politica -ne "ALWAYS" -or $estado -ne "RUNNABLE") {
        Write-Host "Cloud SQL esta $estado (activationPolicy=$politica). Encendiendo..." -ForegroundColor Yellow
        gcloud sql instances patch $SqlInstance --project=$P --activation-policy=ALWAYS --quiet
    }
}

function IpPrivadaSql {
    $json = gcloud sql instances describe $SqlInstance --project=$P --format=json | ConvertFrom-Json
    ($json.ipAddresses | Where-Object { $_.type -eq "PRIVATE" } | Select-Object -First 1).ipAddress
}

# Muestra solo el host de DATABASE_URL y REDIS_ADDR, nunca la clave.
$mostrarEnv = "sudo sed -nE 's#^DATABASE_URL=[^@]*@([^:/?]*).*#DATABASE_URL host: \1#p; s#^(REDIS_ADDR=.*)#\1#p' $ENVFILE"

switch ($Fase) {

    "Diagnostico" {
        Titulo "Cloud SQL ($SqlInstance)"
        gcloud sql instances describe $SqlInstance --project=$P --format="yaml(state,settings.activationPolicy,settings.tier,settings.ipConfiguration.ipv4Enabled,settings.ipConfiguration.privateNetwork,settings.ipConfiguration.authorizedNetworks,settings.ipConfiguration.requireSsl,ipAddresses)"

        Titulo "Private Service Access en $NET"
        gcloud compute addresses list --global --project=$P --filter="purpose=VPC_PEERING" --format="table(name,address,prefixLength,network.basename())"
        gcloud services vpc-peerings list --network=$NET --project=$P --quiet

        Titulo "IP externas de las VM"
        foreach ($vm in @($env:WEB_VM, $env:WORKER_VM)) {
            gcloud compute instances describe $vm --zone=$Z --project=$P --format="table(name,status,networkInterfaces[0].networkIP:label=IP_INTERNA,networkInterfaces[0].accessConfigs[0].natIP:label=IP_EXTERNA,networkInterfaces[0].accessConfigs[0].name:label=ACCESS_CONFIG)"
        }

        Titulo "Cloud Router / NAT"
        $router = RouterDeLaVpc
        gcloud compute routers list --project=$P --filter="network~$NET" --format="table(name,region.basename())"
        Write-Host "NAT configurados en ${router}:"
        gcloud compute routers nats list --router=$router --region=$R --project=$P --format="table(name,natIpAllocateOption,sourceSubnetworkIpRangesToNat)"

        Titulo "Variables relevantes en la VM web"
        Ssh $env:WEB_VM $mostrarEnv
        Titulo "Variables relevantes en la VM worker"
        Ssh $env:WORKER_VM $mostrarEnv

        Titulo "Salud publica"
        curl.exe -s -o NUL -w "GET /health -> %{http_code} en %{time_total}s`n" "$BaseUrl/health"
    }

    "RedisWeb" {
        # C1
        EncenderSql
        $workerIp = IpInterna $env:WORKER_VM
        Titulo "Apuntando REDIS_ADDR del Web Server a ${workerIp}:6379"
        $cmd = @"
set -e
sudo sed -i 's#^REDIS_ADDR=.*#REDIS_ADDR=${workerIp}:6379#' $ENVFILE
sudo grep '^REDIS_ADDR=' $ENVFILE
timeout 3 bash -c '</dev/tcp/${workerIp}/6379' && echo 'Redis alcanzable desde el Web Server' || echo 'ATENCION: Redis NO alcanzable (revisar firewall mooc-e2-allow-redis-from-web)'
cd /opt/mooc/deploy
sudo docker compose -f docker-compose.web.yml --env-file .env up -d --force-recreate api
sleep 5
sudo docker compose -f docker-compose.web.yml logs --tail 15 api
"@
        Ssh $env:WEB_VM $cmd
        Write-Host "Siguiente: -Fase Verificar con un usuario real para confirmar login 200."
    }

    "IpPrivadaSql" {
        # C2, paso 1: activar IP privada (la publica sigue, para no cortar el servicio)
        Titulo "Rango reservado para Private Service Access"
        $rango = gcloud compute addresses list --global --project=$P --filter="name=$($env:PRIVATE_SERVICE_RANGE)" --format="value(name)"
        if (-not $rango) {
            gcloud compute addresses create $env:PRIVATE_SERVICE_RANGE --global --purpose=VPC_PEERING --prefix-length=16 --network=$NET --project=$P
        } else { Write-Host "Ya existe $rango" }

        Titulo "Peering con servicenetworking"
        $peer = gcloud services vpc-peerings list --network=$NET --project=$P --format="value(peering)"
        if (-not $peer) {
            gcloud services enable servicenetworking.googleapis.com --project=$P
            gcloud services vpc-peerings connect --service=servicenetworking.googleapis.com --ranges=$env:PRIVATE_SERVICE_RANGE --network=$NET --project=$P
        } else { Write-Host "Ya existe el peering $peer" }

        EncenderSql
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
        Titulo "Cambiando el host de DATABASE_URL a $ip en ambas VM"
        $webCmd = @"
set -e
sudo cp $ENVFILE $ENVFILE.bak-antes-ip-privada
sudo sed -i -E 's#^(DATABASE_URL=[^@]*@)[^:/?]+#\1${ip}#' $ENVFILE
$mostrarEnv
cd /opt/mooc/deploy && sudo docker compose -f docker-compose.web.yml --env-file .env up -d --force-recreate api
sleep 5 && sudo docker compose -f docker-compose.web.yml logs --tail 15 api
"@
        $workerCmd = @"
set -e
sudo cp $ENVFILE $ENVFILE.bak-antes-ip-privada
sudo sed -i -E 's#^(DATABASE_URL=[^@]*@)[^:/?]+#\1${ip}#' $ENVFILE
$mostrarEnv
cd /opt/mooc/deploy && sudo docker compose -f docker-compose.worker.yml --env-file .env up -d --force-recreate worker
sleep 5 && sudo docker compose -f docker-compose.worker.yml logs --tail 15 worker
"@
        Ssh $env:WEB_VM $webCmd
        Ssh $env:WORKER_VM $workerCmd
        Write-Host "Revisa que los logs no muestren errores de conexion y corre -Fase Verificar ANTES de quitar la IP publica."
    }

    "QuitarIpPublicaSql" {
        # C2, paso 3: sin IP publica ni redes autorizadas
        if (-not (IpPrivadaSql)) { throw "Sin IP privada; no se quita la publica." }
        Titulo "Quitando redes autorizadas e IP publica de $SqlInstance"
        gcloud sql instances patch $SqlInstance --project=$P --clear-authorized-networks --quiet
        gcloud sql instances patch $SqlInstance --project=$P --no-assign-ip --quiet
        # El diagnostico mostro requireSsl=false aunque la documentacion dice
        # "SSL requerido". ENCRYPTED_ONLY exige cifrado sin pedir certificado de
        # cliente, que es lo que usa sslmode=require en DATABASE_URL.
        gcloud sql instances patch $SqlInstance --project=$P --ssl-mode=ENCRYPTED_ONLY --quiet
        gcloud sql instances describe $SqlInstance --project=$P --format="yaml(settings.ipConfiguration.ipv4Enabled,settings.ipConfiguration.authorizedNetworks,settings.ipConfiguration.sslMode,ipAddresses)"
        Write-Host "Nota: las migraciones (deploy/run-migrations.ps1) ya no se pueden correr desde tu equipo; corre migrate desde una VM de la VPC o con Cloud SQL Auth Proxy por IAP."
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

        Titulo "Private Google Access en la subred (Cloud Storage y Artifact Registry sin salir a Internet)"
        gcloud compute networks subnets update $env:SUBNET --region=$R --project=$P --enable-private-ip-google-access

        Titulo "Quitando la IP externa de $($env:WORKER_VM)"
        $ac = gcloud compute instances describe $env:WORKER_VM --zone=$Z --project=$P --format="value(networkInterfaces[0].accessConfigs[0].name)"
        if ($ac) {
            gcloud compute instances delete-access-config $env:WORKER_VM --zone=$Z --project=$P --access-config-name="$ac"
        } else { Write-Host "El worker ya no tiene IP externa." }

        Titulo "Probando salida del worker por NAT"
        $cmd = @"
curl -s -o /dev/null -w 'storage.googleapis.com -> %{http_code}\n' https://storage.googleapis.com
curl -s -o /dev/null -w 'us-central1-docker.pkg.dev -> %{http_code}\n' https://us-central1-docker.pkg.dev/v2/
curl -s -o /dev/null -w 'database.clamav.net -> %{http_code}\n' https://database.clamav.net
cd /opt/mooc/deploy && sudo docker compose -f docker-compose.worker.yml ps
"@
        Ssh $env:WORKER_VM $cmd
        Write-Host "Si la IP 34.28.33.182 estaba reservada como estatica, liberala en Red de VPC > Direcciones IP para que no siga cobrando."
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
            Write-Host "Pasa -Email y -Password de un usuario de prueba para verificar el login."
        }
    }
}
