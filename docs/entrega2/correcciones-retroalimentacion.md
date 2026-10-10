# Correcciones a la retroalimentación de la Entrega 2

Este documento registra cada observación de la calificación de la Entrega 2
(76,67/100), la corrección aplicada, el script que la reproduce y la evidencia
que la cierra. Las correcciones son además el punto de partida de la línea
base de la Entrega 3.

| Id | Observación del equipo docente | Corrección | Script | Estado |
| --- | --- | --- | --- | --- |
| C1 | "El login del Web Server real devolvía 500 al cierre" | `REDIS_ADDR` del Web Server apuntado a la IP interna actual del Worker (`10.20.0.2:6379`) después del cambio a `mooc-e2-vpc` | `deploy/gcp/06-correcciones-entrega2.ps1 -Fase RedisWeb` | Cerrada (9 oct 2026): `REDIS_ADDR` ya apuntaba a `10.20.0.2:6379`; registro, verificación y login responden 200 contra el Web Server real |
| C2 | "La conexión no es privada: la instancia expone IP pública con redes autorizadas" | Private Service Access con el rango `mooc-e2-private-services`, IP privada en `mooc-postgres`, `DATABASE_URL` de ambas VM a la IP privada, sin IP pública ni redes autorizadas | `06-correcciones-entrega2.ps1 -Fase IpPrivadaSql`, `CambiarDatabaseUrl`, `QuitarIpPublicaSql` | Cerrada (10 oct 2026): IP privada `10.235.0.3`, `ipv4Enabled: false`, sin redes autorizadas, `sslMode: ENCRYPTED_ONLY`; ambas VM usan la IP privada y el login responde 200 |
| C3 | Desviación documentada: Worker con IP externa | Cloud Router + Cloud NAT, Private Google Access en `mooc-e2-subnet` y Worker sin IP externa | `06-correcciones-entrega2.ps1 -Fase WorkerSinIpPublica` | Cerrada (10 oct 2026): `mooc-e2-worker` solo con IP interna `10.20.0.2`; Private Google Access activo en `mooc-e2-subnet`; salida por `mooc-e2-nat` verificada |
| C6 | Hallazgo propio: la imagen desplegada de la API era anterior a las cookies seguras y CSRF | Imagen reconstruida desde el commit actual, publicada como `mooc-e2-api:298301b` y fijada en `API_IMAGE` del Web Server | `deploy/gcp/08-publicar-imagen-api.ps1` | Cerrada (10 oct 2026): el login emite `mooc_session` (Secure, HttpOnly, SameSite=Lax) y `mooc_csrf` (Secure, SameSite=Lax); `GET /api/v1/me` con cookie 200 |
| C4 | "El flujo multimedia se probó contra una API temporal dentro del Worker, no sobre la topología desplegada" | Repetir el Escenario 2 contra el Web Server real por HTTPS, con los mismos perfiles y niveles | `k6/run-escenario2.ps1 -BaseUrl https://35.254.78.215.sslip.io/api/v1` | Pendiente de ejecutar (después de C1 a C3) |
| C5 | Rúbrica: "archivos trasladados sin comprobar integridad o correspondencia" y "release sin resolver" | Verificación de existencia, tamaño, SHA-256, HLS y PDF de cada `media_asset` contra el bucket; ramas unificadas en `main` y tag `entrega-2` | `deploy/gcp/07-verificar-integridad-objetos.ps1` | Ramas unificadas; verificación pendiente de ejecutar |

## Orden de ejecución

1. `Diagnostico`: deja constancia del estado antes de cambiar nada.
2. `RedisWeb` y `Verificar -Email ... -Password ...`: el login debe responder 200 y crear las cookies `mooc_session` y `mooc_csrf`.
3. `IpPrivadaSql`: activa la IP privada sin quitar todavía la pública, para no cortar el servicio.
4. `CambiarDatabaseUrl`: ambas VM pasan a la IP privada. Se guarda una copia `.env.bak-antes-ip-privada` en cada VM por si hay que revertir.
5. `Verificar` de nuevo. Solo si el login y las operaciones responden bien se sigue.
6. `QuitarIpPublicaSql`: elimina redes autorizadas e IP pública.
7. `WorkerSinIpPublica`: NAT, Private Google Access y retiro de la IP externa del Worker; el script prueba la salida a Cloud Storage, Artifact Registry y las firmas de ClamAV.
8. `07-verificar-integridad-objetos.ps1`: genera `docs/entrega2/integridad-objetos.md`.
9. `k6/run-escenario2.ps1`: nuevas corridas `k6/results/escenario2-ws-*.json`, mientras `capacity-planning/monitor-queue.ps1` registra la cola.

## Efectos sobre la operación

- Con Cloud SQL solo por IP privada, `deploy/run-migrations.ps1` ya no puede correr desde un equipo personal. Las migraciones se ejecutan desde una VM de la VPC (por ejemplo `docker run migrate/migrate` en el Web Server) o mediante Cloud SQL Auth Proxy a través de IAP.
- La administración sigue siendo por SSH a través de IAP (`35.235.240.0/20`), que no necesita IP externa en el Worker.
- Si la IP `34.28.33.182` del Worker estaba reservada como estática, hay que liberarla para que deje de cobrarse.

## Evidencia

Completar con la salida real de cada fase (sin secretos) y la fecha en que se ejecutó.

- Diagnóstico inicial (9 oct 2026): Cloud SQL detenida (`activationPolicy: NEVER`), IP pública `34.42.6.180` con tres redes autorizadas (`mooc-e2-web`, `mooc-e2-worker`, `camila-local`), sin peering de Private Service Access; Worker con IP externa `34.28.33.182`; Cloud NAT `mooc-e2-nat` ya existente en `mooc-e2-nat-router`.
- Login después de C1 (9 oct 2026): `POST /api/v1/auth/register` 201, `verify-email` 200, `login` 200 en 0,45 s, `GET /api/v1/me` con Bearer 200.
- Cloud SQL sin IP pública (10 oct 2026): `ipAddresses` solo `10.235.0.3 (PRIVATE)`, `ipv4Enabled: false`; `verify-email` 200 y `login` 200 después del cambio; migraciones en versión 5, `dirty=false`.
- Hallazgos adicionales durante la corrección: el contenedor `clamav` del Worker llevaba 11 días `unhealthy` y la API temporal `api-test` seguía corriendo en el Worker (se detuvo, sin borrarla); la imagen `mooc-e2-api:latest` desplegada era anterior al commit `6c879a6` de cookies seguras y CSRF, por lo que el login no emitía cookies aunque `AUTH_COOKIE_ENABLED=true` (se corrige con `deploy/gcp/08-publicar-imagen-api.ps1`).
- Worker sin IP externa y salida por NAT (10 oct 2026): `natIP` vacío; desde el Worker por IAP, `storage.googleapis.com` y `database.clamav.net` responden (HTTP 400 y 403, es decir, hay conectividad saliente); `deploy-worker-1`, `deploy-redis-1` y `deploy-clamav-1` en estado `Up` (healthy).
- Imagen de la API (10 oct 2026): `mooc-e2-api:298301b` desplegada; respuesta del login con `csrf_token, token, user` y dos cabeceras `Set-Cookie`; `/api/v1/me` 200 por cookie y por Bearer.
- Reporte de integridad:
- Corridas del Escenario 2 contra el Web Server:
