<#
Entrega 4 - Construye y publica las imagenes en Artifact Registry (Daniela).

Cada imagen se etiqueta con el commit corto (git rev-parse --short HEAD), asi
que una etiqueta corresponde exactamente a un commit. Terraform despliega por
digest la imagen de esa etiqueta (ver infra/terraform/cloudrun.tf).

Requisitos: Docker Desktop corriendo, la base ya creada con Terraform (el
repositorio mooc-e4 de Artifact Registry) y una sola vez
  gcloud auth configure-docker us-central1-docker.pkg.dev

Uso:
  .\deploy\gcp\e4\construir-imagenes.ps1                          (backend y migraciones)
  .\deploy\gcp\e4\construir-imagenes.ps1 -Servicios worker
  .\deploy\gcp\e4\construir-imagenes.ps1 -Servicios backend,frontend,worker,migraciones
  .\deploy\gcp\e4\construir-imagenes.ps1 -PermitirCambios          (solo pruebas: arbol con cambios sin commit)

Al final imprime el comando de tf.ps1 con la etiqueta.
#>
param(
    [ValidateSet("backend", "worker", "frontend", "migraciones")]
    [string[]]$Servicios = @("backend", "migraciones"),
    [switch]$PermitirCambios
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "construir-imagenes"

if (-not (ArbolLimpio) -and -not $PermitirCambios) {
    Falla "Hay cambios sin commit. Haz commit primero (la etiqueta de la imagen es el commit) o usa -PermitirCambios solo para pruebas."
    CerrarRegistro; return
}
$tag = Commit
if ($PermitirCambios -and -not (ArbolLimpio)) { $tag = "$tag-local" }
Titulo "Etiqueta: $tag"

foreach ($s in $Servicios) {
    $img = $IMAGENES[$s]
    if (-not (Test-Path $img.dockerfile)) {
        Aviso "Se omite $s : todavia no existe $($img.dockerfile)."
        continue
    }
    $ref = "$REGISTRO/$P/$REPO_AR/$($img.nombre):$tag"
    Titulo "$s -> $ref"
    # El frontend publica el commit en /version.json.
    docker build --platform linux/amd64 --progress=plain --build-arg "APP_COMMIT=$tag" -f $img.dockerfile -t $ref . 2>&1 | Select-Object -Last 8 | ForEach-Object { "$_" }
    if ($LASTEXITCODE -ne 0) { Falla "Fallo docker build de $s (revisa que Docker Desktop este corriendo)."; CerrarRegistro; return }
    docker push $ref 2>&1 | Select-Object -Last 2 | ForEach-Object { "$_" }
    if ($LASTEXITCODE -ne 0) { Falla "Fallo docker push de $s. Corre una vez: gcloud auth configure-docker $REGISTRO"; CerrarRegistro; return }
    Ok "Publicada $ref"
}

Titulo "Siguiente paso"
Write-Host "  .\deploy\gcp\e4\tf.ps1 plan -Tag $tag"
Write-Host "  .\deploy\gcp\e4\tf.ps1 apply"
CerrarRegistro
