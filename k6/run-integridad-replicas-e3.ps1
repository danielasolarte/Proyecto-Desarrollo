param(
    [string]$BaseUrl = "http://host.docker.internal:8080/api/v1",

    [string]$StudentEmail = "camila.student@mooc.test",

    [string]$StudentPassword = "Student12345",

    [string]$QuizId = "",

    [string]$QuestionId = "",

    [string]$CorrectOptionId = "",

    [bool]$RequireTwoInstances = $false,

    [int]$ReplicaProbes = 20
)

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "========================================"
Write-Host "INTEGRIDAD ENTRE REPLICAS - ENTREGA 3"
Write-Host "========================================"
Write-Host "Base URL:              $BaseUrl"
Write-Host "Student:               $StudentEmail"
Write-Host "Require two instances: $RequireTwoInstances"
Write-Host "Replica probes:         $ReplicaProbes"
Write-Host "========================================"
Write-Host ""

if ([string]::IsNullOrWhiteSpace($QuizId)) {
    throw "Debes proporcionar -QuizId"
}

if ([string]::IsNullOrWhiteSpace($QuestionId)) {
    throw "Debes proporcionar -QuestionId"
}

if ([string]::IsNullOrWhiteSpace($CorrectOptionId)) {
    throw "Debes proporcionar -CorrectOptionId"
}

$env:BASE_URL = $BaseUrl
$env:STUDENT_EMAIL = $StudentEmail
$env:STUDENT_PASSWORD = $StudentPassword
$env:QUIZ_ID = $QuizId
$env:QUESTION_ID = $QuestionId
$env:CORRECT_OPTION_ID = $CorrectOptionId
$env:REQUIRE_TWO_INSTANCES = $RequireTwoInstances.ToString().ToLower()
$env:REPLICA_PROBES = $ReplicaProbes.ToString()

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"

$resultDir = Join-Path $PSScriptRoot "results"

if (-not (Test-Path $resultDir)) {
    New-Item `
        -ItemType Directory `
        -Path $resultDir `
        -Force | Out-Null
}

$resultFile = Join-Path `
    $resultDir `
    "integridad-replicas-e3-$timestamp.json"

Write-Host "Resultado JSON:"
Write-Host $resultFile
Write-Host ""

try {
    docker run --rm `
        --network host `
        -v "${PWD}:/project" `
        -w /project `
        -e BASE_URL=$env:BASE_URL `
        -e STUDENT_EMAIL=$env:STUDENT_EMAIL `
        -e STUDENT_PASSWORD=$env:STUDENT_PASSWORD `
        -e QUIZ_ID=$env:QUIZ_ID `
        -e QUESTION_ID=$env:QUESTION_ID `
        -e CORRECT_OPTION_ID=$env:CORRECT_OPTION_ID `
        -e REQUIRE_TWO_INSTANCES=$env:REQUIRE_TWO_INSTANCES `
        -e REPLICA_PROBES=$env:REPLICA_PROBES `
        grafana/k6 run `
        k6/integridad-replicas-e3.js `
        --out "json=/project/k6/results/integridad-replicas-e3-$timestamp.json"

    if ($LASTEXITCODE -ne 0) {
        throw "k6 terminó con código $LASTEXITCODE"
    }

    Write-Host ""
    Write-Host "========================================"
    Write-Host "PRUEBA FINALIZADA"
    Write-Host "========================================"
    Write-Host "Resultado guardado en:"
    Write-Host $resultFile
}
finally {
    Remove-Item Env:\BASE_URL -ErrorAction SilentlyContinue
    Remove-Item Env:\STUDENT_EMAIL -ErrorAction SilentlyContinue
    Remove-Item Env:\STUDENT_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:\QUIZ_ID -ErrorAction SilentlyContinue
    Remove-Item Env:\QUESTION_ID -ErrorAction SilentlyContinue
    Remove-Item Env:\CORRECT_OPTION_ID -ErrorAction SilentlyContinue
    Remove-Item Env:\REQUIRE_TWO_INSTANCES -ErrorAction SilentlyContinue
    Remove-Item Env:\REPLICA_PROBES -ErrorAction SilentlyContinue
}