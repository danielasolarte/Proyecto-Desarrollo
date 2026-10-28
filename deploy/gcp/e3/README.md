# Despliegue elástico (Entrega 3)

Scripts de PowerShell para Windows. Se ejecutan desde la raíz del repositorio o con el botón **Run** de VS Code. Cada uno deja su registro en `deploy/gcp/logs/e3-*.log` (sin secretos).

## Cómo arranca una VM de los grupos

1. La **imagen de máquina** `mooc-<rol>-<commit>` ya trae Docker, el contenedor descargado y `mooc.service` habilitado (`02-imagen-maquina.ps1`).
2. Al encender, `mooc.service` ejecuta `mooc-preparar.sh`, que:
   - lee de los metadatos de la plantilla la imagen del contenedor, la configuración no sensible (`mooc-config`) y los nombres de los secretos (`mooc-secretos`);
   - pide los secretos a Secret Manager con la cuenta de servicio de la VM;
   - escribe `/run/mooc/env` en memoria, con permisos 600.
3. `mooc.service` arranca el contenedor con nombre fijo (`mooc`). Un reinicio no duplica procesos, y `docker stop -t` da tiempo al apagado ordenado.

No hay acceso manual ni pasos posteriores. Las migraciones **no** se ejecutan al arrancar.

## Orden (API)

| Paso | Comando | Qué deja |
| --- | --- | --- |
| 1 | `.\deploy\gcp\e3\01-secretos-identidades.ps1` | `sa-mooc-api`, `sa-mooc-worker`, secretos en Secret Manager |
| 2 | `.\deploy\gcp\e3\02-imagen-maquina.ps1 -Rol api` | Contenedor `mooc-e2-api:<commit>` e imagen `mooc-api-<commit>` |
| 3 | `.\deploy\gcp\e3\03-grupo-api.ps1 -Fase Red` | Firewall: health checks, SSH por IAP, Redis temporal |
| 4 | `.\deploy\gcp\e3\03-grupo-api.ps1 -Fase Plantilla` | Plantilla `mooc-api-<commit>-<fecha>` |
| 5 | `.\deploy\gcp\e3\03-grupo-api.ps1 -Fase Salud` | `mooc-api-hc-listo` (balanceador) y `mooc-api-hc-vida` (autorreparación) |
| 6 | `.\deploy\gcp\e3\03-grupo-api.ps1 -Fase Grupo` | `mooc-api-mig` con 1 instancia |
| 7 | `.\deploy\gcp\e3\03-grupo-api.ps1 -Fase Probar` | `/health/ready`, `X-Instance-Id` y tiempo de arranque |
| 8 | `.\deploy\gcp\e3\03-grupo-api.ps1 -Fase Autoscaling` | 1 a 3 instancias, 60 % de CPU |

Para publicar una versión nueva: hacer commit y luego ejecutar `02 -Rol api`, `03 -Fase Plantilla` y `03 -Fase Actualizar`.

Entre pruebas, `03 -Fase Apagar` deja el grupo en 0 instancias y `03 -Fase Encender` lo vuelve a levantar.

## VM de servicios (Mateo)

VM fija `mooc-servicios`, fuera de los grupos, con Redis (cola asynq y sesiones, AOF) y ClamAV. Usa una IP interna reservada (`mooc-servicios-ip`) para que `REDIS_ADDR` de las plantillas no cambie si la VM se recrea. Solo `mooc-api` y `mooc-worker` llegan a 6379 y 3310.

| Paso | Comando | Qué deja |
| --- | --- | --- |
| 1 | `.\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Identidad` | `sa-mooc-servicios` (logs, métricas, solo `mooc-database-url`) |
| 2 | `.\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Red` | IP reservada y regla `mooc-e3-allow-servicios` |
| 3 | `.\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Crear` | VM con `deploy/docker-compose.servicios.yml` levantado por su script de arranque |
| 4 | `.\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Probar` | Redis, ClamAV y conectividad desde una instancia de la API |
| 5 | `.\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Replicar` | Copia en vivo del Redis de la E2 (cola y sesiones) |
| 6 | `.\deploy\gcp\e3\04-vm-servicios.ps1 -Fase Promover` | Redis nuevo como principal; imprime el `-RedisAddr` para la plantilla de la API |

## Workers (Mateo)

Mismo mecanismo de arranque que la API. La señal de autoscaling es `custom.googleapis.com/mooc/cola/demanda` (pendientes + activos de la cola `media`), que publica `cola-exportador` cada 30 s desde `mooc-servicios`; incluye los activos para no retirar workers ocupados con la cola vacía. Concurrencia 2 por VM (igual que la C4 de la E2).

| Paso | Comando | Qué deja |
| --- | --- | --- |
| 1 | `.\deploy\gcp\e3\02-imagen-maquina.ps1 -Rol worker` | Contenedor `mooc-e2-worker:<commit>` (incluye `cola-exportador`) e imagen `mooc-worker-<commit>` |
| 2 | `.\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Red` | Regla `mooc-e3-allow-hc-worker` (health checks → :8081) |
| 3 | `.\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Salud` | `mooc-worker-hc-vida` (`/health` en 8081, autorreparación) |
| 4 | `.\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Exportador` | `cola-exportador` en `mooc-servicios` (una sola copia) |
| 5 | `.\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Plantilla` | Plantilla `mooc-worker-<commit>-<fecha>`: e2-medium, Redis y ClamAV en `mooc-servicios`, `mooc-detener-s=330` |
| 6 | `.\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Grupo` | `mooc-worker-mig` con 1 instancia |
| 7 | `.\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Probar` | `/health`, consumo de la cola, tiempo de arranque y muestras de la cola |
| 8 | `.\deploy\gcp\e3\07-grupo-workers.ps1 -Fase Autoscaling` | 1 a 3 VM, 4 trabajos (pendientes + activos) por VM |

Retiro seguro: al borrar una VM, `docker stop -t 330` manda SIGTERM; el worker deja de tomar trabajos y espera hasta 300 s (`WORKER_SHUTDOWN_TIMEOUT_SECONDS`) a los activos. Lo que no termine vuelve a la cola y lo toma otro worker; el estado del job en Postgres evita derivados duplicados.

Permisos que necesita `sa-mooc-servicios` para el exportador: `roles/monitoring.metricWriter` (proyecto) y `roles/artifactregistry.reader` (repositorio `mooc-e2`).

## Migraciones

`.\deploy\gcp\e3\05-migraciones.ps1 -Fase Version` y `-Fase Aplicar`. Corren una sola vez desde `mooc-servicios`, leen `DATABASE_URL` de Secret Manager con `sa-mooc-servicios` y usan el bloqueo de golang-migrate; nunca en el arranque de las réplicas. Se aplican antes de publicar una versión de la API que las necesite.

## CORS del bucket de medios

El navegador del profesor sube los archivos directo al bucket con URL prefirmadas. El bucket debe aceptar el origen del frontend:

```powershell
.\deploy\gcp\e3\06-cors-bucket.ps1 -Origen "http://<IP o dominio del balanceador>"
.\deploy\gcp\e3\06-cors-bucket.ps1 -Ver
```

## Balanceador HTTPS y frontend (Samara)

Un solo dominio, `https://<IP del balanceador>.sslip.io`: `/api/*` va al servicio de backend `mooc-api-bs` (grupo `mooc-api-mig`) y todo lo demás al bucket del frontend. HTTP redirige a HTTPS con un 301. Como el frontend y la API comparten origen, las cookies `mooc_session` y `mooc_csrf` funcionan sin CORS, y la API no necesitó cambios. `/metrics` y `/health` no se publican.

| Paso | Comando | Qué deja |
| --- | --- | --- |
| 1 | `.\deploy\gcp\e3\08-balanceador.ps1 -Fase Crear` | IP `mooc-e3-lb-ip`, bucket `mooc-e3-frontend-<proyecto>` y backend bucket, `mooc-api-bs` (health check `mooc-api-hc-listo`, drenaje de 30 s, logs), mapas de URL, certificado administrado, proxies y reglas 443/80 |
| 2 | `.\deploy\gcp\e3\08-balanceador.ps1 -Fase Publicar` | Compila `frontend/` con el commit, agrega `version.json` y sincroniza con el bucket |
| 3 | `.\deploy\gcp\e3\06-cors-bucket.ps1 -Origen "https://<dominio>","http://localhost:5173"` | CORS del bucket de medios para las cargas desde el dominio nuevo |
| 4 | `.\deploy\gcp\e3\08-balanceador.ps1 -Fase Probar` | Estado del certificado, salud por instancia, rutas y reparto por `X-Instance-Id` |
| 5 | `.\deploy\gcp\e3\08-balanceador.ps1 -Fase Sesion` | Login, `/me` entre réplicas, CSRF (403 sin encabezado) y logout. Usa `MOOC_STUDENT_EMAIL` y `MOOC_STUDENT_PASSWORD` |
| 6 | `.\deploy\gcp\e3\08-balanceador.ps1 -Fase Excluir -Segundos 60` | Escenario 3A: con 2 instancias sanas, bloquea Redis en una, `/health/ready` pasa a 503 y el balanceador la saca sin que la autorreparación la recree. Al final la reincorpora. CSV en `capacity-planning/results/` |

El certificado tarda entre 15 y 60 minutos en pasar de `PROVISIONING` a `ACTIVE`. `-Fase Estado` muestra los recursos. `-Fase Borrar -Confirmar` quita el balanceador, pero conserva la IP (y con ella el dominio y el certificado al recrear) y el bucket.
