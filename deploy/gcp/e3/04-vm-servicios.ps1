<#
Entrega 3 - VM de servicios: Redis (cola asynq + sesiones) y ClamAV (Mateo).

Es una VM fija, fuera de los grupos de API y workers: el autoscaling nunca la
retira, asi que la cola no depende de una maquina que pueda desaparecer.

Fases, en este orden la primera vez (cada una es idempotente):

  Identidad   Cuenta sa-mooc-servicios: escribir logs y metricas (exportador
              de la cola) y leer solo mooc-database-url (migraciones
              controladas, que corren aqui y no en cada replica).
  Red         IP interna reservada mooc-servicios-ip (las plantillas de los
              grupos guardan REDIS_ADDR; si la VM se recrea, la IP no cambia)
              y regla mooc-e3-allow-servicios: tcp:6379 y tcp:3310 solo desde
              las etiquetas mooc-api y mooc-worker.
  Crear       VM mooc-servicios (e2-medium, sin IP externa, Debian 12). El
              script de arranque instala Docker si falta y levanta
              deploy/docker-compose.servicios.yml. Sin acceso manual.
  Probar      Contenedores, Redis (ping, AOF), ClamAV (PING) y conectividad
              real desde una instancia de mooc-api-mig a 6379 y 3310.
  Replicar    Copia en vivo el Redis del Worker de la E2 (REPLICAOF): cola y
              sesiones pasan sin perder trabajos.
  Promover    Corta la replicacion; el Redis nuevo queda como principal.
              Justo despues, Daniela crea la plantilla de la API con
              -RedisAddr <IP>:6379 y aplica -Fase Actualizar.
  Estado      VM, IP y contenedores.
  Apagar      Detiene la VM (el disco y la cola se conservan).
  Encender    La vuelve a arrancar (el script de arranque levanta todo).

Uso:
  .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Identidad
  .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Red
  .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Crear
  .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Probar
  .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Replicar
  .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Promover
#>
param(
    [ValidateSet("Identidad", "Red", "Crear", "Probar", "Replicar", "Promover", "Estado", "Apagar", "Encender")]
    [string]$Fase = "Estado",
    [string]$TipoMaquina = "e2-medium"
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "04-vm-servicios-$Fase"

$VM = "mooc-servicios"
$IP_NOMBRE = "mooc-servicios-ip"
$REGLA = "mooc-e3-allow-servicios"
$REGLA_REPLICA = "mooc-e3-redis-replica-temporal"

function IpServicios {
    gcloud compute addresses describe $IP_NOMBRE --region=$R --project=$P --format="value(address)" 2>$null
}

switch ($Fase) {

"Identidad" {
    Titulo "Cuenta de servicio de la VM de servicios"
    $correo = $SA.servicios
    if (Existe "iam service-accounts" $correo) {
        Write-Host "Ya existe $correo"
    } else {
        gcloud iam service-accounts create $correo.Split("@")[0] --project=$P --display-name="MOOC E3 servicios" --description="VM fija con Redis y ClamAV (Entrega 3)"
        Ok "Creada $correo"
        Start-Sleep -Seconds 5   # la cuenta tarda unos segundos en ser visible para IAM
    }
    foreach ($rolIam in @("roles/logging.logWriter", "roles/monitoring.metricWriter")) {
        gcloud projects add-iam-policy-binding $P --member="serviceAccount:$correo" --role=$rolIam --condition=None --quiet --format=none
    }
    gcloud secrets add-iam-policy-binding $SECRETOS.DATABASE_URL --project=$P --member="serviceAccount:$correo" --role="roles/secretmanager.secretAccessor" --format=none 2>&1 | Out-Null
    Write-Host "  servicios -> logging.logWriter, monitoring.metricWriter (proyecto); secretAccessor solo en $($SECRETOS.DATABASE_URL)"
    Write-Host "  No lee las claves de Cloud Storage: Redis y ClamAV no las necesitan."
}

"Red" {
    Titulo "IP interna reservada"
    if (Existe "compute addresses" $IP_NOMBRE @("--region=$R")) {
        Write-Host "Ya existe $IP_NOMBRE"
    } else {
        gcloud compute addresses create $IP_NOMBRE --project=$P --region=$R --subnet=$SUBNET --description="IP fija de la VM de servicios (Redis y ClamAV)" --format=none
        Ok "Reservada $IP_NOMBRE"
    }
    Write-Host "IP de servicios: $(IpServicios)"

    Titulo "Firewall"
    if (Existe "compute firewall-rules" $REGLA) {
        Write-Host "Ya existe la regla $REGLA"
    } else {
        gcloud compute firewall-rules create $REGLA --project=$P --network=$NET --direction=INGRESS `
            --allow=tcp:6379,tcp:3310 --source-tags="$($TAG.api),$($TAG.worker)" --target-tags=$($TAG.servicios) `
            --description="Redis y ClamAV de la VM de servicios, solo desde la API y los workers" --format=none
        if ($LASTEXITCODE -eq 0) { Ok "Creada la regla $REGLA" }
    }
    Write-Host "SSH por IAP ya lo cubre mooc-e3-allow-iap-ssh (etiqueta $($TAG.servicios)). Nada mas entra a esta VM."
}

"Crear" {
    $ip = IpServicios
    if (-not $ip) { Falla "No existe $IP_NOMBRE. Corre -Fase Red."; break }
    if (-not (Existe "iam service-accounts" $SA.servicios)) { Falla "No existe $($SA.servicios). Corre -Fase Identidad."; break }
    if (Existe "compute instances" $VM @("--zone=$Z")) { Write-Host "Ya existe $VM ($ip)."; break }

    Titulo "VM $VM en $ip"
    $fArranque = ArchivoLF "deploy/gcp/e3/maquina/servicios-arranque.sh"
    $fCompose = ArchivoLF "deploy/docker-compose.servicios.yml"
    gcloud compute instances create $VM --project=$P --zone=$Z `
        --machine-type=$TipoMaquina `
        --image-family=debian-12 --image-project=debian-cloud `
        --boot-disk-size=20GB --boot-disk-type=pd-balanced `
        --network=$NET --subnet=$SUBNET --private-network-ip=$ip --no-address `
        --tags=$($TAG.servicios) `
        --service-account=$($SA.servicios) --scopes=cloud-platform `
        --metadata-from-file="startup-script=$fArranque,mooc-compose=$fCompose" `
        "--labels=app=mooc,entrega=e3,rol=servicios" --format=none
    $codigo = $LASTEXITCODE
    Remove-Item $fArranque, $fCompose -Force -ErrorAction SilentlyContinue
    if ($codigo -ne 0) { Falla "No se pudo crear la VM."; break }
    Ok "VM creada. El primer arranque instala Docker y descarga las firmas de ClamAV: 3 a 5 minutos."
    Write-Host "Luego: -Fase Probar"
}

"Probar" {
    $ip = IpServicios
    Titulo "Servicios en $VM ($ip)"
    $remoto = @'
sudo journalctl -u google-startup-scripts -o cat --no-pager | grep servicios-arranque | tail -2
sudo docker ps --format 'table {{.Names}}\t{{.Status}}'
echo "redis ping: $(sudo docker exec redis redis-cli ping)"
echo "redis aof:  $(sudo docker exec redis redis-cli config get appendonly | tail -1)"
echo "redis llaves: $(sudo docker exec redis redis-cli dbsize)"
echo "clamav: $(sudo docker exec clamav clamdcheck.sh >/dev/null 2>&1 && echo PONG || echo 'todavia cargando firmas')"
'@
    Ssh $VM $remoto | ForEach-Object { Write-Host "  $_" }

    Titulo "Conectividad desde la API (regla $REGLA)"
    $api = gcloud compute instance-groups managed list-instances mooc-api-mig --project=$P --zone=$Z --format="value(name)" 2>$null | Select-Object -First 1
    if (-not $api) {
        Aviso "mooc-api-mig no tiene instancias encendidas; prueba de nuevo con el grupo encendido."
    } else {
        $api = ($api -split "/")[-1]
        $prueba = "for p in 6379 3310; do if timeout 3 bash -c `"</dev/tcp/$ip/`$p`" 2>/dev/null; then echo `"$api -> ${ip}:`$p abierto`"; else echo `"$api -> ${ip}:`$p CERRADO`"; fi; done"
        Ssh $api $prueba | ForEach-Object { Write-Host "  $_" }
    }
}

"Replicar" {
    $ipViejo = gcloud compute instances describe $env:WORKER_VM --zone=$Z --project=$P --format="value(networkInterfaces[0].networkIP)"
    Titulo "Replicando el Redis de $($env:WORKER_VM) ($ipViejo) hacia $VM"
    if (-not (Existe "compute firewall-rules" $REGLA_REPLICA)) {
        gcloud compute firewall-rules create $REGLA_REPLICA --project=$P --network=$NET --direction=INGRESS `
            --allow=tcp:6379 --source-tags=$($TAG.servicios) --target-tags=$($env:WORKER_TAG) `
            --description="TEMPORAL: replicacion del Redis de la E2 hacia la VM de servicios. Se borra en -Fase Promover." --format=none
        Ok "Creada la regla temporal $REGLA_REPLICA"
    }
    $remoto = @"
sudo docker exec redis redis-cli replicaof $ipViejo 6379
for i in `$(seq 1 30); do
  E=`$(sudo docker exec redis redis-cli info replication | tr -d '\r')
  if echo "`$E" | grep -q 'master_link_status:up' && echo "`$E" | grep -q 'master_sync_in_progress:0'; then echo SINCRONIZADO; break; fi
  sleep 2
done
echo "llaves en la VM de servicios: `$(sudo docker exec redis redis-cli dbsize)"
"@
    Ssh $VM $remoto | ForEach-Object { Write-Host "  $_" }
    $origen = Ssh $env:WORKER_VM "sudo docker exec `$(sudo docker ps -q --filter name=redis | head -1) redis-cli dbsize"
    Write-Host "  llaves en $($env:WORKER_VM): $(@($origen)[-1])"
    Aviso "La replica es de solo lectura y sigue copiando cambios. Cuando Daniela este lista para cambiar la plantilla: -Fase Promover."
}

"Promover" {
    Titulo "Promoviendo el Redis de $VM a principal"
    Ssh $VM "sudo docker exec redis redis-cli replicaof no one; sudo docker exec redis redis-cli info replication | grep role; echo llaves: `$(sudo docker exec redis redis-cli dbsize)" | ForEach-Object { Write-Host "  $_" }
    if (Existe "compute firewall-rules" $REGLA_REPLICA) {
        gcloud compute firewall-rules delete $REGLA_REPLICA --project=$P --quiet --format=none
        Ok "Borrada la regla temporal $REGLA_REPLICA"
    }
    Ok "Listo. REDIS_ADDR para las plantillas: $(IpServicios):6379"
    Write-Host "Daniela: 03-grupo-api.ps1 -Fase Plantilla -RedisAddr $(IpServicios):6379, luego -Fase Actualizar, y borrar mooc-e3-redis-temporal."
}

"Estado" {
    Titulo "VM de servicios"
    gcloud compute instances describe $VM --zone=$Z --project=$P --format="table(name,status,machineType.basename(),networkInterfaces[0].networkIP,serviceAccounts[0].email)" 2>$null
    Write-Host "IP reservada ($IP_NOMBRE): $(IpServicios)"
}

"Apagar" {
    Titulo "Deteniendo $VM"
    Aviso "Mientras este apagada, la API y los workers no tienen cola ni sesiones."
    gcloud compute instances stop $VM --zone=$Z --project=$P --format=none
    if ($LASTEXITCODE -eq 0) { Ok "Detenida. El disco (cola en AOF y firmas de ClamAV) se conserva." }
}

"Encender" {
    Titulo "Encendiendo $VM"
    gcloud compute instances start $VM --zone=$Z --project=$P --format=none
    if ($LASTEXITCODE -eq 0) { Ok "Encendida. El script de arranque levanta Redis y ClamAV; ClamAV tarda 1 a 2 min en cargar firmas." }
}
}

CerrarRegistro
