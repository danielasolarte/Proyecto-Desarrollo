param(
    [int]$IntervalSeconds = 5,
    [int]$DurationSeconds = 60
)

$start = Get-Date
$end = $start.AddSeconds($DurationSeconds)

Write-Host ""
Write-Host "Starting queue monitoring..."
Write-Host "Interval: $IntervalSeconds seconds"
Write-Host "Duration: $DurationSeconds seconds"
Write-Host ""

while ((Get-Date) -lt $end) {
    .\capacity-planning\measure-queue.ps1

    Start-Sleep -Seconds $IntervalSeconds
}

Write-Host ""
Write-Host "Queue monitoring finished."