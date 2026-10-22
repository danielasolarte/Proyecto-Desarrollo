# Cola compartida, workers elásticos y retiro seguro

Sección de `docs/entrega3/arquitectura.md` sobre la capa de procesamiento asíncrono. Scripts en [`deploy/gcp/e3/`](../../deploy/gcp/e3/README.md): `04-vm-servicios.ps1` (cola compartida) y `07-grupo-workers.ps1` (grupo de workers).

## Componentes

| Componente | Dónde corre | Responsabilidad |
| --- | --- | --- |
| Redis 7 (cola asynq `media` y sesiones) | `mooc-servicios`, VM fija fuera de los grupos | Cola compartida por la API y todos los workers. AOF activado y `maxmemory-policy noeviction` (asynq no tolera que se borren llaves) |
| ClamAV (`clamd`, puerto 3310) | `mooc-servicios` | Escaneo antivirus por red; los workers lo alcanzan como `clamav` (`--add-host=clamav:10.20.0.7`) |
| `cola-exportador` | `mooc-servicios`, una sola copia | Publica cada 30 s el estado de la cola en Cloud Monitoring; es la señal del autoscaling |
| Worker (asynq, ffmpeg, LibreOffice, clamdscan) | Grupo `mooc-worker-mig`, 1 a 3 VM | Consume la cola `media`: escaneo, transcodificación HLS y conversión a PDF |

La cola no vive en ninguna máquina que el autoscaling pueda retirar. `mooc-servicios` tiene la IP interna reservada `mooc-servicios-ip` (10.20.0.7), así que `REDIS_ADDR` en las plantillas no cambia si la VM se recrea. La cola y las sesiones se migraron desde el Worker de la E2 con `REPLICAOF` (13 llaves, sin pérdida) antes de cambiar la plantilla de la API.

## Plantilla de los workers

| Parámetro | Valor |
| --- | --- |
| Plantilla | `mooc-worker-60a53f8-10212129` |
| Tipo de máquina | e2-medium (2 vCPU compartidas, 4 GB), igual que el Worker ajustado en la corrección C4 de la E2 |
| Disco | 30 GB pd-balanced (archivos temporales de transcodificación en `/tmp/media`) |
| Imagen de máquina | `mooc-worker-60a53f8` (familia `mooc-worker`): Debian 12, Docker, contenedor ya descargado y `mooc.service` |
| Contenedor | `us-central1-docker.pkg.dev/proyecto1-entrega2-desarrollo/mooc-e2/mooc-e2-worker:60a53f8` |
| Red | `mooc-e2-subnet`, sin IP externa (salida por Cloud NAT), etiqueta `mooc-worker` |
| Identidad | `sa-mooc-worker`: lee sus tres secretos, lee Artifact Registry, escribe logs y métricas |
| Configuración (metadato `mooc-config`, sin secretos) | `REDIS_ADDR=10.20.0.7:6379`, `WORKER_CONCURRENCY=2`, `WORKER_SHUTDOWN_TIMEOUT_SECONDS=300`, `WORKER_HEALTH_ADDR=:8081`, `DB_MAX_CONNS=5` ([`config/worker.config`](../../deploy/gcp/e3/config/worker.config)) |
| Secretos (Secret Manager, leídos al arrancar) | `DATABASE_URL`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`; se escriben en `/run/mooc/env` (tmpfs, permisos 600) |
| Apagado | Apagado ordenado de Compute Engine de hasta 6 min y `mooc-detener-s=330` (`docker stop -t 330`) |

**Concurrencia:** 2 trabajos por VM, la misma de la E2, para que el Escenario 2 sea comparable. Lo que cambia en la E3 es el número de VM, de 1 a 3, así que la concurrencia total efectiva va de 2 a 6.

## Arranque y criterio de listo

Al crear una VM, `mooc.service` ejecuta `mooc-preparar.sh` (lee metadatos y secretos) y arranca el contenedor con nombre fijo `mooc`, de modo que un reinicio no duplica procesos. No hay acceso manual ni migraciones en el arranque.

Un worker está **listo** cuando `GET :8081/health` responde `{"status":"ok"}` y el log muestra `worker de multimedia escuchando en la cola 'media'`. Desde ese momento ya consume trabajos.

**Tiempo de arranque medido: 48 s**, desde la creación de la VM por el autoscaler (14:39:25 UTC del 22/10) hasta que el worker escucha la cola (14:40:14), en `mooc-worker-5h39`. La API tarda 28 s; la diferencia se debe al tamaño de la imagen del worker (ffmpeg y LibreOffice).

## Política de escalamiento

| Parámetro | Valor configurado | Justificación |
| --- | --- | --- |
| Señal | `custom.googleapis.com/mooc/cola/demanda` = pendientes + activos de la cola `media`, métrica por grupo (filtro `resource.type = "global" AND metric.labels.cola = "media"`) | La CPU no sirve: ffmpeg satura la CPU con un solo trabajo y no distingue entre 1 y 20 trabajos en espera. Se suman los activos porque una cola sin pendientes puede tener trabajos en ejecución, y retirar esos workers los cortaría |
| Objetivo | 4 trabajos por VM (`singleInstanceAssignment: 4`) | Con concurrencia 2: 2 en ejecución y 2 en espera por VM. Con demanda 5 el grupo pide 2 VM; con 9 o más, 3 |
| Muestreo | Cada 30 s (`cola-exportador`) | Más fino que la evaluación del autoscaler (del orden de 1 min) |
| Inicialización | 120 s (`coolDownPeriodSec`) | Medido: una VM tarda 48 s en procesar. Se deja margen para el health check |
| Estabilización y reducción | Espera de 10 min de Compute Engine antes de reducir, y como máximo 1 VM retirada cada 600 s (`scaleInControl`) | Evita retirar workers por una caída momentánea de la cola y reduce la rotación de VM |
| Límites | Mínimo 1, máximo 3 | Al llegar a 3 el autoscaler reporta `CAPPED_AT_MAX_NUM_REPLICAS` y la cola crece. Los trabajos esperan, no se rechazan |

Política aplicada (`07-grupo-workers.ps1 -Fase Autoscaling`, autoscaler `mooc-worker-mig-nvbn`):

```yaml
coolDownPeriodSec: 120
customMetricUtilizations:
- filter: resource.type = "global" AND metric.labels.cola = "media"
  metric: custom.googleapis.com/mooc/cola/demanda
  singleInstanceAssignment: 4.0
maxNumReplicas: 3
minNumReplicas: 1
mode: ON
scaleInControl:
  maxScaledInReplicas:
    fixed: 1
  timeWindowSec: 600
```

## Health check y autorreparación

| Parámetro | Valor |
| --- | --- |
| Health check | `mooc-worker-hc-vida`: HTTP `GET /health` en el puerto 8081 |
| Intervalo y timeout | 10 s y 5 s |
| Umbrales | 2 aciertos para sano, 3 fallos para no sano |
| Retraso inicial de autorreparación | 180 s |
| Firewall | `mooc-e3-allow-hc-worker`: solo los rangos de health checks de Google (130.211.0.0/22, 35.191.0.0/16) a tcp:8081. Los workers no reciben otro tráfico |

`/health` responde 200 solo si el servidor de asynq arrancó, el último ping a Redis (cada 15 s) salió bien y tiene menos de 45 s, y el worker no se está apagando. No revisa Postgres a propósito: una caída breve de Cloud SQL haría que el grupo recreara todos los workers a la vez sin ganar nada. La respuesta no expone direcciones ni mensajes de error.

## Retiro seguro de workers

Cuando el autoscaler retira una VM:

1. Compute Engine inicia el apagado ordenado (hasta 6 min) y el sistema detiene `mooc.service`.
2. `docker stop -t 330` envía SIGTERM al worker.
3. El worker deja de tomar trabajos nuevos, marca `/health` como `apagando` y espera hasta 300 s a los trabajos activos.
4. Si un trabajo no termina en ese tiempo, asynq lo devuelve a la cola y lo toma otro worker. El estado del job en Postgres hace que una entrega repetida de un job ya en `done` o `dead_letter` se ignore.

Los archivos temporales viven en `/tmp/media` del disco de la VM, se reconstruyen desde Cloud Storage y se pierden con ella. Ningún archivo local es la única copia de un dato de negocio.

## Verificación del ciclo de escalamiento (22/10/2026)

Para comprobar la política antes del Escenario 3B se escribió una **demanda simulada** en la métrica (12 durante 8 min y luego 0) con la API de Cloud Monitoring desde Cloud Shell. Después, `cola-exportador` empezó a publicar la cola real, en 0. **Esta corrida no reemplaza el Escenario 3B**, que se ejecuta con cargas multimedia reales; solo verifica la configuración y los tiempos.

| Hora (UTC) | Evento | Ejecutado por |
| --- | --- | --- |
| 14:35:36 | Se crea el autoscaler `mooc-worker-mig-nvbn` | Equipo (configuración) |
| 14:37:28 | Primer valor `demanda=12` | Simulación |
| 14:38:52 | El autoscaler sigue en 1 (acaba de recibir los primeros datos) | — |
| 14:39:25 | Se crea `mooc-worker-5h39` | Cuenta de servicio de Compute Engine |
| 14:39:26 | Se crea `mooc-worker-bbmg` | Cuenta de servicio de Compute Engine |
| 14:40:14 | `mooc-worker-5h39` escucha la cola | — |
| 14:47:24 | Último valor `demanda=12` | Simulación |
| 14:49:41 | `cola-exportador` publica la cola real (`demanda=0`) | — |
| 14:58:28 | Se retira `mooc-worker-f3wk` | Cuenta de servicio de Compute Engine |
| 15:09:02 | Se retira `mooc-worker-bbmg` | Cuenta de servicio de Compute Engine |

| Medida | Valor |
| --- | --- |
| Desde el aumento de la demanda hasta la decisión de escalar | ~1 min 57 s (14:37:28 → 14:39:25) |
| Desde la decisión hasta que la nueva VM procesa | 48 s (creación de la VM a las 14:39:25.7 → worker escuchando a las 14:40:14) |
| Desde el último valor alto hasta la primera reducción | 11 min 4 s (14:47:24 → 14:58:28) |
| Entre la primera y la segunda reducción | 10 min 34 s (`scaleInControl` de 600 s) |

Las creaciones y retiros los ejecutó `167186676477@cloudservices.gserviceaccount.com`, la cuenta con la que actúa el grupo de instancias administrado. La cuenta del equipo solo aparece al crear el grupo y el autoscaler. Salida de `gcloud compute operations list` (horas en UTC−7):

```
TIMESTAMP                      TYPE                                  TARGET                USER
2026-10-21T14:38:31.878-07:00  compute.instanceGroupManagers.insert  mooc-worker-mig       (cuenta del equipo)
2026-10-21T14:40:05.049-07:00  insert                                mooc-worker-f3wk      167186676477@cloudservices.gserviceaccount.com
2026-10-22T07:35:36.452-07:00  compute.autoscalers.insert            mooc-worker-mig-nvbn  (cuenta del equipo)
2026-10-22T07:39:25.688-07:00  insert                                mooc-worker-5h39      167186676477@cloudservices.gserviceaccount.com
2026-10-22T07:39:26.912-07:00  insert                                mooc-worker-bbmg      167186676477@cloudservices.gserviceaccount.com
2026-10-22T07:58:28.273-07:00  delete                                mooc-worker-f3wk      167186676477@cloudservices.gserviceaccount.com
2026-10-22T08:09:02.871-07:00  delete                                mooc-worker-bbmg      167186676477@cloudservices.gserviceaccount.com
```

Registros de las fases en `deploy/gcp/logs/`: `e3-07-grupo-workers-Autoscaling-20261022-143534.log`, `e3-07-grupo-workers-Exportador-20261022-144854.log`, `e3-07-grupo-workers-Probar-20261022-151845.log` y `e3-07-grupo-workers-Apagar-20261022-152121.log`.

## Puntos únicos de falla y limitaciones

- **`mooc-servicios`** aloja Redis, ClamAV y el exportador. Si se detiene, la API pierde sesiones y cola, y los workers no arrancan. No se exige alta disponibilidad en esta entrega; queda registrado en la deuda técnica.
- **`cola-exportador`.** Si deja de publicar, el autoscaler no tiene datos y conserva el tamaño actual: no sube ni baja, pero los workers siguen procesando. Se observó el 22/10: al cortarse la simulación, el grupo se quedó en 3 VM hasta que el exportador volvió a publicar.
- **Cloud SQL.** Los workers no arrancan sin Postgres (lo validan al iniciar).
- **Redis sin contraseña.** El acceso a 6379 y 3310 está limitado por firewall a las etiquetas `mooc-api` y `mooc-worker`.
- **Cancelación de ffmpeg.** Si se agota el `ShutdownTimeout`, asynq devuelve el trabajo a la cola, pero el proceso de ffmpeg no se cancela con el contexto: termina cuando el contenedor se detiene. El trabajo se repite completo en otro worker.
