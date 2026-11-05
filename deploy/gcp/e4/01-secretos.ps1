<#
Entrega 4 - Valores de los secretos (Camila, una vez despues de crear la base).

Terraform crea los contenedores de los secretos pero nunca sus valores, para
que no queden en el estado. Este script los genera y los carga en Secret
Manager sin mostrarlos:

  mooc-database-url    crea el usuario "mooc" en Cloud SQL con una clave
                       aleatoria y guarda postgres://mooc:<clave>@<ip privada>:5432/mooc
  mooc-s3-access-key   crea una llave HMAC de sa-medios (la que firma las URL
  mooc-s3-secret-key   del bucket) y guarda su id y su secreto
  mooc-admin-password  clave aleatoria de la primera cuenta de administrador;
                       el backend crea esa cuenta al arrancar si la base no
                       tiene ningun administrador. Para verla UNA vez:
                       gcloud secrets versions access latest --secret=mooc-admin-password

Cada valor viaja en memoria y en un archivo temporal que se borra de
inmediato. Si un secreto ya tiene version, no se toca (usa -Rotar para
generar valores nuevos).

Uso:
  .\deploy\gcp\e4\01-secretos.ps1
  .\deploy\gcp\e4\01-secretos.ps1 -Rotar
#>
param([switch]$Rotar)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "01-secretos"

function TieneVersion($secreto) {
    $v = gcloud secrets versions list $secreto --project=$P --filter="state=ENABLED" --format="value(name)" --limit=1 2>$null
    return [bool]$v
}

function GuardarSecreto($secreto, $valor) {
    $tmp = [System.IO.Path]::GetTempFileName()
    try {
        [System.IO.File]::WriteAllText($tmp, $valor)
        gcloud secrets versions add $secreto --project=$P --data-file=$tmp --format=none
        if ($LASTEXITCODE -eq 0) { Ok "  $secreto <- valor nuevo (no se muestra)" } else { Falla "  No se pudo guardar $secreto" }
    } finally { Remove-Item $tmp -Force -ErrorAction SilentlyContinue }
}

function ClaveAleatoria($largo) {
    $letras = [char[]]"ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
    $bytes = New-Object byte[] $largo
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    return -join ($bytes | ForEach-Object { $letras[$_ % $letras.Length] })
}

$ip = SalidaTerraform "sql_ip_privada"
if (-not $ip) { Falla "No se encontro la IP de Cloud SQL. Crea primero la base: .\deploy\gcp\e4\tf.ps1 plan y apply"; CerrarRegistro; return }
$saMedios = "sa-medios@$P.iam.gserviceaccount.com"

Titulo "DATABASE_URL (mooc-database-url)"
if ((TieneVersion "mooc-database-url") -and -not $Rotar) {
    Write-Host "  Ya tiene version (usa -Rotar para cambiar la clave)."
} else {
    $clave = ClaveAleatoria 32
    $existe = gcloud sql users list --instance=$SQL --project=$P --format="value(name)" 2>$null | Where-Object { $_ -eq "mooc" }
    if ($existe) {
        gcloud sql users set-password mooc --instance=$SQL --project=$P --password=$clave --format=none
    } else {
        gcloud sql users create mooc --instance=$SQL --project=$P --password=$clave --format=none
    }
    if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear o actualizar el usuario mooc en $SQL."; CerrarRegistro; return }
    # El usuario debe ser dueno de la base "mooc" para que las migraciones creen tablas.
    GuardarSecreto "mooc-database-url" "postgres://mooc:$clave@$($ip):5432/mooc?sslmode=require"
    $clave = $null
    Aviso "  Si las migraciones dicen 'permission denied for schema public', corre en Cloud SQL Studio: ALTER DATABASE mooc OWNER TO mooc;"
}

Titulo "Llave HMAC de sa-medios (mooc-s3-access-key y mooc-s3-secret-key)"
if ((TieneVersion "mooc-s3-access-key") -and (TieneVersion "mooc-s3-secret-key") -and -not $Rotar) {
    Write-Host "  Ya tienen version (usa -Rotar para crear otra llave)."
} else {
    $json = gcloud storage hmac create $saMedios --project=$P --format=json 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $json) { Falla "No se pudo crear la llave HMAC de $saMedios."; CerrarRegistro; return }
    $hmac = ($json | Out-String) | ConvertFrom-Json
    $accessId = if ($hmac.metadata) { $hmac.metadata.accessId } else { $hmac.accessId }
    GuardarSecreto "mooc-s3-access-key" $accessId
    GuardarSecreto "mooc-s3-secret-key" $hmac.secret
    $hmac = $null; $json = $null
    if ($Rotar) { Aviso "  Desactiva la llave vieja cuando la nueva funcione: gcloud storage hmac list --project=$P" }
}

Titulo "Clave del administrador inicial (mooc-admin-password)"
if ((TieneVersion "mooc-admin-password") -and -not $Rotar) {
    Write-Host "  Ya tiene version. Cambiarla no cambia la clave de una cuenta admin que ya exista."
} else {
    GuardarSecreto "mooc-admin-password" (ClaveAleatoria 24)
    Write-Host "  Correo: el de correo_admin en infra/terraform/variables.tf. Clave, cuando la necesites:"
    Write-Host "  gcloud secrets versions access latest --secret=mooc-admin-password --project=$P"
}

Titulo "Resumen (sin valores)"
gcloud secrets list --project=$P --format="table(name,createTime.date('%Y-%m-%d'))"
CerrarRegistro
