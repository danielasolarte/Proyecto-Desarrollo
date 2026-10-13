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

## Workers (Mateo)

`02-imagen-maquina.ps1 -Rol worker` construye la imagen del worker con el mismo mecanismo. La plantilla del worker usa los mismos metadatos, más:

- `mooc-detener-s`: más largo, alineado con `ShutdownTimeout` de asynq.
- `mooc-docker-args=--add-host=clamav:<IP de la VM de servicios>`: el contenedor del worker busca ClamAV por el nombre `clamav`.

## Pendiente cuando exista la VM de servicios

- Crear una plantilla nueva con `-RedisAddr <IP de servicios>:6379` y aplicar `-Fase Actualizar`.
- Borrar la regla temporal `mooc-e3-redis-temporal`.
- Mover las migraciones a la VM de servicios. Mientras tanto se usa `..\08-publicar-imagen-api.ps1 -AplicarMigraciones`, que las ejecuta una sola vez desde el Web Server dentro de la VPC.
