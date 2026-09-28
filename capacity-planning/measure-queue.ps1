param(
    [string]$ComposeFile = "deploy/docker-compose.worker.yml",
    [string]$Queue = "media",
    [string]$OutputFile = "capacity-planning/results/queue-metrics.csv"
)

$timestamp = (Get-Date).ToUniversalTime().ToString("o")

$pending = docker compose -f $ComposeFile exec -T redis redis-cli LLEN "asynq:{$Queue}:pending"
$active = docker compose -f $ComposeFile exec -T redis redis-cli LLEN "asynq:{$Queue}:active"
$scheduled = docker compose -f $ComposeFile exec -T redis redis-cli ZCARD "asynq:{$Queue}:scheduled"
$retry = docker compose -f $ComposeFile exec -T redis redis-cli ZCARD "asynq:{$Queue}:retry"
$archived = docker compose -f $ComposeFile exec -T redis redis-cli ZCARD "asynq:{$Queue}:archived"
$completed = docker compose -f $ComposeFile exec -T redis redis-cli ZCARD "asynq:{$Queue}:completed"

Write-Host ""
Write-Host "=== ASYNQ QUEUE METRICS ==="
Write-Host "Timestamp UTC: $timestamp"
Write-Host "Queue: $Queue"
Write-Host "Pending:   $pending"
Write-Host "Active:    $active"
Write-Host "Scheduled: $scheduled"
Write-Host "Retry:     $retry"
Write-Host "Archived:  $archived"
Write-Host "Completed: $completed"
Write-Host ""

$directory = Split-Path $OutputFile -Parent

if (!(Test-Path $directory)) {
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
}

$row = [PSCustomObject]@{
    timestamp_utc = $timestamp
    queue         = $Queue
    pending       = [int]$pending
    active        = [int]$active
    scheduled     = [int]$scheduled
    retry         = [int]$retry
    archived      = [int]$archived
    completed     = [int]$completed
}

if (Test-Path $OutputFile) {
    $row | Export-Csv -Path $OutputFile -Append -NoTypeInformation
}
else {
    $row | Export-Csv -Path $OutputFile -NoTypeInformation
}

Write-Host "Metrics saved to: $OutputFile"