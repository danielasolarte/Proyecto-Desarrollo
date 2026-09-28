# Escenario 1 - niveles de carga (uno por corrida, con pausa entre corridas).
# Uso desde la raiz del repo:
#   .\k6\run-escenario1.ps1 -Tag local -Levels 10,25,50
#   .\k6\run-escenario1.ps1 -Tag aws -BaseUrl https://TU-DOMINIO/api/v1 -Levels 10,25,50,100,150
# El primer nivel de la lista es la linea base.
param(
    [string]$BaseUrl = "http://host.docker.internal:8080/api/v1",
    [int[]]$Levels = @(10, 25, 50, 100),
    [int]$WarmupS = 30,
    [int]$HoldS = 180,
    [int]$PauseBetweenS = 60,
    [string]$Tag = "local",
    [string]$K6Image = "grafana/k6:latest",
    [switch]$SaveTimeSeries
)

# Continue (no Stop): k6 escribe sus avisos por stderr y con Stop PowerShell 5.1
# cortaria el script al primer aviso, aunque la prueba siga sana.
$ErrorActionPreference = "Continue"
New-Item -ItemType Directory -Force "k6/results" | Out-Null

Write-Host "Version de k6 usada:"
docker run --rm $K6Image version

for ($i = 0; $i -lt $Levels.Count; $i++) {
    $level = $Levels[$i]
    $prefix = "k6/results/e1-$Tag-$level"

    Write-Host ""
    Write-Host "=== Escenario 1 | nivel $level VUs | destino $BaseUrl ==="
    Write-Host ""

    $extra = @()
    if ($SaveTimeSeries) { $extra += @("--out", "csv=/project/$prefix.csv.gz") }

    docker run --rm `
        -v "${PWD}:/project" `
        -w /project `
        $K6Image run `
        -e PROFILE=level `
        -e TOTAL_VUS=$level `
        -e WARMUP_S=$WarmupS `
        -e HOLD_S=$HoldS `
        -e BASE_URL=$BaseUrl `
        --summary-export "/project/$prefix.json" `
        --summary-trend-stats "avg,min,med,p(90),p(95),p(99),max" `
        @extra `
        k6/escenario1.js 2>&1 | Tee-Object -FilePath "$prefix.log"

    if ($i -lt $Levels.Count - 1) {
        Write-Host "Pausa de $PauseBetweenS s para que la cola y las conexiones se estabilicen..."
        Start-Sleep -Seconds $PauseBetweenS
    }
}

Write-Host ""
Write-Host "Listo. Resultados en k6/results/e1-$Tag-*.json y .log"
