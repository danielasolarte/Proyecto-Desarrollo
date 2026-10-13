<#
Entrega 3 - Secretos e identidades (Daniela).

Que hace (idempotente, se puede correr varias veces):
  1. Habilita Secret Manager, Cloud Monitoring y Cloud Logging.
  2. Crea una cuenta de servicio por rol: sa-mooc-api y sa-mooc-worker.
  3. Les da solo lo que necesitan: escribir logs y metricas, leer imagenes de
     Artifact Registry (solo el repositorio mooc-e2) y leer sus secretos.
  4. Crea los secretos mooc-database-url, mooc-s3-access-key y
     mooc-s3-secret-key. Si no tienen version, copia el valor que usa hoy la
     API del Web Server de la Entrega 2 (se lee por IAP, viaja en memoria y se
     escribe en un archivo temporal que se borra de inmediato). Nunca se
     imprimen.

Autorizacion de acceso al servicio cloud (IAM) y autenticacion de la app ante
PostgreSQL son cosas distintas: IAM decide que VM puede LEER el secreto; el
usuario y la clave que van DENTRO de DATABASE_URL son los que PostgreSQL
valida.

Uso (boton Run de VS Code o desde la raiz del repo):
  .\deploy\gcp\e3\01-secretos-identidades.ps1
  .\deploy\gcp\e3\01-secretos-identidades.ps1 -ActualizarValores   (agrega una version nueva con el valor actual del Web Server)
#>
param([switch]$ActualizarValores)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "01-secretos"

Titulo "APIs del proyecto"
gcloud services enable secretmanager.googleapis.com monitoring.googleapis.com logging.googleapis.com artifactregistry.googleapis.com --project=$P

Titulo "Cuentas de servicio"
foreach ($rol in @("api", "worker")) {
    $correo = $SA[$rol]
    $id = $correo.Split("@")[0]
    if (Existe "iam service-accounts" $correo) {
        Write-Host "Ya existe $correo"
    } else {
        gcloud iam service-accounts create $id --project=$P --display-name="MOOC E3 $rol" --description="Identidad de las VM del grupo $rol (Entrega 3)"
        Ok "Creada $correo"
        Start-Sleep -Seconds 5   # la cuenta tarda unos segundos en ser visible para IAM
    }
    foreach ($rolIam in @("roles/logging.logWriter", "roles/monitoring.metricWriter")) {
        gcloud projects add-iam-policy-binding $P --member="serviceAccount:$correo" --role=$rolIam --condition=None --quiet --format=none
    }
    gcloud artifacts repositories add-iam-policy-binding $REPO_AR --location=$R --project=$P --member="serviceAccount:$correo" --role="roles/artifactregistry.reader" --quiet --format=none
    Write-Host "  $rol -> logging.logWriter, monitoring.metricWriter (proyecto); artifactregistry.reader (repositorio $REPO_AR)"
}

Titulo "Secretos"
# Valores actuales de la API de la Entrega 2. Solo se leen si hacen falta.
$valores = $null
function LeerValoresWebServer {
    $remoto = @'
C=$(sudo docker ps -q --filter label=com.docker.compose.service=api | head -1)
if [ -z $C ]; then echo SIN_CONTENEDOR_API; exit 0; fi
sudo docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' $C | grep -E '^(DATABASE_URL|S3_ACCESS_KEY|S3_SECRET_KEY)='
'@
    $lineas = Ssh $env:WEB_VM $remoto
    $mapa = @{}
    foreach ($l in $lineas) {
        if ($l -match '^(DATABASE_URL|S3_ACCESS_KEY|S3_SECRET_KEY)=(.+)$') { $mapa[$Matches[1]] = $Matches[2].Trim() }
    }
    if ($lineas -match "SIN_CONTENEDOR_API") { Falla "No hay contenedor de la API corriendo en $($env:WEB_VM); enciende la VM y vuelve a correr." }
    return $mapa
}

foreach ($var in $SECRETOS.Keys) {
    $nombre = $SECRETOS[$var]
    if (-not (Existe "secrets" $nombre)) {
        gcloud secrets create $nombre --project=$P --replication-policy=automatic "--labels=app=mooc,entrega=e3" --format=none
        Ok "Creado el secreto $nombre"
    }
    $versiones = gcloud secrets versions list $nombre --project=$P --filter="state=ENABLED" --format="value(name)" 2>$null
    if (-not $versiones -or $ActualizarValores) {
        if ($null -eq $valores) { $valores = LeerValoresWebServer }
        if ($valores.ContainsKey($var) -and $valores[$var]) {
            $tmp = [System.IO.Path]::GetTempFileName()
            try {
                [System.IO.File]::WriteAllText($tmp, $valores[$var])
                gcloud secrets versions add $nombre --project=$P --data-file=$tmp --format=none
                Ok "  $nombre <- valor de $var del Web Server (no se muestra)"
            } finally { Remove-Item $tmp -Force -ErrorAction SilentlyContinue }
        } else {
            Falla "  No se pudo leer $var del Web Server; agrega el valor a mano con: gcloud secrets versions add $nombre --data-file=<archivo>"
        }
    } else {
        Write-Host "  $nombre ya tiene version activa (usa -ActualizarValores para reemplazarla)"
    }
    foreach ($rol in @("api", "worker")) {
        gcloud secrets add-iam-policy-binding $nombre --project=$P --member="serviceAccount:$($SA[$rol])" --role="roles/secretmanager.secretAccessor" --format=none 2>&1 | Out-Null
    }
    Write-Host "  $nombre legible solo por sa-mooc-api y sa-mooc-worker"
}
$valores = $null

Titulo "Resumen (sin valores)"
gcloud secrets list --project=$P --filter="labels.app=mooc" --format="table(name,createTime.date('%Y-%m-%d'))"
gcloud iam service-accounts list --project=$P --filter="email~^sa-mooc-" --format="table(email,displayName)"
CerrarRegistro
