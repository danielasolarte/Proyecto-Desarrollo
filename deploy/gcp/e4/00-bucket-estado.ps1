<#
Entrega 4 - Bucket del estado de Terraform (Daniela). Se corre UNA vez.

Terraform no puede crear el bucket que guarda su propio estado, asi que este
script lo crea con gcloud: region us-central1, acceso uniforme, sin acceso
publico y con versionado (si un apply deja el estado mal, se puede volver a
una version anterior del archivo).

Uso:
  .\deploy\gcp\e4\00-bucket-estado.ps1
#>
$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "00-bucket-estado"

Titulo "Bucket de estado gs://$BUCKET_ESTADO"
$existe = gcloud storage buckets describe "gs://$BUCKET_ESTADO" --project=$P --format="value(name)" 2>$null
if ($existe) {
    Write-Host "Ya existe gs://$BUCKET_ESTADO"
} else {
    gcloud storage buckets create "gs://$BUCKET_ESTADO" --project=$P --location=$R --uniform-bucket-level-access --public-access-prevention
    if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear el bucket."; CerrarRegistro; return }
    Ok "Creado gs://$BUCKET_ESTADO"
}
gcloud storage buckets update "gs://$BUCKET_ESTADO" --versioning --project=$P --format=none
Ok "Versionado activo."

Titulo "Siguiente paso"
Write-Host "  .\deploy\gcp\e4\tf.ps1 init"
CerrarRegistro
