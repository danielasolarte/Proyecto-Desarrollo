# Arquitectura Entrega 2

Este documento consolida la arquitectura desplegada para la Entrega 2. Las secciones de red y Web Server fueron preparadas por Samara Martinez.

## Componentes

- API modular en Go con Echo.
- Worker en Go con asynq.
- Redis para sesiones y cola.
- ClamAV para escaneo.
- Cloud SQL for PostgreSQL para persistencia transaccional.
- Cloud Storage como almacenamiento de objetos compatible con S3.
- Caddy como proxy inverso HTTPS en Web Server.

## Despliegue

El despliegue usa dos maquinas virtuales fijas:

- Web Server: API, Mailpit de laboratorio y proxy HTTPS.
- Worker Server: worker, Redis y ClamAV.

PostgreSQL se ejecuta en Cloud SQL y los objetos se almacenan en Cloud Storage.

Modelo de componentes (módulos, worker y comunicación síncrona/asíncrona):

- [Modelo de componentes](modelo-componentes.md)

Detalle de red, firewall y HTTPS:

- [Modelo de despliegue de Samara](modelo-despliegue-samara.md)
- [Fuente del diagrama de red](red-gcp.mmd)

Seguridad y manejo de secretos:

- [Secretos y seguridad](secretos-y-seguridad.md)

Decisiones y adaptaciones de empaquetado y configuración:

- [Decisiones y adaptaciones: empaquetado (Daniela)](decisiones-empaquetado-daniela.md)

Operación y recuperación (migraciones, despliegue, reinicio, respaldo):

- [Operación y recuperación](../operacion-recuperacion.md)

Capacidad, costo y procedimiento de métricas:

- [Estimación de costos](../costos-entrega2.md)
- [Procedimiento de métricas de infraestructura](../metricas-infra-entrega2.md)
- [Informe de capacidad (Escenario 1 y 2)](../../capacity-planning/pruebas_de_carga_entrega2.md)

## Diferencias frente a Entrega 1

- PostgreSQL local se reemplaza por Cloud SQL.
- MinIO local se reemplaza por Cloud Storage.
- La API se publica mediante HTTPS en un Web Server.
- Redis queda en Worker Server y se protege por firewall.
- Los secretos salen del repositorio y se inyectan con archivos `.env` protegidos.

## Evidencias del despliegue real (2026-09-27)

- Dominio HTTPS: `https://35.254.78.215.sslip.io` (Caddy con certificado
  Let's Encrypt).
- Red: VPC personalizada `mooc-e2-vpc` / subred `mooc-e2-subnet`
  (`10.20.0.0/24`), la propuesta en `modelo-despliegue-samara.md`.
- IP estatica del Web Server (`mooc-e2-web`): `35.254.78.215` (interna
  `10.20.0.3`).
- IP interna del Worker Server (`mooc-e2-worker`): `10.20.0.2` (tambien
  tiene IP externa `34.28.33.182`, desviacion registrada; ver
  `docs/costos-entrega2.md`).
- Instancia Cloud SQL: `mooc-postgres` (IP publica `34.42.6.180`).
- Bucket de Cloud Storage: `mooc-e2-media-proyecto1-entrega2`.
- Evidencia de `curl`:

```text
$ curl -v https://35.254.78.215.sslip.io/health
HTTP/1.1 200 OK
Content-Type: application/json
{"status":"ok"}
```

- Resultado de pruebas Postman/k6 sobre la URL cloud: ver
  `capacity-planning/pruebas_de_carga_entrega2.md` (Escenario 1 y 2).

## Desviaciones frente al diseno de red original

El Worker Server quedo con IP externa (`34.28.33.182`) en vez de solo
salida por Cloud NAT, como recomienda el diseno original. La VPC
personalizada si se implemento (ver "Evidencias" arriba); esa desviacion
inicial quedo resuelta el 27 de septiembre. Detalle completo en
`docs/costos-entrega2.md`.

## Capacidad, costo y limitaciones

Configuración exacta, estimación y consumo observado: ver
[Estimación de costos](../costos-entrega2.md) (Compute Engine `e2-small` x2,
Cloud SQL `db-g1-small` sin HA, Cloud Storage clase Standard) y el
[informe de capacidad](../../capacity-planning/pruebas_de_carga_entrega2.md)
para el consumo observado durante las corridas de k6.

### Puntos únicos de falla

Esta configuración básica (sin balanceo, sin réplicas, número fijo de
máquinas) tiene varios puntos únicos de falla, todos aceptados
explícitamente para el alcance de esta entrega:

- **Web Server (`mooc-e2-web`).** Una sola instancia sirve toda la API y
  el proxy HTTPS. Si la VM cae, la aplicación completa deja de responder;
  no hay balanceador ni segunda instancia.
- **Worker Server (`mooc-e2-worker`).** Concentra tres responsabilidades en
  una sola VM: el worker de procesamiento (asynq), Redis (sesiones y cola)
  y ClamAV. La caída de esta VM detiene el procesamiento asíncrono
  multimedia, invalida todas las sesiones activas y bloquea la cola,
  simultáneamente.
- **Redis como servicio único compartido.** Al vivir en un solo contenedor
  sin persistencia administrada, una caída pierde tanto las sesiones
  activas (fuerza relogin a todos los usuarios) como los mensajes de la
  cola `asynq` en tránsito.
- **Cloud SQL sin alta disponibilidad.** La instancia `db-g1-small` corre
  en una sola zona sin réplica; una falla de zona deja la base de datos
  transaccional completa fuera de servicio hasta que se restaure desde
  respaldo.
- **Ausencia de balanceo o escalado automático.** No hay mecanismo para
  desviar tráfico si una VM se degrada ni para absorber picos de carga
  añadiendo capacidad (confirmado por el punto de quiebre del Escenario 1,
  entre 50 y 100 VUs concurrentes).

### Cambios que permitirían evolucionar hacia una aplicación elástica

En orden de impacto esperado frente al costo de implementarlos:

1. **Separar Redis de Worker Server hacia un servicio administrado**
   (Memorystore for Redis), para que la caída de la VM del worker deje de
   arrastrar sesiones y cola al mismo tiempo, y para poder escalar el
   worker sin perder estado compartido.
2. **Subir el límite de conexiones hacia PostgreSQL** (`DB_MAX_CONNS` y el
   tamaño de instancia de Cloud SQL, hoy `db-g1-small` con
   `max_connections = 50`), la palanca de mayor impacto medido: el
   Escenario 1 identificó el pool de conexiones como el cuello de botella
   entre 50 y 100 VUs, antes que CPU o memoria.
3. **Habilitar alta disponibilidad en Cloud SQL** (instancia regional con
   failover automático) para eliminar el punto único de falla de la base
   de datos transaccional.
4. **Reemplazar la VM única de Web Server por un Managed Instance Group
   detrás de un balanceador de carga HTTP(S)**, con las instancias
   configuradas sin estado local (las sesiones ya viven en Redis, no en la
   VM), habilitando escalado automático por CPU o por solicitudes.
5. **Escalar Worker Server horizontalmente** (más instancias del worker
   detrás de la misma cola de asynq) en vez de aumentar
   `WORKER_CONCURRENCY` indefinidamente en una sola VM, una vez Redis ya
   no dependa de esa misma VM.
6. **Añadir una CDN delante de Cloud Storage** para la entrega de HLS y
   otros derivados, reduciendo la latencia de reproducción y la carga
   directa sobre el bucket a medida que crece el número de estudiantes
   concurrentes (fuera del alcance de esta entrega, mencionado
   explícitamente en el enunciado como evolución posterior).

Ninguno de estos cambios se implementó en esta entrega: el alcance
definido fija el número de máquinas y descarta explícitamente balanceo,
réplicas y autoescalado. Quedan aquí como la propuesta de evolución
sustentada en la evidencia de capacidad recolectada.
