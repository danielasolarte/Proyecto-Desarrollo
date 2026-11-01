<#
Entrega 3 - Cuenta de administrador de PRUEBA en la nube (Samara).

Cuando nadie tiene la clave del administrador de Cloud SQL, este script crea
uno de prueba sin tocar las cuentas del equipo (mismo criterio que
deploy/gcp/remoto/promover-admin-prueba.sh de la Entrega 2):

  1. Registra la cuenta por la API publica (POST /auth/register) a traves del
     balanceador. Si ya existe, sigue.
  2. Desde mooc-servicios (dentro de la VPC), lee DATABASE_URL de Secret
     Manager con la identidad de la VM y la promueve a admin activa en Cloud
     SQL. Solo acepta correos @mooc.test. La URL de la base no sale de la VM.
  3. Verifica el login por la API y que el rol sea admin.

Uso (pide la clave sin mostrarla):
  .\deploy\gcp\e3\09-admin-prueba.ps1 -Correo "admin.samara@mooc.test"

Requiere Cloud SQL, mooc-servicios y el grupo de la API encendidos.
#>
param(
    [Parameter(Mandatory = $true)][string]$Correo,
    [string]$Vm = "mooc-servicios"
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "09-admin-prueba"

function Fin { CerrarRegistro; exit }

$Correo = $Correo.Trim().ToLower()
if ($Correo -notmatch '^[a-z0-9._+-]+@mooc\.test$') { Falla "Solo se permiten correos de prueba @mooc.test (letras, numeros, punto, guion)."; Fin }

$ip = "$(gcloud compute addresses describe mooc-e3-lb-ip --global --project=$P --format='value(address)' 2>$null)".Trim()
if (-not $ip) { Falla "No existe el balanceador (mooc-e3-lb-ip). Corre 08-balanceador.ps1 -Fase Crear."; Fin }
$api = "https://$ip.sslip.io/api/v1"

$estado = "$(gcloud compute instances describe $Vm --zone=$Z --project=$P --format='value(status)' 2>$null)".Trim()
if ($estado -ne "RUNNING") { Falla "$Vm esta '$estado'. Enciendela con .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Encender"; Fin }

$segura = Read-Host "Clave para $Correo (minimo 8 caracteres)" -AsSecureString
$clave = [System.Net.NetworkCredential]::new('', $segura).Password
if ($clave.Length -lt 8) { Falla "La clave debe tener al menos 8 caracteres."; Fin }

function Enviar($ruta, $datos) {
    $cuerpo = TextoLF ($datos | ConvertTo-Json -Compress)
    $salida = [System.IO.Path]::GetTempFileName()
    $codigo = & curl.exe -s -o $salida -w "%{http_code}" --max-time 20 -H "Content-Type: application/json" --data-binary "@$cuerpo" "$api$ruta"
    $texto = Get-Content $salida -Raw -ErrorAction SilentlyContinue
    Remove-Item $cuerpo, $salida -Force -ErrorAction SilentlyContinue
    return [pscustomobject]@{ Codigo = "$codigo"; Texto = "$texto" }
}

# ---------------------------------------------------------------- 1
Titulo "1. Registro de $Correo por la API ($api)"
$r = Enviar "/auth/register" @{ email = $Correo; password = $clave; full_name = "Admin pruebas E3" }
if ($r.Codigo -match '^20') { Ok "Cuenta registrada." }
elseif ($r.Codigo -eq "409") { Aviso "La cuenta ya existia; se promueve igual (la clave debe ser la que ya tenia)." }
else { Falla "El registro respondio $($r.Codigo): $($r.Texto)"; Fin }

# ---------------------------------------------------------------- 2
Titulo "2. Promocion a admin en Cloud SQL desde $Vm"
$remoto = @'
set -euo pipefail
MD=http://metadata.google.internal/computeMetadata/v1
H='Metadata-Flavor: Google'
PROYECTO=$(curl -sf -H "$H" "$MD/project/project-id")
TOKEN=$(curl -sf -H "$H" "$MD/instance/service-accounts/default/token" | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')
DATABASE_URL=$(curl -sf -H "Authorization: Bearer $TOKEN" "https://secretmanager.googleapis.com/v1/projects/$PROYECTO/secrets/mooc-database-url/versions/latest:access" \
  | python3 -c 'import sys,json,base64; print(base64.b64decode(json.load(sys.stdin)["payload"]["data"]).decode())') \
  || { echo "ERROR: no se pudo leer mooc-database-url desde la VM"; exit 1; }
export DATABASE_URL
docker run --rm -i --network host -e DATABASE_URL postgres:16 sh -c 'psql "$DATABASE_URL" -X -q -v ON_ERROR_STOP=1 -At' <<'SQL'
UPDATE users
   SET role = 'admin', status = 'active',
       email_verified_at = COALESCE(email_verified_at, now()), updated_at = now()
 WHERE lower(email) = '__CORREO__'
RETURNING 'PROMOVIDO ' || email || ' ' || role || ' ' || status;
SQL
'@
$remoto = $remoto.Replace("__CORREO__", $Correo) -replace "`r", ""
$b64 = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($remoto))
$salida = Ssh $Vm "echo $b64 | base64 -d > /tmp/mooc-admin-prueba.sh && sudo bash /tmp/mooc-admin-prueba.sh; rm -f /tmp/mooc-admin-prueba.sh"
$salida | Where-Object { $_ -notmatch 'Unable to find image|Pulling|Pull complete|Digest:|Status: Downloaded|Already exists|Waiting|Verifying|Download complete' } | ForEach-Object { Write-Host "  $_" }
if (-not ($salida -match "^PROMOVIDO ")) { Falla "No se pudo promover la cuenta. Revisa la salida de arriba."; Fin }
Ok "Cuenta promovida a admin activa."

# ---------------------------------------------------------------- 3
Titulo "3. Verificacion del login"
$r = Enviar "/auth/login" @{ email = $Correo; password = $clave }
$rol = try { ($r.Texto | ConvertFrom-Json).user.role } catch { "" }
if ($r.Codigo -eq "200" -and $rol -eq "admin") {
    Ok "Login 200 con rol admin."
    Write-Host ""
    Write-Host "Ahora, desde frontend/:"
    Write-Host "  `$env:MOOC_ADMIN_EMAIL = ""$Correo"""
    Write-Host "  `$env:MOOC_ADMIN_PASSWORD = ""<la clave que escribiste>"""
    Write-Host "  npm run cuentas"
} else {
    Falla "El login respondio $($r.Codigo) con rol '$rol'. Si la cuenta ya existia, la clave debe ser la original."
}
$clave = $null
Fin
