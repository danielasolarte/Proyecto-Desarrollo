$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force "k6/results" | Out-Null

$levels = @(100, 250, 500, 1000, 2000)

foreach ($level in $levels) {

    Write-Host ""
    Write-Host "============================================"
    Write-Host " PRUEBA DE CAPACIDAD: $level VUs"
    Write-Host "============================================"
    Write-Host ""

    docker run --rm `
      -v "${PWD}:/project" `
      -w /project `
      grafana/k6 run `
      -e PROFILE=capacity `
      -e TOTAL_VUS=$level `
      -e BASE_URL=http://host.docker.internal:8080/api/v1 `
      --summary-export "/project/k6/results/capacity-$level.json" `
      k6/full-project.js

    Write-Host ""
    Write-Host "Finalizada prueba de $level VUs."
    Write-Host ""

    if ($level -ne 2000) {
        Write-Host "Esperando 20 segundos antes de la siguiente carga..."
        Start-Sleep -Seconds 20
    }
}

Write-Host ""
Write-Host "============================================"
Write-Host " TODAS LAS PRUEBAS FINALIZARON"
Write-Host "============================================"
Write-Host ""

Write-Host "Resultados:"
Write-Host "k6/results/capacity-100.json"
Write-Host "k6/results/capacity-250.json"
Write-Host "k6/results/capacity-500.json"
Write-Host "k6/results/capacity-1000.json"
Write-Host "k6/results/capacity-2000.json"