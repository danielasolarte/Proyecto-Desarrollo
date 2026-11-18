# Cloud Function `mooc-validar-carga` (Entrega 5)

Valida cada carga antes del antivirus: SHA-256 real, MIME real y firma ZIP
de las presentaciones. Reemplaza esa parte de `scan_antivirus` en el worker.

- Contrato, idempotencia y reintentos: `docs/entrega5/contrato-validacion.md`
- Infraestructura (función, disparador de Eventarc, permisos):
  `infra/terraform/funcion.tf`
- Terraform empaqueta esta carpeta como zip (sin `*_test.go` ni este
  README) y la sube al bucket de fuentes; Cloud Build la compila con el
  runtime `go125`.

Pruebas locales: `go test ./...` dentro de esta carpeta.
