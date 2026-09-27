param(
    [Parameter(Mandatory = $true)]
    [string]$DatabaseUrl
)

$ErrorActionPreference = "Stop"

Write-Host "Running database migrations..."

migrate `
    -path "migrations" `
    -database $DatabaseUrl `
    up

if ($LASTEXITCODE -ne 0) {
    throw "Database migration failed."
}

Write-Host "Migrations completed successfully."

Write-Host "Current migration version:"

migrate `
    -path "migrations" `
    -database $DatabaseUrl `
    version