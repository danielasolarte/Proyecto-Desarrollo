<#
Entrega 4 - Migraciones con el Cloud Run Job mooc-migraciones (Daniela).

Reemplaza a deploy/gcp/e3/05-migraciones.ps1: ya no hace falta la VM
mooc-servicios. El job corre dentro de la VPC (Cloud SQL solo tiene IP
privada), lee DATABASE_URL de Secret Manager con sa-migraciones y ejecuta
golang-migrate con las migraciones del mismo commit que la imagen.

Orden para publicar una version con migraciones nuevas:
  .\deploy\gcp\e4\construir-imagenes.ps1 -Servicios backend,migraciones
  terraform apply (actualiza la imagen del job)
  .\deploy\gcp\e4\migrar.ps1 -Accion version
  .\deploy\gcp\e4\migrar.ps1 -Accion up

Uso:
  .\deploy\gcp\e4\migrar.ps1                 (up)
  .\deploy\gcp\e4\migrar.ps1 -Accion version
#>
param(
    [ValidateSet("up", "version")]
    [string]$Accion = "up"
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "migrar-$Accion"

$sql = gcloud sql instances describe mooc-postgres --project=$P --format="value(state)" 2>$null
if ($sql -ne "RUNNABLE") {
    Falla "Cloud SQL esta '$sql'. Enciendela: gcloud sql instances patch mooc-postgres --activation-policy=ALWAYS --project=$P"
    CerrarRegistro; return
}

$locales = @(Get-ChildItem migrations -Filter "*.up.sql" | Sort-Object Name)
$ultima = [int]($locales[-1].Name.Substring(0, 6))
Titulo "Migraciones en el repositorio: $($locales.Count) (la ultima es la $ultima, commit $(Commit))"

Titulo "Ejecutando mooc-migraciones ($Accion)"
gcloud run jobs execute mooc-migraciones --region=$R --project=$P --args=$Accion --wait 2>&1 | ForEach-Object { "$_" }
$codigo = $LASTEXITCODE

Titulo "Salida del job (Cloud Logging)"
Start-Sleep -Seconds 5
$filtro = "resource.type=cloud_run_job AND resource.labels.job_name=mooc-migraciones AND timestamp>=`"$((Get-Date).ToUniversalTime().AddMinutes(-15).ToString('yyyy-MM-ddTHH:mm:ssZ'))`""
gcloud logging read $filtro --project=$P --limit=20 --order=asc --format="value(textPayload)" 2>$null | Where-Object { $_ } | ForEach-Object { Write-Host "  $_" }

if ($codigo -ne 0) {
    Falla "El job fallo. Si dice 'dirty', una migracion quedo a medias: no publiques la version nueva."
} else {
    Ok "Job terminado. En la salida, la version aplicada debe ser $ultima."
}
CerrarRegistro
