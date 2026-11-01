<#
Entrega 3 - Balanceador HTTPS externo y frontend en bucket (Samara).

Un solo dominio HTTPS para todo:
  https://<IP del balanceador>.sslip.io/api/*  -> servicio de backend mooc-api-bs -> mooc-api-mig
  https://<IP del balanceador>.sslip.io/*      -> backend bucket del frontend (SPA compilada)
  http://...                                   -> redireccion 301 a https

Frontend y API comparten origen, asi que las cookies mooc_session (Secure,
HttpOnly, SameSite=Lax) y mooc_csrf funcionan sin CORS. La API no necesita
cambios: las cookies no fijan Domain y el balanceador termina el HTTPS.
/metrics y /health no se publican: solo /api/* llega a la API.

Fases (cada una es idempotente; la primera vez se puede usar -Fase Crear):

  Ip           IP global reservada mooc-e3-lb-ip. El dominio es <IP>.sslip.io.
  Bucket       Bucket del frontend (lectura publica, index.html como pagina
               principal y de error para las rutas de la SPA) y backend bucket.
  Backend      Servicio de backend mooc-api-bs: HTTP, puerto con nombre http,
               health check mooc-api-hc-listo, drenaje de 30 s, logs al 100 %,
               backend mooc-api-mig.
  Mapa         Mapa de URL: /api/* a la API y el resto al bucket. Mapa aparte
               que redirige HTTP a HTTPS.
  Certificado  Certificado administrado por Google para el dominio sslip.io.
  Frente       Proxies HTTPS y HTTP y reglas de reenvio 443 y 80 sobre la IP.
  Crear        Ip, Bucket, Backend, Mapa, Certificado y Frente, en orden.
  Publicar     Compila frontend/ con el commit actual, agrega version.json y
               sincroniza dist/ con el bucket.
  Probar       Certificado, salud de cada instancia, rutas (SPA, API, version,
               redireccion) y reparto: cuenta X-Instance-Id en una rafaga.
  Sesion       Login, /me, CSRF y logout por el balanceador con la cuenta de
               estudiante (MOOC_STUDENT_EMAIL / MOOC_STUDENT_PASSWORD).
  Excluir      Prueba de exclusion (Escenario 3A): bloquea Redis en una
               instancia, /health/ready pasa a 503, el balanceador deja de
               enviarle trafico; luego la reincorpora. Deja un CSV de evidencia.
  Estado       Recursos del balanceador y su estado.
  Borrar       Quita reglas, proxies, mapas, certificado, servicio y backend
               bucket. Conserva la IP (y con ella el dominio) y el bucket.

Uso:
  .\deploy\gcp\e3\08-balanceador.ps1 -Fase Crear
  .\deploy\gcp\e3\08-balanceador.ps1 -Fase Publicar
  .\deploy\gcp\e3\08-balanceador.ps1 -Fase Probar
  $env:MOOC_STUDENT_EMAIL="..."; $env:MOOC_STUDENT_PASSWORD="..."
  .\deploy\gcp\e3\08-balanceador.ps1 -Fase Sesion
  .\deploy\gcp\e3\08-balanceador.ps1 -Fase Excluir -Segundos 60
#>
param(
    [ValidateSet("Ip", "Bucket", "Backend", "Mapa", "Certificado", "Frente", "Crear", "Publicar", "Probar", "Sesion", "Excluir", "Estado", "Borrar")]
    [string]$Fase = "Estado",
    # Solicitudes de la rafaga de Probar para ver el reparto entre instancias.
    [int]$Rafaga = 30,
    # Duracion de la exclusion en la fase Excluir.
    [int]$Segundos = 60,
    # Borrar pide esta confirmacion explicita.
    [switch]$Confirmar
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "08-balanceador-$Fase"

$MIG = "mooc-api-mig"
$HC_LISTO = "mooc-api-hc-listo"
$IP_LB = "mooc-e3-lb-ip"
$BUCKET_FRONT = "mooc-e3-frontend-$P"
$BB = "mooc-e3-frontend-bb"
$BS = "mooc-api-bs"
$UM = "mooc-e3-lb"
$UM_REDIR = "mooc-e3-lb-redirect"
$CERT = "mooc-e3-cert"
$PX_HTTPS = "mooc-e3-https-proxy"
$PX_HTTP = "mooc-e3-http-proxy"
$FR_HTTPS = "mooc-e3-https"
$FR_HTTP = "mooc-e3-http"
$COMPUTE = "https://www.googleapis.com/compute/v1/projects/$P/global"

function IpLb { "$(gcloud compute addresses describe $IP_LB --global --project=$P --format='value(address)' 2>$null)".Trim() }
function Dominio {
    $ip = IpLb
    if (-not $ip) { return "" }
    return "$ip.sslip.io"
}

function Pedir($url, [string[]]$extra) {
    # curl.exe (incluido en Windows 10+) y no el alias curl de PowerShell.
    $h = & curl.exe -s -o NUL -D - --max-time 15 @extra $url 2>$null
    $linea = $h | Where-Object { $_ -match '^HTTP/' } | Select-Object -Last 1
    $codigo = if ($linea -match '^HTTP/\S+\s+(\d+)') { $Matches[1] } else { "sin respuesta" }
    $inst = $h | Where-Object { $_ -match '^x-instance-id:' } | Select-Object -Last 1
    $inst = if ($inst) { ($inst -replace '^x-instance-id:\s*', '').Trim() } else { "" }
    return [pscustomobject]@{ Codigo = $codigo; Instancia = $inst; Encabezados = $h }
}

function SaludBackend {
    $json = gcloud compute backend-services get-health $BS --global --project=$P --format=json 2>$null | Out-String
    if (-not $json.Trim()) { return @() }
    foreach ($g in @($json | ConvertFrom-Json)) {
        foreach ($s in @($g.status.healthStatus)) {
            if ($s) { [pscustomobject]@{ Instancia = ($s.instance -split '/')[-1]; Estado = $s.healthState } }
        }
    }
}

function MostrarSalud {
    $salud = @(SaludBackend)
    if ($salud.Count -eq 0) { Aviso "  El servicio de backend aun no reporta instancias (el grupo puede estar en 0 o recien conectado)."; return }
    foreach ($s in $salud) {
        $color = if ($s.Estado -eq "HEALTHY") { "Green" } else { "Yellow" }
        Write-Host ("  {0,-20} {1}" -f $s.Instancia, $s.Estado) -ForegroundColor $color
    }
}

function ImportarMapa($nombre, $yaml) {
    $f = TextoLF $yaml
    gcloud compute url-maps import $nombre --global --project=$P --source=$f --quiet --format=none
    $codigo = $LASTEXITCODE
    Remove-Item $f -Force -ErrorAction SilentlyContinue
    return ($codigo -eq 0)
}

# ---------------------------------------------------------------- fases

function FaseIp {
    Titulo "IP global del balanceador"
    if (Existe "compute addresses" $IP_LB @("--global")) {
        Write-Host "Ya existe $IP_LB"
    } else {
        gcloud compute addresses create $IP_LB --global --ip-version=IPV4 --network-tier=PREMIUM --project=$P `
            --description="Entrega 3: IP del balanceador HTTPS (frontend + API)" --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo reservar la IP."; $script:OK = $false; return }
        Ok "Reservada $IP_LB"
    }
    Ok "IP: $(IpLb)   Dominio: $(Dominio)"
    return
}

function FaseBucket {
    Titulo "Bucket del frontend gs://$BUCKET_FRONT"
    $existe = gcloud storage buckets describe "gs://$BUCKET_FRONT" --project=$P --format="value(name)" 2>&1 |
        Where-Object { $_ -isnot [System.Management.Automation.ErrorRecord] }
    if ($existe) {
        Write-Host "Ya existe el bucket."
    } else {
        gcloud storage buckets create "gs://$BUCKET_FRONT" --project=$P --location=$R --uniform-bucket-level-access --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear el bucket."; $script:OK = $false; return }
        Ok "Bucket creado en $R."
    }
    # La SPA es publica (solo HTML, JS y CSS compilados, sin secretos).
    gcloud storage buckets add-iam-policy-binding "gs://$BUCKET_FRONT" --project=$P --member=allUsers --role=roles/storage.objectViewer --format=none
    if ($LASTEXITCODE -ne 0) { Falla "No se pudo dar lectura publica (revisa si la organizacion bloquea el acceso publico)."; $script:OK = $false; return }
    # index.html tambien como pagina de error: /catalogo o /cursos/:id
    # recargados en el navegador cargan la SPA y react-router resuelve la ruta.
    gcloud storage buckets update "gs://$BUCKET_FRONT" --project=$P --web-main-page-suffix=index.html --web-error-page=index.html --format=none
    Ok "Lectura publica e index.html como pagina principal y de error."

    if (Existe "compute backend-buckets" $BB) {
        Write-Host "Ya existe el backend bucket $BB"
    } else {
        gcloud compute backend-buckets create $BB --gcs-bucket-name=$BUCKET_FRONT --project=$P `
            --description="Entrega 3: SPA del frontend" --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear el backend bucket."; $script:OK = $false; return }
        Ok "Creado el backend bucket $BB (sin CDN, no se exige)."
    }
    return
}

function FaseBackend {
    Titulo "Servicio de backend $BS"
    if (-not (Existe "compute health-checks" $HC_LISTO)) { Falla "Falta $HC_LISTO. Corre 03-grupo-api.ps1 -Fase Salud."; $script:OK = $false; return }
    if (-not (Existe "compute instance-groups managed" $MIG @("--zone=$Z"))) { Falla "Falta $MIG. Corre 03-grupo-api.ps1 -Fase Grupo."; $script:OK = $false; return }

    if (Existe "compute backend-services" $BS @("--global")) {
        Write-Host "Ya existe $BS; se reaplican drenaje, timeout y logs."
        gcloud compute backend-services update $BS --global --project=$P `
            --health-checks=$HC_LISTO --connection-draining-timeout=30 --timeout=30s `
            --enable-logging --logging-sample-rate=1 --format=none
    } else {
        gcloud compute backend-services create $BS --global --project=$P `
            --load-balancing-scheme=EXTERNAL_MANAGED --protocol=HTTP --port-name=http `
            --health-checks=$HC_LISTO --connection-draining-timeout=30 --timeout=30s `
            --enable-logging --logging-sample-rate=1 `
            --description="Entrega 3: API (mooc-api-mig) detras del balanceador HTTPS" --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear el servicio de backend."; $script:OK = $false; return }
        Ok "Creado $BS (HTTP, puerto http, $HC_LISTO, drenaje 30 s, timeout 30 s, logs 100 %)."
    }

    $grupos = gcloud compute backend-services describe $BS --global --project=$P --format="value(backends[].group)"
    if ("$grupos" -match "/instanceGroups/$MIG") {
        Write-Host "$MIG ya es backend de $BS"
    } else {
        gcloud compute backend-services add-backend $BS --global --project=$P `
            --instance-group=$MIG --instance-group-zone=$Z `
            --balancing-mode=UTILIZATION --max-utilization=0.8 --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo agregar $MIG como backend."; $script:OK = $false; return }
        Ok "Agregado $MIG (zona $Z) como backend."
    }
    Write-Host "Sin afinidad de sesion: cualquier replica atiende cualquier solicitud (sesiones en Redis)."
    return
}

function FaseMapa {
    Titulo "Mapa de URL $UM"
    if (-not (Existe "compute backend-buckets" $BB)) { Falla "Falta $BB. Corre -Fase Bucket."; $script:OK = $false; return }
    if (-not (Existe "compute backend-services" $BS @("--global"))) { Falla "Falta $BS. Corre -Fase Backend."; $script:OK = $false; return }
    $yaml = @"
name: $UM
description: "Entrega 3: /api/* a la API, el resto al frontend"
defaultService: $COMPUTE/backendBuckets/$BB
hostRules:
- hosts: ['*']
  pathMatcher: rutas
pathMatchers:
- name: rutas
  defaultService: $COMPUTE/backendBuckets/$BB
  pathRules:
  - paths: ['/api/*']
    service: $COMPUTE/backendServices/$BS
"@
    if (-not (ImportarMapa $UM $yaml)) { Falla "No se pudo crear o actualizar $UM."; $script:OK = $false; return }
    Ok "${UM}: /api/* -> $BS, /* -> $BB"

    $yamlRedir = @"
name: $UM_REDIR
description: "Entrega 3: HTTP a HTTPS"
defaultUrlRedirect:
  httpsRedirect: true
  redirectResponseCode: MOVED_PERMANENTLY_DEFAULT
"@
    if (-not (ImportarMapa $UM_REDIR $yamlRedir)) { Falla "No se pudo crear o actualizar $UM_REDIR."; $script:OK = $false; return }
    Ok "${UM_REDIR}: http -> https (301)"
    return
}

function FaseCertificado {
    Titulo "Certificado administrado $CERT"
    $dom = Dominio
    if (-not $dom) { Falla "Falta la IP. Corre -Fase Ip."; $script:OK = $false; return }
    if (Existe "compute ssl-certificates" $CERT @("--global")) {
        $actual = "$(gcloud compute ssl-certificates describe $CERT --global --project=$P --format='value(managed.domains)')".Trim()
        Write-Host "Ya existe $CERT para $actual"
        if ($actual -ne $dom) { Aviso "El certificado es para '$actual' pero el dominio actual es '$dom'. Borra con -Fase Borrar -Confirmar y vuelve a crear." }
    } else {
        gcloud compute ssl-certificates create $CERT --domains=$dom --global --project=$P --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear el certificado."; $script:OK = $false; return }
        Ok "Creado $CERT para $dom"
    }
    Write-Host "Google lo emite cuando la regla 443 existe y el dominio apunta a la IP: entre 15 y 60 minutos (PROVISIONING -> ACTIVE)."
    return
}

function FaseFrente {
    Titulo "Proxies y reglas de reenvio"
    if (-not (Existe "compute target-https-proxies" $PX_HTTPS @("--global"))) {
        gcloud compute target-https-proxies create $PX_HTTPS --global --project=$P --url-map=$UM --ssl-certificates=$CERT --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear $PX_HTTPS."; $script:OK = $false; return }
        Ok "Creado $PX_HTTPS"
    } else { Write-Host "Ya existe $PX_HTTPS" }

    if (-not (Existe "compute target-http-proxies" $PX_HTTP @("--global"))) {
        gcloud compute target-http-proxies create $PX_HTTP --global --project=$P --url-map=$UM_REDIR --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear $PX_HTTP."; $script:OK = $false; return }
        Ok "Creado $PX_HTTP"
    } else { Write-Host "Ya existe $PX_HTTP" }

    foreach ($regla in @(
        @{ Nombre = $FR_HTTPS; Puerto = 443; Proxy = "--target-https-proxy=$PX_HTTPS" },
        @{ Nombre = $FR_HTTP;  Puerto = 80;  Proxy = "--target-http-proxy=$PX_HTTP" })) {
        if (Existe "compute forwarding-rules" $regla.Nombre @("--global")) { Write-Host "Ya existe $($regla.Nombre)"; continue }
        gcloud compute forwarding-rules create $regla.Nombre --global --project=$P `
            --load-balancing-scheme=EXTERNAL_MANAGED --network-tier=PREMIUM `
            --address=$IP_LB $regla.Proxy --ports=$($regla.Puerto) --format=none
        if ($LASTEXITCODE -ne 0) { Falla "No se pudo crear $($regla.Nombre)."; $script:OK = $false; return }
        Ok "Creada $($regla.Nombre) ($(IpLb):$($regla.Puerto))"
    }
    return
}

function FasePublicar {
    $commit = Commit
    Titulo "Publicar el frontend ($commit) en gs://$BUCKET_FRONT"
    $pendientes = git status --porcelain -- frontend
    if ($pendientes) { Aviso "Hay cambios sin commit en frontend/: lo publicado no correspondera exactamente a $commit." }

    Push-Location frontend
    try {
        if (-not (Test-Path "node_modules")) {
            npm ci
            if ($LASTEXITCODE -ne 0) { Falla "npm ci fallo."; $script:OK = $false; return }
        }
        $env:APP_COMMIT = $commit
        npm run build
        if ($LASTEXITCODE -ne 0) { Falla "La compilacion fallo; no se publica nada."; $script:OK = $false; return }
    } finally {
        Remove-Item Env:\APP_COMMIT -ErrorAction SilentlyContinue
        Pop-Location
    }

    $version = [ordered]@{ commit = $commit; fecha_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ") } | ConvertTo-Json
    [System.IO.File]::WriteAllText((Join-Path (Resolve-Path "frontend/dist") "version.json"), $version)
    Write-Host "version.json: $($version -replace '\s+', ' ')"

    gcloud storage rsync frontend/dist "gs://$BUCKET_FRONT" --recursive --delete-unmatched-destination-objects --project=$P
    if ($LASTEXITCODE -ne 0) { Falla "No se pudo sincronizar con el bucket."; $script:OK = $false; return }
    # Los archivos de assets/ llevan hash en el nombre: se pueden guardar en
    # cache para siempre. index.html y version.json deben revisarse siempre
    # para que una publicacion nueva se vea al recargar.
    gcloud storage objects update "gs://$BUCKET_FRONT/assets/**" --cache-control="public, max-age=31536000, immutable" --project=$P --format=none 2>$null
    gcloud storage objects update "gs://$BUCKET_FRONT/index.html" "gs://$BUCKET_FRONT/version.json" --cache-control="no-cache, max-age=0" --project=$P --format=none
    Ok "Publicado $commit. Abre https://$(Dominio)/ y revisa la version en el pie de pagina."
    return
}

function FaseProbar {
    $dom = Dominio
    if (-not $dom) { Falla "Falta la IP. Corre -Fase Crear."; $script:OK = $false; return }
    Titulo "Balanceador $dom"

    $estadoCert = "$(gcloud compute ssl-certificates describe $CERT --global --project=$P --format='value(managed.status)' 2>$null)".Trim()
    $estadoDom = "$(gcloud compute ssl-certificates describe $CERT --global --project=$P --format='value(managed.domainStatus)' 2>$null)".Trim()
    Write-Host "Certificado: $estadoCert ($estadoDom)"

    Titulo "Salud de las instancias segun el balanceador ($HC_LISTO)"
    MostrarSalud

    Titulo "Rutas"
    $resp = Pedir "http://$dom/catalogo"
    $destino = ($resp.Encabezados | Where-Object { $_ -match '^location:' } | Select-Object -First 1)
    Write-Host ("  http://{0}/catalogo -> {1} {2}" -f $dom, $resp.Codigo, "$destino".Trim())

    if ($estadoCert -ne "ACTIVE") {
        Aviso "El certificado aun no esta ACTIVE: las pruebas por HTTPS se saltan. Vuelve a correr -Fase Probar en unos minutos."
        return
    }

    $v = & curl.exe -s --max-time 15 "https://$dom/version.json"
    Write-Host "  /version.json        -> $($v -join ' ')"
    foreach ($ruta in @("/", "/catalogo", "/api/v1/catalog", "/metrics")) {
        $resp = Pedir "https://$dom$ruta"
        $tipo = ($resp.Encabezados | Where-Object { $_ -match '^content-type:' } | Select-Object -Last 1) -replace '^content-type:\s*', ''
        $origen = if ($resp.Instancia) { "API ($($resp.Instancia))" } else { "bucket" }
        Write-Host ("  {0,-20} -> {1}  {2}  [{3}]" -f $ruta, $resp.Codigo, "$tipo".Trim(), $origen)
    }
    Write-Host "  (Las rutas de la SPA salen del bucket con index.html; /metrics no llega a la API.)"

    Titulo "Reparto: $Rafaga solicitudes a /api/v1/catalog"
    $conteo = @{}
    $errores = 0
    for ($i = 1; $i -le $Rafaga; $i++) {
        $resp = Pedir "https://$dom/api/v1/catalog"
        if ($resp.Codigo -ne "200") { $errores++ }
        $clave = if ($resp.Instancia) { $resp.Instancia } else { "(sin X-Instance-Id, codigo $($resp.Codigo))" }
        $conteo[$clave] = 1 + [int]$conteo[$clave]
    }
    $conteo.GetEnumerator() | Sort-Object Name | ForEach-Object { Write-Host ("  {0,-20} {1}" -f $_.Name, $_.Value) }
    if ($errores -gt 0) { Aviso "  $errores respuestas distintas de 200." } else { Ok "  $Rafaga/$Rafaga respuestas 200." }
    if ($conteo.Count -lt 2) { Write-Host "  Con una sola instancia sana todo va a la misma; con dos o mas debe aparecer el reparto." }
    Write-Host ""
    Write-Host "Si el profesor va a subir archivos desde este dominio, agrega el origen al CORS del bucket de medios:"
    Write-Host "  .\deploy\gcp\e3\06-cors-bucket.ps1 -Origen ""https://$dom"",""http://localhost:5173"""
    return
}

function FaseSesion {
    $dom = Dominio
    $correo = $env:MOOC_STUDENT_EMAIL
    $clave = $env:MOOC_STUDENT_PASSWORD
    if (-not $correo -or -not $clave) {
        Falla "Define `$env:MOOC_STUDENT_EMAIL y `$env:MOOC_STUDENT_PASSWORD (las cuentas de frontend/scripts/crear-cuentas.mjs)."
        $script:OK = $false; return
    }
    Titulo "Sesion y CSRF por https://$dom con $correo"
    $api = "https://$dom/api/v1"
    $jar = [System.IO.Path]::GetTempFileName()
    $cuerpo = TextoLF (@{ email = $correo; password = $clave } | ConvertTo-Json -Compress)
    $archivoResp = [System.IO.Path]::GetTempFileName()
    try {
        $h = & curl.exe -s -D - -o $archivoResp -c $jar --max-time 15 -H "Content-Type: application/json" --data-binary "@$cuerpo" "$api/auth/login"
        $codigo = if (($h | Where-Object { $_ -match '^HTTP/' } | Select-Object -Last 1) -match '^HTTP/\S+\s+(\d+)') { $Matches[1] } else { "sin respuesta" }
        Write-Host "  POST /auth/login -> $codigo"
        if ($codigo -ne "200") { Falla "  El login fallo. Revisa la cuenta (npm run cuentas con MOOC_API_URL=$api)."; $script:OK = $false; return }
        foreach ($c in @($h | Where-Object { $_ -match '^set-cookie:' })) {
            $nombre = ($c -replace '^set-cookie:\s*', '') -replace '=.*$', ''
            $banderas = @("Secure", "HttpOnly", "SameSite=Lax") | Where-Object { $c -match [regex]::Escape($_) }
            Write-Host "  Set-Cookie $nombre : $($banderas -join ', ')"
        }
        $csrf = (Get-Content $archivoResp -Raw | ConvertFrom-Json).csrf_token
        if (-not $csrf) { Aviso "  La respuesta del login no trae csrf_token." }

        $instancias = @()
        for ($i = 1; $i -le 6; $i++) {
            $resp = Pedir "$api/me" @("-b", $jar)
            $instancias += "$($resp.Codigo)@$($resp.Instancia)"
        }
        Write-Host "  GET /me x6 con la cookie -> $($instancias -join '  ')"
        Write-Host "  (Si aparecen instancias distintas, la misma sesion es valida en varias replicas.)"

        $resp = Pedir "$api/auth/logout" @("-b", $jar, "-X", "POST", "-H", "Content-Length: 0")
        Write-Host "  POST /auth/logout SIN X-CSRF-Token -> $($resp.Codigo) (esperado 403)"
        $resp = Pedir "$api/auth/logout" @("-b", $jar, "-X", "POST", "-H", "Content-Length: 0", "-H", "X-CSRF-Token: $csrf")
        Write-Host "  POST /auth/logout CON X-CSRF-Token -> $($resp.Codigo) (esperado 2xx)"
        $resp = Pedir "$api/me" @("-b", $jar)
        Write-Host "  GET /me despues del logout -> $($resp.Codigo) (esperado 401)"
    } finally {
        Remove-Item $jar, $cuerpo, $archivoResp -Force -ErrorAction SilentlyContinue
    }
    return
}

function Muestrear($fase, $segundos, $csv, $dom) {
    $fin = (Get-Date).AddSeconds($segundos)
    $proximaSalud = Get-Date
    while ((Get-Date) -lt $fin) {
        if ((Get-Date) -ge $proximaSalud) {
            $s = (@(SaludBackend) | ForEach-Object { "$($_.Instancia)=$($_.Estado)" }) -join "  "
            Write-Host ("  {0:HH:mm:ss} salud: {1}" -f (Get-Date).ToUniversalTime(), $s)
            $proximaSalud = (Get-Date).AddSeconds(10)
        }
        $vistas = @()
        for ($i = 1; $i -le 5; $i++) {
            $resp = Pedir "https://$dom/api/v1/catalog"
            $utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
            Add-Content -Path $csv -Value "$utc,$fase,$($resp.Codigo),$($resp.Instancia)"
            $vistas += "$($resp.Codigo)@$($resp.Instancia)"
        }
        Write-Host ("  {0:HH:mm:ss} {1}: {2}" -f (Get-Date).ToUniversalTime(), $fase, ($vistas -join " "))
        Start-Sleep -Seconds 1
    }
}

function FaseExcluir {
    $dom = Dominio
    Titulo "Exclusion de una instancia que falla el health check"
    $sanas = @(SaludBackend | Where-Object { $_.Estado -eq "HEALTHY" })
    if ($sanas.Count -lt 2) {
        Falla "Hay $($sanas.Count) instancia(s) sana(s); se necesitan 2 para ver que el trafico se va a la otra."
        Write-Host "Corre esta fase durante el Escenario 3A, cuando el autoscaler haya subido el grupo a 2 o mas."
        $script:OK = $false; return
    }
    $vm = $sanas[0].Instancia
    $csv = "capacity-planning/results/e3-3a-exclusion-$(Get-Date -Format 'yyyyMMdd-HHmmss').csv"
    Set-Content -Path $csv -Value "utc,fase,codigo,instancia"
    Write-Host "Instancia a excluir: $vm. Se bloquea su salida a Redis (tcp 6379): /health/ready responde 503,"
    Write-Host "pero /health sigue en 200, asi que la autorreparacion NO la recrea. Solo el balanceador la saca."

    Muestrear "antes" 15 $csv $dom
    $bloqueo = Ssh $vm "sudo iptables -I OUTPUT -p tcp --dport 6379 -j REJECT && echo BLOQUEADO"
    if (-not ($bloqueo -match "BLOQUEADO")) { Falla "No se pudo bloquear Redis en $vm. Salida: $bloqueo"; $script:OK = $false; return }
    Aviso ("  {0:HH:mm:ss} UTC: Redis bloqueado en {1}" -f (Get-Date).ToUniversalTime(), $vm)
    try {
        Muestrear "excluida" $Segundos $csv $dom
    } finally {
        $quitar = Ssh $vm "while sudo iptables -D OUTPUT -p tcp --dport 6379 -j REJECT 2>/dev/null; do :; done; echo DESBLOQUEADO"
        Aviso ("  {0:HH:mm:ss} UTC: Redis desbloqueado en {1} ({2})" -f (Get-Date).ToUniversalTime(), $vm, ($quitar -join ' '))
    }
    Muestrear "recuperada" 40 $csv $dom

    Titulo "Resumen (respuestas por instancia en cada fase)"
    Import-Csv $csv | Group-Object -Property fase, instancia | ForEach-Object { Write-Host ("  {0,-40} {1}" -f $_.Name, $_.Count) }
    Ok "Evidencia en $csv"
    return
}

function FaseEstado {
    Titulo "Balanceador"
    Write-Host "IP: $(IpLb)   Dominio: $(Dominio)"
    gcloud compute forwarding-rules list --global --project=$P --filter="name~^mooc-e3-" --format="table(name,IPAddress,portRange,loadBalancingScheme,target.basename())"
    gcloud compute ssl-certificates list --global --project=$P --filter="name=$CERT" --format="table(name,managed.status,managed.domains.list(),expireTime)"
    gcloud compute backend-services describe $BS --global --project=$P --format="yaml(backends[].group.basename(),healthChecks[].basename(),connectionDraining,timeoutSec,logConfig)" 2>$null
    Titulo "Salud"
    MostrarSalud
    Titulo "Frontend publicado"
    & curl.exe -s --max-time 10 "https://storage.googleapis.com/$BUCKET_FRONT/version.json"
    Write-Host ""
    return
}

function FaseBorrar {
    if (-not $Confirmar) { Falla "Borrar quita el balanceador completo. Repite con -Confirmar."; $script:OK = $false; return }
    Titulo "Borrando el balanceador (se conservan $IP_LB y el bucket)"
    $pasos = @(
        @("compute forwarding-rules", $FR_HTTPS, "--global"),
        @("compute forwarding-rules", $FR_HTTP, "--global"),
        @("compute target-https-proxies", $PX_HTTPS, "--global"),
        @("compute target-http-proxies", $PX_HTTP, "--global"),
        @("compute url-maps", $UM, "--global"),
        @("compute url-maps", $UM_REDIR, "--global"),
        @("compute ssl-certificates", $CERT, "--global"),
        @("compute backend-services", $BS, "--global"),
        @("compute backend-buckets", $BB, $null))
    foreach ($paso in $pasos) {
        $extra = if ($paso[2]) { @($paso[2]) } else { @() }
        if (-not (Existe $paso[0] $paso[1] $extra)) { Write-Host "No existe $($paso[1])"; continue }
        $argumentos = @($paso[0].Split(" ")) + @("delete", $paso[1], "--project=$P", "--quiet") + $extra
        & gcloud @argumentos 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) { Ok "Borrado $($paso[1])" } else { Falla "No se pudo borrar $($paso[1])" }
    }
    return
}

$script:OK = $true
switch ($Fase) {
    "Ip"          { FaseIp }
    "Bucket"      { FaseBucket }
    "Backend"     { FaseBackend }
    "Mapa"        { FaseMapa }
    "Certificado" { FaseCertificado }
    "Frente"      { FaseFrente }
    "Crear" {
        foreach ($f in @("FaseIp", "FaseBucket", "FaseBackend", "FaseMapa", "FaseCertificado", "FaseFrente")) {
            & $f
            if (-not $script:OK) { Falla "Se detuvo en $f."; break }
        }
        if ($script:OK) {
            Write-Host ""
            Ok "Balanceador creado en https://$(Dominio)/"
            Write-Host "Siguiente: -Fase Publicar y, cuando el certificado este ACTIVE, -Fase Probar."
        }
    }
    "Publicar"    { FasePublicar }
    "Probar"      { FaseProbar }
    "Sesion"      { FaseSesion }
    "Excluir"     { FaseExcluir }
    "Estado"      { FaseEstado }
    "Borrar"      { FaseBorrar }
}

CerrarRegistro
