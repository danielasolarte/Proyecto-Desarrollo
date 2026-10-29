<#
Entrega 3 - Pasos 1 a 3 del balanceador en un solo script (Samara).

  1. Revisa la sesion de gcloud y el proyecto.
  2. Enciende Cloud SQL, la VM de servicios y el grupo de la API, y prueba
     que la API responda /health/ready en 200.
  3. Crea el balanceador (08-balanceador.ps1 -Fase Crear).

Se puede correr con el boton Run de VS Code o con:
  .\deploy\gcp\e3\08-preparar-balanceador.ps1
#>
$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")

$SQL = "mooc-postgres"
$SERVICIOS = "mooc-servicios"

# ---------------------------------------------------------------- paso 1
Titulo "Paso 1: sesion de gcloud y proyecto"
$cuenta = "$(gcloud auth list --filter=status:ACTIVE --format='value(account)' 2>$null)".Trim()
if (-not $cuenta) {
    Aviso "No hay sesion activa en gcloud. Se abre el navegador para iniciar sesion."
    gcloud auth login
    $cuenta = "$(gcloud auth list --filter=status:ACTIVE --format='value(account)' 2>$null)".Trim()
    if (-not $cuenta) { Falla "No se pudo iniciar sesion en gcloud."; return }
}
gcloud config set project $P 2>$null | Out-Null
Ok "Cuenta: $cuenta   Proyecto: $P"

# ---------------------------------------------------------------- paso 2
Titulo "Paso 2: encender Cloud SQL, $SERVICIOS y el grupo de la API"

$politica = "$(gcloud sql instances describe $SQL --project=$P --format='value(settings.activationPolicy)' 2>$null)".Trim()
if ($politica -eq "ALWAYS") {
    Write-Host "Cloud SQL ($SQL) ya esta encendida."
} else {
    Write-Host "Encendiendo Cloud SQL ($SQL); tarda unos minutos..."
    gcloud sql instances patch $SQL --project=$P --activation-policy=ALWAYS --quiet
    if ($LASTEXITCODE -ne 0) { Falla "No se pudo encender Cloud SQL."; return }
}
$estadoSql = "$(gcloud sql instances describe $SQL --project=$P --format='value(state)' 2>$null)".Trim()
Write-Host "Cloud SQL: $estadoSql"

$estadoServ = "$(gcloud compute instances describe $SERVICIOS --zone=$Z --project=$P --format='value(status)' 2>$null)".Trim()
if (-not $estadoServ) {
    Falla "No existe la VM $SERVICIOS. Pidele a Mateo que corra 04-vm-servicios.ps1 -Fase Crear."; return
} elseif ($estadoServ -eq "RUNNING") {
    Write-Host "$SERVICIOS ya esta encendida."
} else {
    Write-Host "Encendiendo $SERVICIOS..."
    gcloud compute instances start $SERVICIOS --zone=$Z --project=$P --quiet
    if ($LASTEXITCODE -ne 0) { Falla "No se pudo encender $SERVICIOS."; return }
    Write-Host "Esperando 60 s a que Redis arranque..."
    Start-Sleep -Seconds 60
}

& (Join-Path $PSScriptRoot "03-grupo-api.ps1") -Fase Encender

Write-Host ""
Write-Host "Esperando 120 s a que la instancia de la API arranque..."
Start-Sleep -Seconds 120
& (Join-Path $PSScriptRoot "03-grupo-api.ps1") -Fase Probar

Write-Host ""
$respuesta = Read-Host "Arriba aparece 'HTTP/1.1 200' en /health/ready? Escribe s para crear el balanceador, o n para detenerte"
if ($respuesta -notmatch '^[sS]') {
    Aviso "Detenido antes del paso 3. Si no salio 200, espera un minuto y vuelve a correr este script (es seguro repetirlo),"
    Aviso "o copia la salida de arriba para revisarla."
    return
}

# ---------------------------------------------------------------- paso 3
Titulo "Paso 3: crear el balanceador"
& (Join-Path $PSScriptRoot "08-balanceador.ps1") -Fase Crear

Write-Host ""
Ok "Listo hasta el paso 3. Siguiente: .\deploy\gcp\e3\08-balanceador.ps1 -Fase Publicar"
