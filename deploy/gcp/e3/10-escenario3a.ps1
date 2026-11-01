<#
Entrega 3 - Escenario 3A: linea de tiempo del autoscaling de la API (Samara).

Fases:

  Monitor   Cada -Intervalo s (15 por defecto), durante -Minutos (45), escribe
            una fila en capacity-planning/results/e3-3a-timeline-<fecha>.csv:
              utc, tamano objetivo del grupo, tamano recomendado por el
              autoscaler, instancias (nombre:estado:accion), instancias sanas
              en el balanceador, reparto de 5 solicitudes por X-Instance-Id y
              CPU promedio del ultimo minuto por instancia (Cloud Monitoring).
            Marca como EVENTO cada cambio de tamano, instancia nueva o
            retirada y cambio de salud en el balanceador.
  Cpu       Exporta la serie completa de CPU (1 min) de las instancias
            mooc-api-* entre -Desde y -Hasta (UTC; por defecto la ultima hora)
            a capacity-planning/results/e3-3a-cpu-<fecha>.csv.
  Politica  Muestra la politica de autoscaling vigente (para el informe).

Uso (el monitor se arranca ANTES de la carga, en otra terminal):
  .\deploy\gcp\e3\10-escenario3a.ps1 -Fase Monitor
  .\k6\run-escenario3a.ps1 ...                       (en otra terminal)
  .\deploy\gcp\e3\10-escenario3a.ps1 -Fase Cpu        (al final)
#>
param(
    [ValidateSet("Monitor", "Cpu", "Politica")]
    [string]$Fase = "Monitor",
    [int]$Minutos = 45,
    [int]$Intervalo = 15,
    [string]$Desde,
    [string]$Hasta
)

$ErrorActionPreference = "Continue"
. (Join-Path $PSScriptRoot "comun.ps1")
IniciarRegistro "10-escenario3a-$Fase"

$MIG = "mooc-api-mig"
$BS = "mooc-api-bs"
$dominio = "$(gcloud compute addresses describe mooc-e3-lb-ip --global --project=$P --format='value(address)' 2>$null)".Trim() + ".sslip.io"
New-Item -ItemType Directory -Force "capacity-planning/results" | Out-Null

function G {
    # gcloud con un reintento. Si falla dos veces, muestra el error real
    # (antes se descartaba y el monitor mostraba columnas vacias).
    for ($intento = 1; $intento -le 2; $intento++) {
        $todo = & gcloud @args 2>&1
        $errores = @($todo | Where-Object { $_ -is [System.Management.Automation.ErrorRecord] } | ForEach-Object { "$($_.Exception.Message)" } | Where-Object { $_ -match '\S' })
        $salida = @($todo | Where-Object { $_ -isnot [System.Management.Automation.ErrorRecord] })
        if ($LASTEXITCODE -eq 0) { return $salida }
        Start-Sleep -Seconds 2
    }
    $msg = ($errores | Where-Object { $_ -match 'ERROR' } | Select-Object -First 1)
    if (-not $msg) { $msg = $errores | Select-Object -First 1 }
    Write-Host "          gcloud fallo ($($args[0..2] -join ' ')): $msg" -ForegroundColor Red
    return @()
}

function Autoscaler {
    $url = "$(G compute instance-groups managed describe $MIG --zone=$Z --project=$P --format='value(status.autoscaler)')".Trim()
    if (-not $url) { return $null }
    return ($url -split '/')[-1]
}

function SaludLb {
    $json = G compute backend-services get-health $BS --global --project=$P --format=json | Out-String
    $estados = @{}
    if (-not $json.Trim()) { return $estados }
    foreach ($g in @($json | ConvertFrom-Json)) {
        foreach ($s in @($g.status.healthStatus)) {
            if ($s) { $estados[($s.instance -split '/')[-1]] = $s.healthState }
        }
    }
    return $estados
}

function Reparto {
    $vistos = @()
    for ($i = 1; $i -le 5; $i++) {
        $h = & curl.exe -s -o NUL -D - --max-time 10 "https://$dominio/api/v1/catalog" 2>$null
        $linea = $h | Where-Object { $_ -match '^x-instance-id:' } | Select-Object -Last 1
        $vistos += if ($linea) { ($linea -replace '^x-instance-id:\s*', '').Trim() } else { "error" }
    }
    return (($vistos | Group-Object | ForEach-Object { "$($_.Name)=$($_.Count)" }) -join " ")
}

function SerieCpu($desdeUtc, $hastaUtc) {
    # Cloud Monitoring API: compute.googleapis.com/instance/cpu/utilization
    # (0..1), promediada por minuto, solo instancias mooc-api-*.
    $token = "$(gcloud auth print-access-token 2>$null)".Trim()
    if (-not $token) { return @() }
    $filtro = 'metric.type="compute.googleapis.com/instance/cpu/utilization" AND metric.labels.instance_name=starts_with("mooc-api")'
    $q = "filter=$([uri]::EscapeDataString($filtro))" +
         "&interval.startTime=$($desdeUtc.ToString('yyyy-MM-ddTHH:mm:ssZ'))" +
         "&interval.endTime=$($hastaUtc.ToString('yyyy-MM-ddTHH:mm:ssZ'))" +
         "&aggregation.alignmentPeriod=60s&aggregation.perSeriesAligner=ALIGN_MEAN"
    try {
        $respuesta = Invoke-RestMethod -Uri "https://monitoring.googleapis.com/v3/projects/$P/timeSeries?$q" -Headers @{ Authorization = "Bearer $token" } -TimeoutSec 20
    } catch { return @() }
    foreach ($serie in @($respuesta.timeSeries)) {
        if (-not $serie) { continue }
        $nombre = $serie.metric.labels.instance_name
        foreach ($punto in @($serie.points)) {
            [pscustomobject]@{
                utc       = ([datetime]$punto.interval.endTime).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
                instancia = $nombre
                cpu_pct   = [math]::Round([double]$punto.value.doubleValue * 100, 1)
            }
        }
    }
}

switch ($Fase) {

"Politica" {
    Titulo "Politica de autoscaling de $MIG"
    $as = Autoscaler
    gcloud compute instance-groups managed describe $MIG --zone=$Z --project=$P --format="yaml(targetSize,autoHealingPolicies,namedPorts,autoscaler.name,autoscaler.autoscalingPolicy,autoscaler.recommendedSize,autoscaler.status)"
}

"Monitor" {
    $as = Autoscaler
    if (-not $as) { Falla "El grupo $MIG no tiene autoscaler. Corre 03-grupo-api.ps1 -Fase Encender."; break }
    $csv = "capacity-planning/results/e3-3a-timeline-$(Get-Date -Format 'yyyyMMdd-HHmmss').csv"
    Set-Content -Path $csv -Value "utc,tamano_objetivo,tamano_recomendado,estado_autoscaler,instancias,sanas_lb,reparto,cpu_ultimo_min,evento"
    Titulo "Monitor 3A: $MIG (autoscaler $as), cada $Intervalo s durante $Minutos min"
    Write-Host "Arranca la carga (k6) en otra terminal cuando veas las primeras filas."
    Write-Host "CSV: $csv"
    Write-Host ""

    $fin = (Get-Date).AddMinutes($Minutos)
    $previo = $null
    $cpuTexto = ""
    $vuelta = 0
    while ((Get-Date) -lt $fin) {
        $utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        # Una sola consulta al grupo: tamano objetivo y, del autoscaler
        # asociado, el tamano recomendado y su estado.
        $info = "$(G compute instance-groups managed describe $MIG --zone=$Z --project=$P --format='value(targetSize,autoscaler.recommendedSize,autoscaler.status)')".Trim() -split "\s+"
        $objetivo = $info[0]
        $recomendado = if ($info.Count -gt 1) { $info[1] } else { "" }
        $estadoAs = if ($info.Count -gt 2) { $info[2] } else { "" }
        $lista = @(G compute instance-groups managed list-instances $MIG --zone=$Z --project=$P --format="value(name.basename(),status,currentAction)" |
            Where-Object { $_ } | ForEach-Object { ($_ -split "\s+") -join ":" })
        $salud = SaludLb
        $sanas = @($salud.GetEnumerator() | Where-Object { $_.Value -eq "HEALTHY" } | ForEach-Object { $_.Key } | Sort-Object)
        $reparto = Reparto
        if ($vuelta % [math]::Max(1, [int](60 / $Intervalo)) -eq 0) {
            $ahora = (Get-Date).ToUniversalTime()
            $ultimos = @(SerieCpu $ahora.AddMinutes(-4) $ahora | Group-Object instancia | ForEach-Object {
                $ultimoPunto = $_.Group | Sort-Object utc | Select-Object -Last 1; "$($_.Name)=$($ultimoPunto.cpu_pct)%" })
            $cpuTexto = $ultimos -join " "
        }
        $vuelta++

        # Eventos: cambios respecto de la fila anterior.
        $eventos = @()
        $nombres = @($lista | ForEach-Object { ($_ -split ":")[0] })
        $consultaOk = [bool]$objetivo
        if (-not $consultaOk) { $eventos += "consulta fallida (se ignora esta fila para eventos)" }
        if ($previo -and $consultaOk) {
            if ($objetivo -ne $previo.objetivo) { $eventos += "tamano $($previo.objetivo)->$objetivo" }
            if ($recomendado -ne $previo.recomendado) { $eventos += "recomendado $($previo.recomendado)->$recomendado" }
            foreach ($n in $nombres) { if ($previo.nombres -notcontains $n) { $eventos += "creada $n" } }
            foreach ($n in $previo.nombres) { if ($nombres -notcontains $n) { $eventos += "retirada $n" } }
            foreach ($n in $sanas) { if ($previo.sanas -notcontains $n) { $eventos += "sana en LB $n" } }
            foreach ($n in $previo.sanas) { if ($sanas -notcontains $n) { $eventos += "fuera del LB $n" } }
        } elseif ($consultaOk -and -not $previo) { $eventos += "inicio" }
        $evento = $eventos -join "; "

        $fila = @($utc, $objetivo, $recomendado, $estadoAs, ($lista -join " "), ($sanas -join " "), $reparto, $cpuTexto, $evento) |
            ForEach-Object { '"' + ("$_" -replace '"', "'") + '"' }
        Add-Content -Path $csv -Value ($fila -join ",")

        $color = if ($eventos.Count -gt 0 -and $evento -ne "inicio") { "Yellow" } else { "Gray" }
        Write-Host ("{0}  grupo={1} rec={2} sanas={3}  {4}  cpu[{5}]" -f $utc.Substring(11, 8), $objetivo, $recomendado, $sanas.Count, $reparto, $cpuTexto) -ForegroundColor $color
        if ($color -eq "Yellow") { Write-Host "          EVENTO: $evento" -ForegroundColor Yellow }

        if ($consultaOk) { $previo = @{ objetivo = $objetivo; recomendado = $recomendado; nombres = $nombres; sanas = $sanas } }
        Start-Sleep -Seconds $Intervalo
    }
    Ok "Monitor terminado. Linea de tiempo en $csv"
    Write-Host "Siguiente: .\deploy\gcp\e3\10-escenario3a.ps1 -Fase Cpu"
}

"Cpu" {
    $h = if ($Hasta) { ([datetime]$Hasta).ToUniversalTime() } else { (Get-Date).ToUniversalTime() }
    $d = if ($Desde) { ([datetime]$Desde).ToUniversalTime() } else { $h.AddMinutes(-60) }
    Titulo "CPU de mooc-api-* entre $($d.ToString('u')) y $($h.ToString('u'))"
    $serie = @(SerieCpu $d $h | Sort-Object utc, instancia)
    if ($serie.Count -eq 0) { Falla "Cloud Monitoring no devolvio datos (revisa permisos de Monitoring Viewer o el rango)."; break }
    $csv = "capacity-planning/results/e3-3a-cpu-$(Get-Date -Format 'yyyyMMdd-HHmmss').csv"
    $serie | Export-Csv -Path $csv -NoTypeInformation -Encoding UTF8
    $serie | Group-Object instancia | ForEach-Object {
        $max = ($_.Group | Measure-Object cpu_pct -Maximum).Maximum
        Write-Host ("  {0,-16} {1,3} puntos  maximo {2} %" -f $_.Name, $_.Count, $max)
    }
    Ok "Serie de CPU en $csv"
}
}

CerrarRegistro
