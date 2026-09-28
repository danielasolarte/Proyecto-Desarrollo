# Arquitectura Entrega 2

Este documento consolida la arquitectura desplegada para la Entrega 2, incluyendo red, Web Server, Worker Server, servicios administrados y controles de seguridad.

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

- [Modelo de despliegue, red y Web Server](modelo-despliegue-red-web.md)
- [Fuente del diagrama de red](red-gcp.mmd)

Seguridad y manejo de secretos:

- [Secretos y seguridad](secretos-y-seguridad.md)

Calidad, observabilidad, CI y evidencia E2E:

- [Calidad, observabilidad y CI](calidad-observabilidad-ci.md)

Decisiones y adaptaciones de empaquetado y configuración:

- [Decisiones y adaptaciones: empaquetado (Daniela)](decisiones-empaquetado-daniela.md)

Operación y recuperación (migraciones, despliegue, reinicio, respaldo):

- [Operación y recuperación](../operacion-recuperacion.md)

Capacidad, costo y procedimiento de métricas:

- [Estimación de costos](costos-entrega2.md)
- [Procedimiento de métricas de infraestructura](metricas-infra-entrega2.md)
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
- VPC personalizada: `mooc-e2-vpc`.
- Subred regional: `mooc-e2-subnet` (`10.20.0.0/24`).
- Web Server (`mooc-e2-web`): zona `us-central1-a`, IP interna `10.20.0.3`,
  IP externa estatica `35.254.78.215`.
- Worker Server (`mooc-e2-worker`): zona `us-central1-a`, IP interna
  `10.20.0.2`, IP externa `34.28.33.182` (desviacion registrada; ver
  "Red y seguridad de acceso" y `costos-entrega2.md`).
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

## Red y seguridad de acceso

El despliegue usa una VPC personalizada (`mooc-e2-vpc`) con subred regional
`mooc-e2-subnet`. El Web Server es el único punto de entrada público por
HTTP/HTTPS y Caddy termina TLS con Let's Encrypt. Redis se mantiene en el
Worker Server y solo acepta tráfico interno desde instancias con etiqueta
`web-server`. El Worker Server conserva IP externa para poder salir hacia
Cloud SQL por IP pública mientras no esté completada la conexión privada por
Private Service Access; aun así, no hay regla pública hacia Redis, ClamAV ni
puertos internos de la aplicación. La administración SSH se realiza por IAP
con origen `35.235.240.0/20`.

## Desviaciones frente al diseño de red original

El Worker Server quedó con IP externa (`34.28.33.182`) en vez de solo salida
por Cloud NAT, como recomienda el diseño original; la razón práctica
registrada es que aún no se completó la conexión privada hacia Cloud SQL por
Private Service Access (ver "Red y seguridad de acceso" arriba), así que la
salida por IP pública se mantuvo como solución temporal. La VPC personalizada
sí se implementó (ver "Evidencias del despliegue real" arriba); la desviación
inicial de estar en la red `default` quedó resuelta el 27 de septiembre.
Detalle completo en `costos-entrega2.md`.

## Capacidad, costo y limitaciones

Configuración exacta, estimación y consumo observado: ver
[Estimación de costos](costos-entrega2.md) (Compute Engine `e2-small` x2,
Cloud SQL `db-g1-small` sin HA, Cloud Storage clase Standard) y el
[informe de capacidad](../../capacity-planning/pruebas_de_carga_entrega2.md)
para el consumo observado durante las corridas de k6.

### Capacidad observada – Escenario 1

El Escenario 1 evaluó el comportamiento del Web Server y de la capa de
persistencia bajo una carga académica concurrente. Las pruebas se ejecutaron
contra el despliegue real en GCP mediante k6, utilizando niveles de 10, 25,
50 y 100 usuarios virtuales (VUs).

La carga mantuvo una mezcla constante de operaciones: 50 % de lectura,
30 % de operaciones de progreso y 20 % de interacción con quizzes.

| VUs | Peticiones | Throughput (req/s) | p95 (ms) | p99 (ms) | Fallos HTTP |
|---:|---:|---:|---:|---:|---:|
| 10 | 1 078 | 3.90 | 246 | 474 | 0.186 % |
| 25 | 2 520 | 9.08 | 220 | 411 | 0.198 % |
| 50 | 5 039 | 17.54 | 225 | 411 | 0.198 % |
| 100 | 6 808 | 23.38 | 1 049 | 29 998 | 1.763 % |

Entre 10 y 50 VUs el sistema mantiene un comportamiento estable. El throughput
aumenta aproximadamente con la carga mientras que la latencia p95 permanece
entre 220 y 250 ms y la tasa de fallos HTTP se mantiene por debajo de 0.2 %.

El cambio principal ocurre entre 50 y 100 VUs. En 100 VUs el p95 aumenta de
225 ms a 1 049 ms, mientras que el p99 alcanza aproximadamente 30 segundos,
coincidiendo con el timeout configurado para las solicitudes. La tasa de
fallos HTTP también aumenta de 0.198 % a 1.763 %.

Por lo tanto, para la configuración evaluada, el punto de degradación se
encuentra entre 50 y 100 usuarios concurrentes. El nivel de 50 VUs puede
considerarse una carga estable para el despliegue actual, mientras que
100 VUs ya evidencia saturación y aumento significativo de latencia.

### Relación con Cloud SQL

La instancia utilizada para las pruebas es Cloud SQL for PostgreSQL 16 con
una configuración `db-g1-small`, 1 vCPU, 1.7 GB de memoria y un límite
observado de:

```text
max_connections = 50
```

### Métricas de Cloud SQL durante el Escenario 1

Las métricas de Cloud SQL fueron revisadas durante las dos ventanas
aproximadas de ejecución del Escenario 1, entre las 16:00 y las 17:00 del 27
de septiembre.

Durante este intervalo, el uso de CPU de la instancia se mantuvo bajo en
términos generales. Se observaron incrementos moderados asociados a la carga
de las pruebas, pero el consumo permaneció muy por debajo de la capacidad
total de la vCPU disponible, sin evidencias de saturación.

De forma similar, el número total de conexiones activas permaneció ampliamente
por debajo del límite configurado de la instancia (50 `max_connections`).

Incluso durante los periodos de mayor actividad, Cloud SQL conservó margen
disponible tanto en CPU como en número de conexiones. Por lo tanto, las
métricas observadas no muestran evidencia de que Cloud SQL haya alcanzado su
capacidad máxima durante el Escenario 1. Sin embargo, la API utiliza un pool
de conexiones limitado mediante `DB_MAX_CONNS=5` / `DB_MIN_CONNS=0`. Esto
significa que puede existir espera dentro de la propia aplicación antes de
que las solicitudes lleguen a PostgreSQL. En consecuencia, un número bajo de
conexiones observado en Cloud SQL no permite descartar completamente una
posible contención en el pool de conexiones de la API.

### Métricas de la VM Web durante el Escenario 1

El Escenario 1 fue ejecutado en dos ventanas aproximadas durante la tarde del
27 de septiembre: una entre las 16:00 y 16:30 y otra entre las 16:30 y 17:00.

Durante la primera ejecución, la utilización de CPU de la instancia
`mooc-e2-web` permaneció baja durante la mayor parte de la prueba, generalmente
en valores de un dígito, con algunos incrementos puntuales.

Durante la segunda ejecución se observa una carga mayor sobre la VM. Entre
aproximadamente las 16:45 y las 17:00, la utilización de CPU aumenta de forma
más sostenida y alcanza valores cercanos al 35 %-40 %.

Aunque la segunda ejecución incrementó claramente el uso de CPU respecto a la
primera, la instancia no alcanzó niveles cercanos a saturación. Por lo tanto,
las métricas observadas no muestran evidencia de que la CPU del Web Server
haya sido el límite físico principal durante el Escenario 1.

Al combinar este resultado con las métricas de Cloud SQL, donde tampoco se
observó agotamiento del límite global de conexiones ni saturación de CPU, la
degradación observada bajo mayor concurrencia parece ocurrir antes de alcanzar
los límites físicos de CPU de la VM Web o de la instancia de base de datos.
Una hipótesis relevante continúa siendo la contención en recursos internos de
la aplicación, especialmente el pool de conexiones PostgreSQL (`DB_MAX_CONNS=5`,
`DB_MIN_CONNS=0`): con este límite, solicitudes concurrentes pueden quedar
esperando una conexión disponible aunque Cloud SQL todavía tenga capacidad
libre. Esta hipótesis no puede confirmarse únicamente con las métricas
actuales y requeriría instrumentación adicional del pool de conexiones.

### Costos observados

Durante la ejecución de la Entrega 2 se revisó el consumo acumulado del
proyecto `proyecto1-entrega2-desarrollo` en Google Cloud Billing.

Los costos observados fueron:

| Servicio | Costo por uso | Ahorros/créditos | Subtotal |
|---|---:|---:|---:|
| Cloud SQL | USD 3.45 | -USD 3.45 | USD 0.00 |
| Networking | USD 0.01 | -USD 0.01 | USD 0.00 |

El principal componente de costo observado fue Cloud SQL. Aunque el costo por
uso acumulado alcanzó USD 3.45, los créditos o ahorros aplicados compensaron
el valor durante el periodo analizado, por lo que el subtotal facturado
mostrado por Billing fue de USD 0.00. También se configuró un presupuesto
mensual de USD 20, con alertas de gasto real en 50 %, 75 % y 90 %. Para
Compute Engine y Cloud Storage no se mostraba costo desglosado en la captura
disponible al momento de la revisión.

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
  simultáneamente. **Confirmado empíricamente durante las pruebas del
  Escenario 2** (2026-09-28): bajo carga real de transcodificación, la
  instancia colapsó por completo (incluido el acceso por SSH) y solo se
  recuperó con un reinicio forzado; ver
  `docs/entrega2/evidencia-colapso-worker.txt` y la sección "Punto de
  quiebre" en `capacity-planning/pruebas_de_carga_entrega2.md`.
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
