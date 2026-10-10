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
- Hora de inicio y fin de cada nivel (UTC, para cruzar con las métricas de
  infraestructura de `docs/entrega2/metricas-infra-entrega2.md`):

  | Nivel (VUs) | Inicio (UTC) | Fin (UTC) |
  |---|---|---|
  | 10  | 21:46:42 | 21:50:37 |
  | 25  | 21:52:15 | 21:56:15 |
  | 50  | 21:57:53 | 22:02:04 |
  | 100 | 22:03:49 | 22:07:57 |

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

---

# Pruebas de carga: Escenario 2 (carga, procesamiento y consumo multimedia)

Responsable: Andrés Jurado.

## Repetición contra el Web Server real (corrección C4, 10 de octubre de 2026)

Esta sección responde a la observación del equipo docente: "el flujo multimedia se probó contra una API temporal dentro del Worker, no sobre la topología desplegada". Todas las corridas de esta sección entran por `https://35.254.78.215.sslip.io/api/v1` (Caddy y API en `mooc-e2-web`), con la carga directa a Cloud Storage, el procesamiento en `mooc-e2-worker` y Cloud SQL por IP privada. La API temporal `api-test` del Worker se detuvo antes de empezar.

### Cómo se ejecutó

- Orquestador: `k6/correr-escenario2-cloud.ps1`. Genera los videos, crea un admin de prueba (`admin.k6.e2.<fecha>@mooc.test`, promovido en Cloud SQL desde el Web Server), arranca monitores en ambas VM (`deploy/gcp/remoto/monitor-vm.sh`, muestras cada 5 s), corre `k6/run-escenario2.ps1` y trae los CSV.
- Herramienta: k6 v2.3.0 en Docker Desktop, en el equipo de Daniela (Windows, fuera de las VM de la aplicación).
- Mismo script `k6/escenario2.js`, con dos ajustes que no cambian la carga: las sesiones de k6 descartan las cookies tras el login, porque desde la imagen `mooc-e2-api:298301b` la API emite cookies de sesión y exigiría CSRF a k6, que se autentica con Bearer; y `setupTimeout` pasó de 5 a 20 minutos para alcanzar a precargar los tres perfiles.
- Registros completos: `deploy/gcp/logs/` (local, no versionado), resultados en `k6/results/escenario2-ws-*` y métricas de VM en `capacity-planning/results/escenario2-ws-{web,worker}-*.csv`.

### Primer resultado: con `e2-small` el Worker colapsa en la preparación

Con la configuración de la Entrega 2 (Worker `e2-small`, 2 vCPU compartidas y 2 GiB, `WORKER_CONCURRENCY=5`), el Worker no completó ni la preparación de la línea base. Mientras transcodificaba el video de referencia de 720p (45 s):

- a las 15:18 UTC los health checks de Redis y ClamAV dejaron de responder, y a las 15:24 UTC la VM ya no alcanzaba el servidor de metadatos (`169.254.169.254`, i/o timeout);
- el SSH por IAP quedó sin respuesta;
- como Redis vive en esa VM, el login de toda la plataforma pasó a responder **500 después de ~5,4 s** (`internal_error`), el mismo síntoma que el profesor observó al cierre de la entrega.

Evidencia: `docs/entrega2/evidencia-saturacion-worker-20261010-103447.txt` (consola serial) y `capacity-planning/results/escenario2-ws-web-20261010-101434.csv`. La VM solo se recuperó con `gcloud compute instances reset` (`deploy/gcp/10-recuperar-worker.ps1`). Al reiniciar, el worker retomó con concurrencia 5 los trabajos que habían quedado a medias y colapsó otra vez a los dos minutos (15:38 UTC). Después del reinicio, con los contenedores en reposo, la VM tenía solo **575 MiB disponibles**, porque ClamAV ocupa ~1 GiB de forma permanente.

### Ajuste mínimo aplicado

El enunciado indica que, si una dependencia no puede ejecutarse con los recursos definidos, se documente el fallo, el ajuste mínimo y su efecto en costo y capacidad. El ajuste fue:

- Worker `e2-small` → **`e2-medium`** (2 vCPU, 4 GiB), con `deploy/gcp/12-redimensionar-worker.ps1`. La IP interna `10.20.0.2` se conservó.
- `WORKER_CONCURRENCY` de 5 → **2**, fija durante todas las corridas.
- Costo: según la tarifa on-demand de `us-central1` (valor aproximado; hay que confirmarlo en la calculadora de GCP), `e2-small` cuesta ~USD 0,017/h y `e2-medium` ~USD 0,034/h, es decir unos USD 0,017/h más para el Worker. El Web Server sigue en `e2-small`.

Este es un resultado de la configuración básica: **con 2 GiB, el Worker no puede transcodificar video mientras ClamAV y Redis comparten la máquina**, y su caída arrastra a toda la plataforma porque las sesiones están en el mismo Redis.

### Corridas con `e2-medium` y concurrencia 2

| Corrida | Perfiles | Carga | Checks | `http_req_failed` | Autorizar (avg) | Transferir a GCS (med / p95) | Confirmar (avg) | Hasta `ready` (med / máx) | Manifiesto HLS (avg / p95) | Segmento HLS (med / p95) | Errores HLS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| A, `l1` (`110215`) | 360p 15 s, 720p 45 s, 1080p 90 s | 2 profesores subiendo, 5 estudiantes viendo, 3 min | 278/278 | 0 % (0/593) | 0,27 s* | 1,78 s / 3,2 s | 0,26 s* | 12,6 s / 361 s | 0,19 s / 0,40 s | 0,27 s / 1,06 s | 0/250 |
| B, `baseline` (`114202`) | 360p 15 s, 720p 30 s, 1080p 30 s | 1 profesor, 2 estudiantes, 2,5 min | 63/65 | 2,4 % (4/164) | 0,37 s | 2,1 s / 9,7 s | 0,43 s | 34,8 s / 82,9 s | 0,46 s / 1,08 s | 2,0 s / 17,2 s | 2/41 |
| B, `l1` (`114202`) | 360p 15 s, 720p 30 s, 1080p 30 s | 2 profesores, 5 estudiantes, 3 min | 300/300 | 0 % (0/518) | 0,28 s | 2,7 s / 5,0 s | 0,32 s | 40,7 s / 71,9 s | 0,19 s / 0,35 s | 0,32 s / 1,33 s | 0/264 |

\* Valores de la fase de preparación de esa corrida.

Lectura de los resultados:

- **El flujo completo funciona sobre la topología desplegada.** Las cargas se autorizan en la API, el archivo va directo a Cloud Storage, el worker lo procesa y queda `ready`, y los estudiantes consumen HLS desde el bucket. No hubo fallos funcionales de carga (0/28, 0/24 y 0/36).
- **El tráfico de control de la API es liviano y estable.** Autorizar y confirmar tardan ~0,3 s. En la corrida B la API usó como máximo ~15 % de CPU y ~15 MiB de memoria en el Web Server.
- **El límite está en el procesamiento.** En la corrida A, el tiempo hasta `ready` va de 9 s (360p) a ~6 min (1080p de 90 s). En esa ventana el Worker tuvo una carga promedio de hasta 11 con 2 vCPU, el contenedor worker estuvo por encima del 100 % de CPU la mayor parte del tiempo, la memoria disponible bajó hasta ~1,1 GiB y ClamAV se mantuvo en ~1 GiB. Durante la preparación, el 1080p de 90 s no alcanzó a quedar `ready` en 10 minutos.
- **Los 2 errores HLS y las 4 peticiones fallidas de la línea base B** son descargas de segmentos desde Cloud Storage cortadas con `unexpected EOF` entre las 16:57:51 y las 16:57:57 UTC, justo cuando el equipo generador se suspendió (en ese momento se perdió también la conexión con él). Se atribuyen al generador, no a la plataforma.

### Limitaciones de esta repetición

- Los últimos minutos de la corrida A se cruzaron con la preparación de otra corrida (la `111518`), lo que agregó trabajos al Worker. Su `l2` falló en la preparación porque el 1080p de 90 s no quedó listo en 10 min.
- El nivel `l1` de la corrida B se reanudó después de que el equipo generador volvió de suspensión. Sus resultados de k6 son válidos, pero el monitor de las VM ya había terminado, así que **no hay métricas de infraestructura para ese nivel**.
- Para que la preparación terminara, en la corrida B se redujo la duración de los videos de 720p y 1080p (las resoluciones se mantienen). Por eso las corridas A y B no son comparables en tiempo hasta `ready`.
- No se ejecutaron los niveles `l2`, `l3` ni `peak` sobre el Web Server real. El máximo probado son 2 profesores subiendo y 5 estudiantes viendo, y **no corresponde a la capacidad máxima**.

### Qué cambiaría la capacidad (y qué medición lo respalda)

1. **Sacar ClamAV y Redis del Worker.** ClamAV ocupa ~1 GiB constante y Redis concentra las sesiones de toda la plataforma: cuando el Worker se satura, cae el login. Respaldo: memoria disponible de 575 MiB con la VM en reposo en `e2-small`, y login en 500 a los ~5,4 s durante los colapsos.
2. **Más CPU de procesamiento o más workers en paralelo**, que es lo que introduce la Entrega 3 con autoscaling de workers. Respaldo: carga promedio de hasta 11 con 2 vCPU y tiempo hasta `ready` de ~6 min para el 1080p de 90 s.
3. **Una CDN para HLS** reduciría la latencia de los segmentos vistos desde fuera de `us-central1`, pero no es el cuello de botella actual. Respaldo: con 5 estudiantes, 0 errores HLS y segmentos con mediana de ~0,3 s.

## Estado de esta sección (2026-09-28)

Se ejecutaron dos corridas reales con transferencia de datos y consumo de HLS (línea base y un nivel adicional), pero **no contra el Web Server final**: la infraestructura dedicada no estaba operativa en el momento de la prueba, así que se usó una instancia temporal de la API corriendo como contenedor adicional dentro del propio Worker Server. Esto es una desviación real y documentada, no un error del script, y es también la causa más probable de lo que se describe en "Punto de quiebre" más abajo: bajo la carga siguiente, **el Worker Server colapsó por completo**, incluyendo el acceso por SSH, y solo se recuperó con un reinicio forzado. Por eso no se alcanzaron los niveles `l2`, `l3` ni `peak` originalmente planeados: el tiempo se agotó atendiendo la inestabilidad de la infraestructura compartida en vez de escalar la carga.

Un intento posterior de repetir la prueba contra el Web Server real (`https://35.254.78.215.sslip.io`) reveló un problema de configuración distinto: las peticiones de login tardaban de forma anómala (~5.5 s) y terminaban en error 500, consistente con un `REDIS_ADDR` apuntando a una dirección interna del Worker Server que quedó obsoleta tras el cambio de red del 27 de septiembre. No se alcanzó a corregir ni a repetir la prueba contra el Web Server real antes del cierre de esta entrega; queda como hallazgo pendiente para el equipo.

## Metodología

- Script: `k6/escenario2.js`, reescrito para este escenario (la versión anterior solo probaba el inicio de la carga multipart, sin transferencia real ni consumo de HLS).
- Herramienta: k6 (Grafana k6), instalado desde el repositorio oficial `dl.k6.io`.
- Generador de carga: ejecutado en Cloud Shell / equipo del autor, fuera de las dos VMs de la aplicación.
- Perfiles de video declarados (generados sintéticamente con `ffmpeg`, sin aumentar artificialmente la resolución del original): corto (360p, 15 s), medio (720p, 45 s), largo (1080p, 90 s).
- Recorrido: profesores autorizan y suben un video con carga multipart directa a Cloud Storage (una sola parte, dado el tamaño de los archivos de prueba) y esperan a que el worker lo deje `ready`; en paralelo, estudiantes consumen contenido HLS **ya disponible** (precargado en `setup()`, no el que suben los profesores durante la medición), a la cadencia real de reproducción (segmentos de 6 s, sin descargar todo de golpe).
- Autenticación fuera de la medición (sesiones de profesor/estudiante preparadas en `setup()`).
- Concurrencia del worker: 5, fija en todos los niveles (`WORKER_CONCURRENCY=5`).
- Niveles ejecutados: línea base (1 profesor subiendo, 2 estudiantes
  viendo, 2m30s) y un nivel más (2 profesores, 5 estudiantes, 3m30s).
- Resultados crudos: `k6/results/escenario2-baseline.json`,
  `k6/results/escenario2-l1.json`.

## Entorno de la prueba

Las corridas se ejecutaron sobre el despliegue real en GCP
(`proyecto1-entrega2-desarrollo`, `us-central1`): la API (imagen
`mooc-e2-api` de Artifact Registry) corría como contenedor en la VM
`mooc-e2-worker`, junto al worker, Redis y ClamAV; la persistencia usó
Cloud SQL y el bucket de Cloud Storage reales. El generador de carga
(Cloud Shell) llegaba a esa API a través de un túnel IAP/SSH, por eso el
`BASE_URL` del script aparece como `localhost:8080`: es el extremo local
del túnel, no una ejecución local.

Para interpretar los resultados hay que tener en cuenta que, en estas
corridas, la VM del worker sostenía a la vez el procesamiento (ffmpeg,
ClamAV, Redis) y la API. Es muy probablemente la causa principal del
colapso descrito más abajo.

Se intentó repetir la prueba contra el Web Server (`mooc-e2-web`, vía
`https://35.254.78.215.sslip.io`), con resultados en
`k6/results/escenario2-l1-real.json` y `escenario2-l2-real.json`. Esas
corridas no produjeron datos útiles: el login tardaba ~5.5 s y respondía
500, consistente con un `REDIS_ADDR` del Web Server apuntando a una
dirección interna anterior al cambio de red de la semana. No se alcanzó a
corregir antes del cierre; queda como hallazgo pendiente.

## Resultados por nivel

| Nivel | Perfiles/mezcla | Checks OK | `http_req_failed` | Autorizar (avg) | Transferir (avg) | Confirmar (avg) | Tiempo hasta listo (avg / máx) | Manifiesto HLS (avg) | Segmento HLS (avg) |
|---|---|---|---|---|---|---|---|---|---|
| Línea base (1 subida / 2 vistas) | 3 perfiles, mezcla mixta | 100% (96/96) | 0% | 138 ms | 590 ms | 217 ms | 38.9 s / 145.8 s | 77 ms | 823 ms |
| Nivel siguiente (2 subidas / 5 vistas) | 3 perfiles, mezcla mixta | 96.9% (156/161) | 2.17% | 193 ms | 616 ms | 248 ms | 46.5 s / 156.6 s | 83 ms | 573 ms |

En el nivel siguiente ya aparecen fallos reales: el check `playback 200` cae a 72% de éxito (13 de 18), y una iteración individual tardó 2m33s. No se logró desglosar estos promedios por perfil de video (360p/720p/1080p) en estas dos corridas: el etiquetado necesario para eso se agregó al script después de ejecutarlas, y no hubo tiempo de repetirlas antes del cierre.

## Punto de quiebre: colapso total del Worker Server

Al continuar presionando la carga más allá del segundo nivel, el Worker Server dejó de responder por completo, incluyendo el acceso por SSH. Evidencia capturada de la consola serial y de los logs de la API de pruebas (detalle completo en `docs/entrega2/evidencia-colapso-worker.txt`):

- Los *health checks* de Docker para Redis y ClamAV empezaron a dar timeout.
- El propio sistema operativo no pudo completar una llamada al servicio de metadatos de GCP (`169.254.169.254`), algo normalmente instantáneo — señal de saturación extrema de CPU.
- Una petición de login, que normalmente toma decenas de milisegundos, tardó **9 minutos y 53 segundos** antes de responder con éxito.
- La instancia solo se recuperó tras un reinicio forzado (`gcloud compute instances reset`).

**Cuello de botella identificado:** CPU y memoria del Worker Server (`e2-small`, 2 vCPU / 2 GiB), que bajo carga real debe correr simultáneamente ffmpeg (transcodificación), ClamAV (antivirus) y Redis — y que, en estas corridas, además sostenía la API temporal. No se llegó a medir cuál de los tres procesos domina el consumo individualmente (no se tuvo tiempo de instrumentar `docker stats` de forma continua durante el colapso), pero la combinación de los tres en una máquina de este tamaño es, por sí sola, insuficiente para el nivel de carga probado.

Este colapso confirma empíricamente, con evidencia real de carga, el punto único de falla del Worker Server ya identificado en `docs/entrega2/arquitectura_entrega2.md` (sección "Puntos únicos de falla"): concentrar worker, Redis y ClamAV en una sola VM no solo es un riesgo teórico, sino que se manifestó bajo la primera carga real sostenida.

## Hallazgos técnicos adicionales

- **Idempotencia bloquea el reprocesamiento tras un fallo terminal.** La clave de idempotencia de los jobs (`internal/media/usecase/complete_upload.go`) se construye como `asset.ID + tipo_de_job`. Si un job llega a `dead_letter` (por ejemplo, por una falla transitoria de ClamAV) y el profesor vuelve a subir un archivo al mismo recurso, el sistema encuentra el job viejo por esa misma clave y **nunca vuelve a encolar uno nuevo** — el recurso queda permanentemente bloqueado para reprocesarse, aunque el archivo se suba de nuevo correctamente. No se corrigió por falta de tiempo; queda documentado como hallazgo.
- **Redis mal configurado en el Web Server real** (ver "Estado de esta sección" arriba): `REDIS_ADDR` probablemente apunta a una IP interna del Worker Server anterior al cambio de red de esta semana.

## Limitaciones del experimento

- No se alcanzaron los niveles adicionales planeados (`l2`, `l3`, una
  repetición cerca del límite); el tiempo se agotó atendiendo la
  inestabilidad de la infraestructura compartida.
- Las corridas con datos completos midieron la API alojada en la misma VM
  del worker; no hay una corrida completa contra el Web Server en su VM
  dedicada.
- No se desglosaron los resultados por perfil de video.
- No se instrumentó `docker stats`/CPU de forma continua durante el colapso; la evidencia del cuello de botella es cualitativa (logs y consola serial), no una serie de métricas de recursos.

## Conclusión y recomendación

El hallazgo más importante del Escenario 2 no son los dos niveles medidos, sino el colapso total del Worker Server al presionar la carga un poco más: bajo la arquitectura actual, transcodificación (ffmpeg), antivirus (ClamAV) y cola/sesiones (Redis) compiten por los mismos 2 vCPU / 2 GiB, y en estas corridas además compartían la VM con una API temporal. Esto confirma con evidencia real, no solo teórica, el punto único de falla del Worker Server documentado en la arquitectura.

Propuesta de evolución, en orden de prioridad:

1. **Separar de verdad la API del Worker**, como manda la arquitectura objetivo — esto por sí solo debería eliminar la causa más probable del colapso observado.
2. **Subir el Worker Server a `e2-medium`** (4 GiB de RAM) si, incluso separado de la API, ffmpeg + ClamAV + Redis siguen saturando un `e2-small` bajo carga real.
3. **CDN delante de Cloud Storage** para el prefijo HLS: bajo carga de varios estudiantes viendo el mismo contenido, la mayoría del tráfico es de lectura repetida de los mismos segmentos, un caso ideal para cachear en el borde y reducir tanto la latencia percibida como el costo de egreso del bucket.
4. Corregir la clave de idempotencia de los jobs para que un reintento manual (nueva carga sobre el mismo recurso) sí pueda generar un job nuevo cuando el anterior terminó en `dead_letter`.
5. Corregir `REDIS_ADDR` en el `.env` del Web Server real y repetir la prueba completa (`l1` a `peak`) contra `https://35.254.78.215.sslip.io` antes de dar este escenario por cerrado.
