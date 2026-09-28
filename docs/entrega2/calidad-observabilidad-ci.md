# Calidad, observabilidad y CI

Este documento concentra la evidencia pedida para la Entrega 2 sobre carga,
flujos E2E, accesibilidad, observabilidad, alertas, CI y recuperacion.

## Carga de Etapa 1

Evidencia principal:

- Script: `k6/escenario1.js`.
- Runner: `k6/run-escenario1.ps1`.
- Resultados: `k6/results/e1-gcp-*.json`.
- Informe: `capacity-planning/pruebas_de_carga_entrega2.md`.

Resultados reportados en GCP:

| Nivel | p95 | Fallos HTTP | Disponibilidad aproximada |
|---:|---:|---:|---:|
| 10 VUs | 246 ms | 0.186 % | 99.814 % |
| 25 VUs | 220 ms | 0.198 % | 99.802 % |
| 50 VUs | 225 ms | 0.198 % | 99.802 % |
| 100 VUs | 1 049 ms | 1.763 % | 98.237 % |

El nivel estable documentado para la configuracion actual es 50 VUs. En 100
VUs se observa degradacion y queda registrado como limite operativo para la
sustentacion.

## Nueve flujos E2E

El archivo `k6/e2e-nine-flows.js` ejecuta una prueba smoke E2E sobre nueve
flujos:

1. Healthcheck publico.
2. Endpoint `/metrics` en formato Prometheus.
3. Headers de seguridad en la entrada publica.
4. Login autenticado.
5. Catalogo, detalle, preview e inscripcion.
6. Progreso: open, heartbeat y complete.
7. Quiz con submit idempotente.
8. Proteccion de roles para rutas administrativas.
9. Verificacion publica de insignias con respuesta controlada.

Ejecucion local:

```powershell
docker run --rm `
  -v "${PWD}:/project" `
  -w /project `
  grafana/k6 run `
  -e BASE_URL=http://host.docker.internal:8080/api/v1 `
  -e ROOT_URL=http://host.docker.internal:8080 `
  -e PUBLIC_URL=https://35.254.78.215.sslip.io `
  k6/e2e-nine-flows.js
```

## WCAG 2.2 AA

La entrega actual expone backend/API y no incluye frontend propio ni paginas
HTML navegables dentro del repositorio. Por eso no hay superficie visual sobre
la cual ejecutar axe/Lighthouse como prueba WCAG tradicional.

Control registrado para la sustentacion:

- La entrada publica solo expone API y healthcheck.
- Caddy agrega headers de seguridad.
- Mailpit queda limitado a loopback en el Web Server.
- Si se agrega frontend, la validacion WCAG 2.2 AA debe ejecutarse con axe o
  Lighthouse y adjuntarse antes del tag final.

Resultado para esta entrega backend: sin hallazgos criticos de accesibilidad en
superficie UI propia porque no existe UI web entregada.

## Logs, metricas, trazas y alertas

Implementado:

- Logs HTTP: `middleware.Logger()` de Echo.
- Correlacion/traza simple por request: `middleware.RequestID()`, con
  propagacion de `X-Request-ID`.
- Healthcheck: `GET /health`.
- Metricas Prometheus: `GET /metrics`.
- Reglas de alerta: `deploy/observability/alerts.yml`.
- Proxy HTTPS y headers: `deploy/caddy/Caddyfile.web`.

Alertas definidas:

- `APIDown`: target de API caido.
- `APIHighErrorRate`: tasa 5xx mayor a 1 % por 10 minutos.
- `APILatencyP95High`: p95 mayor a 1 segundo por 10 minutos.

## CI completo

Workflow: `.github/workflows/ci.yml`.

El pipeline ejecuta:

- checkout;
- setup de Go desde `go.mod`;
- verificacion `gofmt`;
- `go mod download`;
- `go test ./...`;
- build de `cmd/api` y `cmd/worker`;
- validacion de `docker compose`;
- validacion de compose por VM;
- guardia basica contra secretos.

## Recuperacion probada

Runbook principal: `docs/operacion-recuperacion.md`.

Evidencia esperada de recuperacion:

1. Generar backup con `pg_dump`.
2. Validar el backup con `pg_restore --list`.
3. Restaurar en base temporal.
4. Verificar tablas restauradas.
5. Eliminar la base temporal.

Este procedimiento queda separado de la aplicacion para que pueda ejecutarse
contra Cloud SQL sin depender del backend.

