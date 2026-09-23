param(
  [Parameter(Mandatory=$true)][string]$DatabaseUrl
)

# Aplica las migraciones de migrations/ contra la base de datos indicada en $DatabaseUrl.
# Se corre desde el Web Server, apuntando a Cloud SQL (no hay acceso directo desde el equipo local).
#
# Ejemplo con IP publica + SSL:
#   .\deploy\run-migrations.ps1 -DatabaseUrl "postgres://usuario:clave@<IP-PUBLICA-CLOUD-SQL>:5432/mooc?sslmode=require"
#
# Ejemplo con Cloud SQL Auth Proxy corriendo en localhost:5432:
#   .\deploy\run-migrations.ps1 -DatabaseUrl "postgres://usuario:clave@127.0.0.1:5432/mooc?sslmode=disable"

docker run --rm -v "${PWD}/migrations:/migrations" migrate/migrate `
  -path=/migrations `
  -database "$DatabaseUrl" `
  up