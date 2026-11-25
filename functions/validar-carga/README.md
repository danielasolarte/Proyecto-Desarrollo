# Cloud Function `mooc-validar-carga` (Entrega 5)

Valida cada carga antes del antivirus: SHA-256 real, MIME real y firma ZIP
de las presentaciones. Reemplaza esa parte de `scan_antivirus` en el worker,
que ya no la hace.

- Contrato, idempotencia y reintentos: `docs/entrega5/contrato-validacion.md`
- Infraestructura (función, disparador de Eventarc, permisos):
  `infra/terraform/funcion.tf`
- Terraform empaqueta esta carpeta como zip (sin `*_test.go` ni este
  README) y la sube al bucket de fuentes; Cloud Build la compila con el
  runtime `go125`.

## Archivos

| Archivo | Qué tiene |
|---|---|
| `funcion.go` | Punto de entrada `ValidarCarga`, configuración y el flujo de la sección 2 del contrato |
| `validacion.go` | Las reglas: lectura en flujo, SHA-256, MIME y firma ZIP |
| `repositorio.go` | Las consultas a Postgres (copiadas de `internal/media/postgres`) |
| `servicios.go` | Cloud Storage y Pub/Sub por API REST, con el token de `sa-funcion-validar` |

No importa el módulo del backend ni los clientes oficiales de Google porque
piden Go 1.26 y el runtime es `go125` (sección 6 del contrato). `go.mod` debe
quedar en `go 1.25.x`. Si `go mod tidy` lo sube, alguna dependencia nueva
pide una versión más reciente.

## Variables

Las pone Terraform: `DATABASE_URL` (secreto), `S3_BUCKET`, `PUBSUB_PROJECT`,
`PUBSUB_TOPIC` y `DB_MAX_CONNS`.

## Pruebas

```bash
cd functions/validar-carga
go test -race ./...
```

Para compilar con la misma versión del runtime: `GOTOOLCHAIN=go1.25.0 go test ./...`.

Las consultas SQL se prueban contra un Postgres desechable con las
migraciones del repo (sin la variable, esa prueba se salta):

```bash
docker run -d --rm --name pg-validar -e POSTGRES_PASSWORD=prueba -p 55432:5432 postgres:16
for f in ../../migrations/*.up.sql; do docker exec -i pg-validar psql -q -U postgres < "$f"; done
# la prueba crea recursos sin curso ni unidad: se apagan las llaves foráneas
docker exec pg-validar psql -q -U postgres -c "ALTER DATABASE postgres SET session_replication_role = replica;"
VALIDAR_PG_URL='postgres://postgres:prueba@localhost:55432/postgres?sslmode=disable' go test -run Postgres -v ./...
docker rm -f pg-validar
```

## Registros

Cada invocación deja un registro JSON con `severity`, `job_id`,
`message_id`, `intento` y `estado_final` (`done`, `reintento`, `ocupado`,
`ya_terminado`, `dead_letter` o `descartado`). Sirven como evidencia de la
cadena Pub/Sub → función → resultado:

```bash
gcloud functions logs read mooc-validar-carga --region=<región> --gen2 --limit=50
```
