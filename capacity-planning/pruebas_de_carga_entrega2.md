# Pruebas de carga: Escenario 1 (actividad académica concurrente)

Responsable: Daniela Solarte.

## Metodología

- Script: `k6/escenario1.js`, ejecutado con `k6/run-escenario1.ps1`.
- Destino: `https://35.254.78.215.sslip.io/api/v1` (despliegue real en GCP,
  no local).
- Niveles: 10, 25, 50 y 100 VUs totales, con la misma mezcla en todos
  (50% lectores, 30% progreso, 20% quizzes).
- Cada nivel: 30 s de rampa, 180 s en el nivel objetivo, 15 s de bajada,
  con 60 s de pausa entre niveles para que la cola y las conexiones se
  estabilicen.
- Fecha de la corrida: 2026-09-27 (tag de resultados `gcp`).
- Resultados crudos en `k6/results/e1-gcp-10.json`, `e1-gcp-25.json`,
  `e1-gcp-50.json`, `e1-gcp-100.json` (con sus `.log`).

## Resultados por nivel

| Nivel (VUs) | Peticiones | Throughput (req/s) | p95 (ms) | p99 (ms) | Máx (ms) | % fallos HTTP | % checks OK | Journeys completados |
|---|---|---|---|---|---|---|---|---|
| 10  | 1 078 | 3.90  | 246  | 474    | 715    | 0.186% | 99.93% | 197 |
| 25  | 2 520 | 9.08  | 220  | 411    | 604    | 0.198% | 99.85% | 489 |
| 50  | 5 039 | 17.54 | 225  | 411    | 5 288  | 0.198% | 99.86% | 1 005 |
| 100 | 6 808 | 23.38 | 1 049 | 29 998 | 30 006 | 1.763% | 98.57% | 1 332 |

## Punto de quiebre

Entre 10 y 50 VUs el sistema escala de forma prácticamente lineal: el
throughput crece con el número de VUs, la latencia p95 se mantiene estable
alrededor de 220-250 ms, y los fallos HTTP se mantienen por debajo del
0.2%. El único síntoma temprano es el máximo aislado de 5.3 s en el nivel
50 (una petición puntual, no un patrón sostenido).

**Entre 50 y 100 VUs el sistema se satura de forma clara:**

- La latencia p95 se multiplica por 4.7 (225 ms → 1 049 ms).
- La latencia p99 llega al techo de `REQUEST_TIMEOUT` (30 s): las
  peticiones más lentas ya no terminan por procesamiento lento, sino por
  timeout del cliente.
- La tasa de fallos HTTP casi se multiplica por 9 (0.198% → 1.763%).
- El porcentaje de checks que pasan cae de 99.86% a 98.57%.

Esto indica que **la capacidad segura del despliegue actual (dos VMs
`e2-small`, Cloud SQL `db-g1-small`, `DB_MAX_CONNS=5`) está entre 50 y 100
usuarios concurrentes**, y que el límite más probable es el tamaño del
pool de conexiones a PostgreSQL (`DB_MAX_CONNS=5` compartido entre todas
las peticiones concurrentes) más que CPU o memoria: en el monitoreo de
Compute Engine durante la corrida, la CPU de `mooc-e2-web` y
`mooc-e2-worker` mostró picos puntuales (uno cercano al 100-110% en
`mooc-e2-worker`) coincidiendo con la corrida, pero sin agotarse de forma
sostenida, mientras que la degradación de latencia sí fue sostenida
durante todo el nivel de 100 VUs.

## Hallazgo funcional: calificación doble bajo concurrencia

El contador `dup_submit_double_grading` (envíos duplicados de un mismo
quiz con la misma `Idempotency-Key`, donde ambas peticiones concurrentes
terminan calificando en vez de que la segunda reciba el resultado ya
calculado) crece con la carga en todos los niveles:

| Nivel (VUs) | dup_submit_double_grading | dup_submit_inconsistent |
|---|---|---|
| 10  | 57  | 1 |
| 25  | 146 | 5 |
| 50  | 296 | 10 |
| 100 | 352 | 42 |

Esto sugiere que la protección de `Idempotency-Key` en el envío de quizzes
(`internal/quizzes`) tiene una condición de carrera: bajo concurrencia
real dos peticiones con la misma llave pueden calificar antes de que la
primera termine de escribir el resultado, en vez de que la segunda
reconozca el resultado ya guardado. No se observaron regresiones de
progreso (`progress_regressions = 0` en todos los niveles), así que el
problema queda acotado al flujo de calificación de quizzes.

## Conclusión y recomendación

El despliegue actual soporta con margen cómodo cargas de hasta 50 VUs
concurrentes (los niveles esperados de uso académico normal, según el
enunciado). Para escalar por encima de 50-100 usuarios concurrentes sin
degradar la experiencia, la primera palanca recomendada es subir
`DB_MAX_CONNS` junto con el tamaño de instancia de Cloud SQL (hoy
`db-g1-small`, `max_connections = 50` a nivel de servidor), antes que
escalar las VMs de cómputo. Independiente de la capacidad, se recomienda
revisar la sección crítica de calificación en `internal/quizzes` para
cerrar la condición de carrera detectada en `dup_submit_double_grading`.

# Pruebas de carga: Escenario 2 (carga, procesamiento y consumo multimedia)

Responsable: Andrés Jurado.

## Metodología

- Script: `k6/escenario2.js`, escrito para este escenario (el guion previo
  solo probaba el inicio de la carga multipart, sin transferencia real ni
  consumo de HLS).
- Herramienta: k6 (Grafana k6), instalado desde el repositorio oficial
  `dl.k6.io`.
- Generador de carga: ejecutado en Cloud Shell / equipo del autor, fuera
  de las dos VMs de la aplicación.
- Perfiles de video declarados (generados sintéticamente con `ffmpeg`,
  sin aumentar artificialmente la resolución del original):
  - Corto: 360p, 15 s.
  - Medio: 720p, 45 s.
  - Largo: 1080p, 90 s.
- Recorrido: profesores autorizan y suben un video con carga multipart
  directa a Cloud Storage (una sola parte, dado el tamaño de los archivos
  de prueba) y esperan a que el worker lo deje `ready`; en paralelo,
  estudiantes consumen contenido HLS **ya disponible** (precargado en
  `setup()`, no el que suben los profesores durante la medición), a la
  cadencia real de reproducción (segmentos de 6 s, sin descargar todo de
  golpe).
- Autenticación: fuera de la medición (sesiones de profesor/estudiante
  preparadas en `setup()`).
- Concurrencia del worker: 5, fija en todos los niveles (`WORKER_CONCURRENCY=5`).
- Niveles ejecutados: línea base (1 profesor subiendo, 2 estudiantes
  viendo, 2m30s) y un nivel más (2 profesores, 5 estudiantes, 3m30s).
- Resultados crudos: `k6/results/escenario2-baseline.json`,
  `k6/results/escenario2-l1.json`.

## Configuración efectiva y una limitación importante

Estas dos corridas **no se ejecutaron contra el Web Server final** de la
arquitectura: se ejecutaron contra una instancia temporal de la imagen
`mooc-e2-api`, corriendo como contenedor adicional dentro del propio
Worker Server, porque el Web Server dedicado no estaba operativo en el
momento de la prueba. Esto es una desviación real y documentada, no un
error de configuración del script: implica que, durante estas corridas,
el Worker Server sostenía simultáneamente el procesamiento (ffmpeg,
ClamAV, Redis) **y** la propia API, algo que la arquitectura objetivo no
contempla. Es también, muy probablemente, la causa principal del colapso
descrito más abajo.

Un intento posterior de repetir la prueba contra el Web Server real
(`https://35.254.78.215.sslip.io`) reveló un problema de configuración
distinto: las peticiones de login tardaban de forma anómala (~5.5 s) y
terminaban en error 500, consistente con un `REDIS_ADDR` apuntando a una
dirección interna del Worker Server que quedó obsoleta tras un cambio de
red durante la semana. No se alcanzó a corregir ni a repetir la prueba
contra el Web Server real antes del cierre de esta entrega; queda como
hallazgo pendiente para el equipo.

## Resultados por nivel

| Nivel | Perfiles/mezcla | Checks OK | `http_req_failed` | Autorizar (avg) | Transferir (avg) | Confirmar (avg) | Tiempo hasta listo (avg / máx) | Manifiesto HLS (avg) | Segmento HLS (avg) |
|---|---|---|---|---|---|---|---|---|---|
| Línea base (1 subida / 2 vistas) | 3 perfiles, mezcla mixta | 100% (96/96) | 0% | 138 ms | 590 ms | 217 ms | 38.9 s / 145.8 s | 77 ms | 823 ms |
| Nivel siguiente (2 subidas / 5 vistas) | 3 perfiles, mezcla mixta | 96.9% (156/161) | 2.17% | 193 ms | 616 ms | 248 ms | 46.5 s / 156.6 s | 83 ms | 573 ms |

En el nivel siguiente ya aparecen fallos reales: el check `playback 200`
cae a 72% de éxito (13 de 18), y una iteración individual tardó 2m33s. No
se logró desglosar estos promedios por perfil de video (360p/720p/1080p)
en estas dos corridas: el etiquetado necesario para eso se agregó al
script después de ejecutarlas, y no hubo tiempo de repetirlas antes del
cierre.

## Punto de quiebre: colapso total del Worker Server

Al continuar presionando la carga más allá del segundo nivel, el Worker
Server dejó de responder por completo, incluyendo el acceso por SSH.
Evidencia capturada de la consola serial y de los logs de la API de
pruebas (detalle completo en
`docs/entrega2/evidencia-colapso-worker.txt`):

- Los *health checks* de Docker para Redis y ClamAV empezaron a dar
  timeout.
- El propio sistema operativo no pudo completar una llamada al servicio
  de metadatos de GCP (`169.254.169.254`), algo normalmente instantáneo —
  señal de saturación extrema de CPU.
- Una petición de login, que normalmente toma decenas de milisegundos,
  tardó **9 minutos y 53 segundos** antes de responder con éxito.
- La instancia solo se recuperó tras un reinicio forzado
  (`gcloud compute instances reset`).

**Cuello de botella identificado:** CPU y memoria del Worker Server
(`e2-small`, 2 vCPU / 2 GiB), que bajo carga real debe correr
simultáneamente ffmpeg (transcodificación), ClamAV (antivirus) y Redis —
y que, en estas corridas, además sostenía la API temporal. No se llegó a
medir cuál de los tres procesos domina el consumo individualmente (no se
tuvo tiempo de instrumentar `docker stats` de forma continua durante el
colapso), pero la combinación de los tres en una máquina de este tamaño
es, por sí sola, insuficiente para el nivel de carga probado.

## Hallazgos técnicos adicionales

- **Idempotencia bloquea el reprocesamiento tras un fallo terminal.** La
  clave de idempotencia de los jobs (`internal/media/usecase/complete_upload.go`)
  se construye como `asset.ID + tipo_de_job`. Si un job llega a
  `dead_letter` (por ejemplo, por una falla transitoria de ClamAV) y el
  profesor vuelve a subir un archivo al mismo recurso, el sistema
  encuentra el job viejo por esa misma clave y **nunca vuelve a encolar
  uno nuevo** — el recurso queda permanentemente bloqueado para
  reprocesarse, aunque el archivo se suba de nuevo correctamente. No se
  corrigió por falta de tiempo; queda documentado como hallazgo.
- **Redis mal configurado en el Web Server real** (ver sección de
  limitaciones arriba): `REDIS_ADDR` probablemente apunta a una IP
  interna del Worker Server anterior al cambio de red de esta semana.

## Limitaciones del experimento

- No se alcanzaron los niveles adicionales planeados (`l2`, `l3`, una
  repetición cerca del límite); el tiempo se agotó atendiendo la
  inestabilidad de la infraestructura compartida.
- Las corridas con datos completos se ejecutaron contra una API temporal
  en el Worker Server, no contra el Web Server final.
- No se desglosaron los resultados por perfil de video.
- No se instrumentó `docker stats`/CPU de forma continua durante el
  colapso; la evidencia del cuello de botella es cualitativa (logs y
  consola serial), no una serie de métricas de recursos.

## Propuesta de evolución

1. **Separar de verdad la API del Worker**, como manda la arquitectura
   objetivo — esto por sí solo debería eliminar la causa más probable del
   colapso observado.
2. **Subir el Worker Server a `e2-medium`** (4 GiB de RAM) si, incluso
   separado de la API, ffmpeg + ClamAV + Redis siguen saturando un
   `e2-small` bajo carga real.
3. **CDN delante de Cloud Storage** para el prefijo HLS: bajo carga de
   varios estudiantes viendo el mismo contenido, la mayoría del tráfico
   es de lectura repetida de los mismos segmentos, un caso ideal para
   cachear en el borde y reducir tanto la latencia percibida como el
   costo de egreso del bucket.
4. Corregir la clave de idempotencia de los jobs para que un reintento
   manual (nueva carga sobre el mismo recurso) sí pueda generar un job
   nuevo cuando el anterior terminó en `dead_letter`.
