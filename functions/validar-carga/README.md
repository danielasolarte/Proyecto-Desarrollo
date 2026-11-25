# Cloud Function `mooc-validar-carga` (Entrega 5)

Valida cada carga antes del antivirus: SHA-256 real, MIME real y firma ZIP de las presentaciones. Esta parte la hacía el worker dentro de `scan_antivirus` hasta la E4; ahora el worker solo pasa ClamAV.

- Contrato, idempotencia y reintentos: `docs/entrega5/contrato-validacion.md`.
- Infraestructura (función, disparador de Eventarc, permisos): `infra/terraform/funcion.tf`.
- Terraform empaqueta esta carpeta como zip y Cloud Build la compila con el runtime `go125`. El punto de entrada es `ValidarCarga`.

## Archivos

- `funcion.go`: punto de entrada (CloudEvent de Pub/Sub) y configuración desde las variables de entorno.
- `proceso.go`: el flujo del trabajo (tomar, validar, guardar, encadenar, reintentos).
- `validacion.go`: las reglas de la validación (las mismas del worker de la E4).
- `postgres.go`: las consultas sobre `media_jobs`, `media_assets` y `resources` (las mismas del backend).
- `gcp.go`: Cloud Storage y Pub/Sub por su API REST con el token de la cuenta de servicio `sa-funcion-validar`. Por eso la función no necesita las llaves HMAC del bucket.

## Pruebas

```bash
cd functions/validar-carga
go test -cover ./...
```

La CI las corre en cada PR (`backend-ci`), junto con `go mod verify`.
