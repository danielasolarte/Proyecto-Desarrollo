<#
Entrega 3 - CORS del bucket de medios (Daniela).

El navegador del profesor sube los archivos DIRECTO al bucket con las URL
prefirmadas que entrega la API (PUT por partes). Como esa peticion sale de la
pagina del frontend hacia storage.googleapis.com, el bucket debe permitir ese
origen (CORS); si no, el navegador bloquea la carga.

Solo se permiten los metodos PUT (subir partes), GET y HEAD (reproducir), y
solo desde los origenes que se pasen. La autorizacion sigue estando en la
firma de cada URL.

Uso (el origen es la direccion con la que se abre el frontend, sin "/" final):
  .\deploy\gcp\e3\06-cors-bucket.ps1 -Origen "http://34.120.10.20"
  .\deploy\gcp\e3\06-cors-bucket.ps1 -Origen "http://34.120.10.20","http://localhost:5173"
  .\deploy\gcp\e3\06-cors-bucket.ps1 -Ver
#>
param(
    [string[]]$Origen,
    [switch]$Ver
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "06-cors-bucket"

if (-not $Ver) {
    $origenes = @($Origen | ForEach-Object { "$_".Split(",") } | ForEach-Object { $_.Trim().TrimEnd("/") } | Where-Object { $_ })
    $malos = @($origenes | Where-Object { $_ -notmatch '^https?://[A-Za-z0-9.\-]+(:\d+)?$' })
    if ($origenes.Count -eq 0 -or $malos.Count -gt 0) {
        Falla "Pasa uno o varios origenes validos con -Origen, por ejemplo -Origen ""http://34.120.10.20"". Invalidos: $($malos -join ', ')"
        CerrarRegistro; return
    }

    $cors = @(@{
        origin         = $origenes
        method         = @("PUT", "GET", "HEAD")
        responseHeader = @("Content-Type", "ETag")
        maxAgeSeconds  = 3600
    })
    $tmp = TextoLF (ConvertTo-Json -InputObject $cors -Depth 4)

    Titulo "CORS de gs://$BUCKET_MEDIA para: $($origenes -join ', ')"
    gcloud storage buckets update "gs://$BUCKET_MEDIA" --project=$P --cors-file=$tmp
    $codigo = $LASTEXITCODE
    Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    if ($codigo -ne 0) { Falla "No se pudo actualizar el CORS del bucket."; CerrarRegistro; return }
    Ok "CORS actualizado. Puede tardar unos minutos en aplicarse."
}

Titulo "CORS actual de gs://$BUCKET_MEDIA"
gcloud storage buckets describe "gs://$BUCKET_MEDIA" --project=$P --format="json(cors_config)"
CerrarRegistro
