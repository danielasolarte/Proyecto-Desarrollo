<#
Entrega 3 - Cuentas de administrador en Cloud SQL (Daniela).

La API no permite cambiar la clave de otro usuario y, en la nube, no hay
correo para "olvide mi clave". Este script lo resuelve desde la VM de
servicios (mooc-servicios), que llega a Cloud SQL por IP privada y lee
DATABASE_URL de Secret Manager, igual que 05-migraciones.ps1.

  Listar   Muestra las cuentas con rol admin (correo, estado, fecha).
  Clave    Pone una clave nueva, generada al azar, a la cuenta admin
           -Correo. Si el correo no existe, crea la cuenta admin (activa)
           con -Nombre. Nunca cambia el rol de una cuenta que no sea admin.

La clave se muestra UNA vez en esta consola y no se guarda en ningun
archivo ni registro. En la base solo queda su hash bcrypt.

Uso (Cloud SQL y mooc-servicios encendidas):
  .\deploy\gcp\e3\09-cuenta-admin.ps1 -Fase Listar
  .\deploy\gcp\e3\09-cuenta-admin.ps1 -Fase Clave -Correo admin.k6@mooc.test
  .\deploy\gcp\e3\09-cuenta-admin.ps1 -Fase Clave -Correo samara.admin@mooc.test -Nombre "Samara Admin"
#>
param(
    [ValidateSet("Listar", "Clave")]
    [string]$Fase = "Listar",
    [string]$Correo,
    [string]$Nombre = "Administrador MOOC",
    [string]$Vm = "mooc-servicios"
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")

$estado = gcloud compute instances describe $Vm --zone=$Z --project=$P --format="value(status)" 2>$null
if ($estado -ne "RUNNING") { Falla "$Vm esta '$estado'. Enciendela con .\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Encender"; return }
$sql = gcloud sql instances describe mooc-postgres --project=$P --format="value(state)" 2>$null
if ($sql -ne "RUNNABLE") { Falla "Cloud SQL esta '$sql'. Enciendela: gcloud sql instances patch mooc-postgres --activation-policy=ALWAYS"; return }

# Lee DATABASE_URL de Secret Manager con la identidad de la VM (solo vive en
# una variable del proceso) y corre psql en un contenedor.
$comun = @'
set -euo pipefail
H='Metadata-Flavor: Google'; MD=http://metadata.google.internal/computeMetadata/v1
PROY=$(curl -sf -H "$H" "$MD/project/project-id")
TOK=$(curl -sf -H "$H" "$MD/instance/service-accounts/default/token" | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')
export DATABASE_URL=$(curl -sf -H "Authorization: Bearer $TOK" "https://secretmanager.googleapis.com/v1/projects/$PROY/secrets/mooc-database-url/versions/latest:access" | python3 -c 'import sys,json,base64; print(base64.b64decode(json.load(sys.stdin)["payload"]["data"]).decode())')
psql() { sudo -E docker run --rm -i --network host -e DATABASE_URL postgres:16-alpine sh -c 'psql "$DATABASE_URL" -X -q -v ON_ERROR_STOP=1 "$@"' psql "$@"; }
'@

# Windows PowerShell 5.1 quita las comillas dobles y parte en pedazos un
# --command de varias lineas. Por eso el script viaja en base64 (sin comillas
# ni espacios) y la VM lo decodifica y lo ejecuta con bash.
function EnBash($script) {
    $b64 = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes(($script -replace "`r", "")))
    return "echo $b64 | base64 -d | bash"
}

if ($Fase -eq "Listar") {
    Titulo "Cuentas con rol admin en Cloud SQL"
    $cmd = $comun + "`n" + @'
psql -c "SELECT email AS correo, status AS estado, to_char(created_at,'YYYY-MM-DD') AS creada FROM users WHERE role='admin' ORDER BY created_at"
'@
    Ssh $Vm (EnBash $cmd) | ForEach-Object { Write-Host "  $_" }
    return
}

# ---------------------------------------------------------------- Clave
if ($Correo -notmatch '^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$') { Falla "Pasa un correo valido con -Correo."; return }
if ($Nombre -notmatch '^[\p{L}0-9 ._\-]{2,80}$') { Falla "El nombre solo puede tener letras, numeros, espacios, punto, guion y guion bajo."; return }

# Clave aleatoria de 16 caracteres (letras y numeros, sin caracteres ambiguos).
$alfabeto = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789".ToCharArray()
$bytes = New-Object byte[] 16
[System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
$clave = -join ($bytes | ForEach-Object { $alfabeto[$_ % $alfabeto.Length] })

Titulo "Clave nueva para $Correo"
# El hash bcrypt (costo 10, igual que la API) se calcula en la VM con
# htpasswd; $2y$ y $2a$ son el mismo algoritmo y la API espera $2a$.
$cmd = $comun + "`n" + @"
CORREO='$Correo'; NOMBRE='$Nombre'; CLAVE='$clave'
"@ + "`n" + @'
HASH=$(sudo docker run --rm httpd:2.4-alpine htpasswd -nbBC 10 x "$CLAVE" | cut -d: -f2 | tr -d '\r\n' | sed 's/^\$2y\$/$2a$/')
case "$HASH" in '$2a$10$'*) ;; *) echo "ERROR: no se pudo calcular el hash"; exit 1;; esac
psql -t -A -v e="$CORREO" -v n="$NOMBRE" -v h="$HASH" <<'SQL'
UPDATE users SET password_hash = :'h', status = 'active', updated_at = now()
 WHERE lower(email) = lower(:'e') AND role = 'admin';
INSERT INTO users (email, password_hash, full_name, role, status, email_verified_at)
SELECT :'e', :'h', :'n', 'admin', 'active', now()
 WHERE NOT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower(:'e'));
SELECT 'RESULTADO ' || email || ' ' || role || ' ' || status FROM users WHERE lower(email) = lower(:'e');
SQL
'@
$salida = @(Ssh $Vm (EnBash $cmd))
$resultado = $salida | Where-Object { $_ -match '^RESULTADO ' } | Select-Object -Last 1
if (-not $resultado) {
    $salida | ForEach-Object { Write-Host "  $_" }
    Falla "No se pudo cambiar la clave (revisa el mensaje de arriba)."
    return
}
if ($resultado -notmatch ' admin active$') {
    Falla "El correo $Correo ya existe pero no es admin ($($resultado.Trim())). No se cambio nada."
    return
}
Ok "Listo: $($resultado.Trim() -replace '^.*RESULTADO ', '')"
Write-Host ""
Write-Host "  Correo: $Correo"
Write-Host "  Clave:  $clave" -ForegroundColor Yellow
Write-Host ""
Aviso "La clave no se guarda en ningun lado: copiala ahora. Para cambiarla, vuelve a correr esta fase."
