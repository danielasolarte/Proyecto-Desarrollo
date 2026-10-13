<#
Entrega 3 - Imagen de maquina por rol (Daniela; Mateo la usa con -Rol worker).

Por que: una VM nueva del grupo no debe instalar Docker ni descargar el
contenedor cuando el autoscaling la crea. Este script prepara todo una vez:

  1. Construye el contenedor del rol desde el commit actual y lo publica en
     Artifact Registry con la etiqueta del commit (exige el codigo sin cambios
     pendientes, para que la etiqueta represente lo que corre).
  2. Crea una VM temporal (sin IP externa, salida por Cloud NAT) que instala
     Docker, descarga el contenedor y deja mooc.service habilitado
     (maquina/instalar-imagen.sh).
  3. Espera la marca MOOC_IMAGEN_LISTA en la consola serial, apaga la VM y
     crea la imagen mooc-<rol>-<commit> en la familia mooc-<rol>.
  4. Borra la VM temporal.

Uso (Docker Desktop encendido; boton Run de VS Code usa -Rol api):
  .\deploy\gcp\e3\02-imagen-maquina.ps1 -Rol api
  .\deploy\gcp\e3\02-imagen-maquina.ps1 -Rol worker
  .\deploy\gcp\e3\02-imagen-maquina.ps1 -Rol api -SinConstruir   (usa el contenedor ya publicado con la etiqueta del commit)
#>
param(
    [ValidateSet("api", "worker")]
    [string]$Rol = "api",
    [switch]$SinConstruir,
    [switch]$Forzar
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "02-imagen-$Rol"

$commit = Commit
$contenedor = "$($IMAGEN_CONTENEDOR[$Rol]):$commit"
$nombreImagen = "mooc-$Rol-$commit"
$vmTemporal = "mooc-construir-$Rol-$commit"
$inicio = Get-Date
$dockerfile = $DOCKERFILE[$Rol]

if ((Existe "compute images" $nombreImagen) -and -not $Forzar) {
    Ok "La imagen $nombreImagen ya existe (usa -Forzar para reconstruirla)."
    CerrarRegistro; return
}

if (-not $SinConstruir) {
    Titulo "1. Contenedor $contenedor"
    if (git status --porcelain -- cmd internal go.mod go.sum $dockerfile) {
        Falla "Hay cambios sin commit en el codigo; la etiqueta $commit no los representaria. Haz commit y vuelve a correr."
        CerrarRegistro; return
    }
    gcloud auth configure-docker $REGISTRO --quiet 2>&1 | Out-Null
    docker build --progress=plain -f $dockerfile -t $contenedor . 2>&1 | Select-Object -Last 5 | ForEach-Object { "$_" }
    if ($LASTEXITCODE -ne 0) { Falla "Fallo docker build (revisa que Docker Desktop este corriendo)."; CerrarRegistro; return }
    docker push $contenedor 2>&1 | Select-Object -Last 2 | ForEach-Object { "$_" }
    if ($LASTEXITCODE -ne 0) { Falla "Fallo docker push."; CerrarRegistro; return }
    Ok "Publicado $contenedor"
} else {
    Write-Host "Se usa el contenedor ya publicado: $contenedor"
}

Titulo "2. VM temporal $vmTemporal"
if (Existe "compute instances" $vmTemporal @("--zone=$Z")) {
    Aviso "Ya existia una VM temporal con ese nombre (corrida anterior interrumpida); se borra."
    gcloud compute instances delete $vmTemporal --zone=$Z --project=$P --quiet
}
$fInstalar = ArchivoLF "deploy/gcp/e3/maquina/instalar-imagen.sh"
$fPreparar = ArchivoLF "deploy/gcp/e3/maquina/mooc-preparar.sh"
$fServicio = ArchivoLF "deploy/gcp/e3/maquina/mooc.service"
gcloud compute instances create $vmTemporal `
    --project=$P --zone=$Z `
    --machine-type=e2-medium `
    --subnet=$SUBNET --no-address `
    --image-family=debian-12 --image-project=debian-cloud `
    --boot-disk-size=20GB --boot-disk-type=pd-balanced `
    --service-account=$($SA[$Rol]) --scopes=cloud-platform `
    --metadata=mooc-imagen=$contenedor `
    --metadata-from-file="startup-script=$fInstalar,mooc-preparar-sh=$fPreparar,mooc-service=$fServicio" `
    "--labels=app=mooc,entrega=e3,uso=construir-imagen" --format=none
Remove-Item $fInstalar, $fPreparar, $fServicio -Force -ErrorAction SilentlyContinue
if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear la VM temporal."; CerrarRegistro; return }

Titulo "3. Esperando la instalacion (Docker + descarga del contenedor, unos minutos)"
$listo = $false
for ($i = 0; $i -lt 90; $i++) {
    Start-Sleep -Seconds 10
    $serial = gcloud compute instances get-serial-port-output $vmTemporal --zone=$Z --project=$P 2>$null | Out-String
    if ($serial -match "MOOC_IMAGEN_ERROR[^\r\n]*") { Falla "La instalacion fallo: $($Matches[0]). Revisa la consola serial de $vmTemporal (no se borra)."; CerrarRegistro; return }
    if ($serial -match "MOOC_IMAGEN_LISTA") { $listo = $true; break }
    if ($i % 6 -eq 5) { Write-Host "  ... $([int]((Get-Date) - $inicio).TotalSeconds) s" }
}
if (-not $listo) { Falla "No llego la marca MOOC_IMAGEN_LISTA en 15 min. Revisa la consola serial de $vmTemporal."; CerrarRegistro; return }
Ok "Instalacion terminada."

Titulo "4. Imagen $nombreImagen (familia $($FAMILIA[$Rol]))"
gcloud compute instances stop $vmTemporal --zone=$Z --project=$P --quiet --format=none
if (Existe "compute images" $nombreImagen) { gcloud compute images delete $nombreImagen --project=$P --quiet }
gcloud compute images create $nombreImagen `
    --project=$P `
    --source-disk=$vmTemporal --source-disk-zone=$Z `
    --family=$($FAMILIA[$Rol]) --storage-location=$R `
    "--labels=app=mooc,entrega=e3,rol=$Rol,commit=$commit" `
    --description="MOOC ${Rol} - Docker + $contenedor + mooc.service" --format=none
if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear la imagen; la VM temporal queda apagada para revisarla."; CerrarRegistro; return }

gcloud compute instances delete $vmTemporal --zone=$Z --project=$P --quiet --format=none
Ok "Imagen $nombreImagen creada en $([int]((Get-Date) - $inicio).TotalMinutes) min. VM temporal borrada."
gcloud compute images list --project=$P --filter="family=$($FAMILIA[$Rol])" --format="table(name,family,creationTimestamp.date('%Y-%m-%d %H:%M'),diskSizeGb)"
Write-Host ""
Write-Host "Siguiente paso: .\deploy\gcp\e3\03-grupo-api.ps1 -Fase Plantilla   (o el script del grupo de workers)"
CerrarRegistro
