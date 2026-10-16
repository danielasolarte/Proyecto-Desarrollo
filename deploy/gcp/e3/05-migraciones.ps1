<#
Entrega 3 - Migraciones controladas (Daniela).

Las migraciones NO corren en el arranque de las replicas (si cada VM nueva
del grupo las intentara aplicar, competirian entre si). Se aplican una sola
vez, a mano y antes de publicar una version que las necesite, desde la VM de
servicios (mooc-servicios), que esta dentro de la VPC y llega a Cloud SQL por
IP privada.

  1. Envia la carpeta migrations/ y maquina/migrar.sh a la VM como metadatos
     temporales (se borran al final) y los descarga por IAP.
  2. En la VM, migrar.sh lee DATABASE_URL de Secret Manager con la cuenta
     sa-mooc-servicios (no viaja desde este equipo ni queda en disco) y
     ejecuta golang-migrate v4.17.1 en un contenedor.
  3. golang-migrate toma un bloqueo en Postgres: dos ejecuciones simultaneas
     no aplican la misma migracion dos veces.

Orden para publicar una version con migraciones nuevas:
  .\deploy\gcp\e3\05-migraciones.ps1 -Fase Version
  .\deploy\gcp\e3\05-migraciones.ps1 -Fase Aplicar
  .\deploy\gcp\e3\02-imagen-maquina.ps1 -Rol api
  .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Plantilla
  .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Actualizar

Requiere la VM de servicios de Mateo (04-vm-servicios.ps1 -Fase Identidad,
Red y Crear) encendida, y Cloud SQL encendida.
#>
param(
    [ValidateSet("Version", "Aplicar")]
    [string]$Fase = "Version",
    [string]$Vm = "mooc-servicios"
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "05-migraciones-$Fase"

if (-not (Existe "compute instances" $Vm @("--zone=$Z"))) {
    Falla "No existe la VM $Vm. Las migraciones corren en la VM de servicios (04-vm-servicios.ps1 de Mateo)."
    CerrarRegistro; return
}
$estado = gcloud compute instances describe $Vm --zone=$Z --project=$P --format="value(status)"
if ($estado -ne "RUNNING") {
    Falla "$Vm esta $estado. Enciendela con .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Encender"
    CerrarRegistro; return
}
$sql = gcloud sql instances describe mooc-postgres --project=$P --format="value(state)" 2>$null
if ($sql -ne "RUNNABLE") {
    Falla "Cloud SQL esta '$sql'. Enciendela: gcloud sql instances patch mooc-postgres --activation-policy=ALWAYS"
    CerrarRegistro; return
}

$locales = @(Get-ChildItem migrations -Filter "*.up.sql" | Sort-Object Name)
$ultima = [int]($locales[-1].Name.Substring(0, 6))
Titulo "Migraciones en el repositorio: $($locales.Count) (la ultima es la $ultima, commit $(Commit))"

Titulo "Enviando migrations/ y migrar.sh a $Vm (por metadatos de la VM, sin scp)"
# Se empaquetan en un .tar.gz en base64 y viajan como metadatos temporales de
# la VM; ella los descarga del servidor de metadatos. Es mas robusto que scp
# en Windows cuando la VM es nueva o se recreo (llaves SSH de PuTTY).
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("mooc-mig-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force $tmpDir | Out-Null
$tgz = Join-Path $tmpDir "migraciones.tgz"
tar -czf $tgz -C migrations .
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $tgz)) { Falla "No se pudo empaquetar la carpeta migrations (tar)."; CerrarRegistro; return }
$b64 = Join-Path $tmpDir "migraciones.b64"
[System.IO.File]::WriteAllText($b64, [Convert]::ToBase64String([System.IO.File]::ReadAllBytes($tgz)))
$fMigrar = ArchivoLF "deploy/gcp/e3/maquina/migrar.sh"
gcloud compute instances add-metadata $Vm --zone=$Z --project=$P --metadata-from-file="mooc-migraciones=$b64,mooc-migrar-sh=$fMigrar" --format=none 2>&1 | Out-Null
$codigoMeta = $LASTEXITCODE
Remove-Item $tmpDir, $fMigrar -Recurse -Force -ErrorAction SilentlyContinue
if ($codigoMeta -ne 0) { Falla "No se pudieron cargar los metadatos en $Vm."; CerrarRegistro; return }

$traer = @'
MD=http://metadata.google.internal/computeMetadata/v1/instance/attributes
rm -rf /tmp/mooc-migrations && mkdir -p /tmp/mooc-migrations
curl -sf -H 'Metadata-Flavor: Google' $MD/mooc-migrar-sh > /tmp/mooc-migrar.sh
curl -sf -H 'Metadata-Flavor: Google' $MD/mooc-migraciones | base64 -d | tar -xz -C /tmp/mooc-migrations
ls /tmp/mooc-migrations/*.up.sql 2>/dev/null | wc -l
'@
$copiadas = @(Ssh $Vm $traer | Where-Object { $_ -match '^\d+$' })[-1]
if ("$copiadas".Trim() -ne "$($locales.Count)") {
    Falla "En la VM hay '$copiadas' migraciones y en el repositorio $($locales.Count). Revisa la salida de arriba."
    gcloud compute instances remove-metadata $Vm --zone=$Z --project=$P --keys=mooc-migraciones,mooc-migrar-sh --format=none 2>&1 | Out-Null
    CerrarRegistro; return
}
Write-Host "  $copiadas migraciones listas en $Vm"

$accion = if ($Fase -eq "Aplicar") { "up" } else { "version" }
Titulo "golang-migrate $accion desde $Vm"
$salida = Ssh $Vm "sudo bash /tmp/mooc-migrar.sh $accion"
$salida | ForEach-Object { Write-Host "  $_" }

$final = $salida | Where-Object { $_ -match '^\d+( \(dirty\))?$' } | Select-Object -Last 1
if ("$final" -match 'dirty') {
    Falla "La base quedo en estado dirty: una migracion fallo a mitad de camino. No publiques la version nueva; revisa el error arriba."
} elseif ($final -and [int]("$final".Trim()) -eq $ultima) {
    Ok "Cloud SQL esta en la version $ultima, igual que el repositorio."
} elseif ($final) {
    Aviso "Cloud SQL esta en la version $final y el repositorio en la $ultima. Usa -Fase Aplicar para ponerla al dia."
}
Ssh $Vm "rm -rf /tmp/mooc-migrations /tmp/mooc-migrar.sh" | Out-Null
gcloud compute instances remove-metadata $Vm --zone=$Z --project=$P --keys=mooc-migraciones,mooc-migrar-sh --format=none 2>&1 | Out-Null
CerrarRegistro
