## Prueba de escalamiento del worker

### Configuración de la prueba

- Tipo de recurso: presentación PPTX
- Tamaño del archivo: 32 254 bytes
- Worker: Cloud Run
- Concurrencia por instancia: 1
- Máximo de instancias: 3
- Ráfaga: 9 cargas simultáneas

### Medición individual

En la prueba con una sola presentación se obtuvo:

- upload_request_ms p95: 205 ms
- time_to_ready_ms p95: 5.05 s
- ready_timeouts: 0
- upload_failures: 0 %

El tiempo observado para que una presentación quedara disponible fue de aproximadamente
5.05 segundos, muy por debajo del objetivo de 2 minutos establecido para documentos.

### Ráfaga de 9 presentaciones

La prueba se ejecutó alrededor de las 20:40 hora Colombia
(01:40 UTC del 12 de noviembre de 2026).

De las 9 cargas iniciadas, 8 completaron correctamente el flujo multimedia.
Una solicitud falló antes de llegar al backend con un error transitorio
`connection refused`. Los logs de Cloud Run durante la misma ventana muestran
únicamente respuestas 200 y 201 para las solicitudes que sí llegaron a la aplicación.

Para las cargas procesadas se obtuvo:

- time_to_ready_ms p95: 5.05 s
- upload_request_ms p95: 276.89 ms
- ready_timeouts: 0
- máximo de workers activos observado: 3

### Comportamiento observado

Durante la ráfaga, el número de instancias activas del worker aumentó hasta 3,
respetando el máximo configurado.

La cantidad de mensajes pendientes en la suscripción aumentó temporalmente hasta
aproximadamente 5 mensajes y posteriormente volvió a 0.

La edad del mensaje no confirmado más antiguo alcanzó aproximadamente 20 segundos
y también volvió a 0 después de procesar la carga.

La mayoría de las entregas push fueron confirmadas mediante HTTP 204.
También se observaron respuestas 500 transitorias, que fueron reintentadas por
Pub/Sub. La cola terminó completamente drenada.

### Evidencias

- `docs/entrega4/evidencias/04-instancias.png`
- `docs/entrega4/evidencias/04-pendientes.png`
- `docs/entrega4/evidencias/04-edad.png`
- `docs/entrega4/evidencias/04-respuestas.png`
- `k6/results/e4/rafaga-presentacion-1-20261112-013408.log`
- `k6/results/e4/rafaga-presentacion-9-20261112-014051.log`

### Prueba de video

Se utilizó un video corto de 15 segundos, resolución 640×360 y tamaño aproximado
de 238 KB.

#### Medición individual

Con una sola carga se obtuvo:

- upload_request_ms p95: 183 ms
- time_to_ready_ms p95: 5.05 s
- ready_timeouts: 0
- upload_failures: 0 %

El video quedó disponible mediante playback aproximadamente 5.05 segundos después
de confirmar la carga, muy por debajo del objetivo de 5 minutos.

#### Ráfaga de 6 videos

La ráfaga de seis videos se ejecutó el 12 de noviembre de 2026 alrededor de las
02:01 UTC.

Resultados:

- 6 de 6 cargas completadas correctamente
- upload_request_ms p95: 640.25 ms
- time_to_ready_ms p95: 5.05 s
- ready_timeouts: 0
- upload_failures: 0 %
- http_req_failed: 0 %

La prueba confirma que el procesamiento HLS mantuvo los objetivos definidos incluso
bajo una ráfaga concurrente.

### Modelo de capacidad

El worker está configurado con concurrencia 1 y un máximo de 3 instancias.

A partir de la medición individual se obtiene una estimación inicial de capacidad:

Presentaciones:

c_presentacion ≈ 60 / 5.051 ≈ 11.88 jobs/min por instancia

Capacidad máxima estimada:

3c_presentacion ≈ 35.64 jobs/min

Video corto:

c_video ≈ 60 / 5.052 ≈ 11.88 jobs/min por instancia

Capacidad máxima estimada:

3c_video ≈ 35.64 jobs/min

Estos valores deben interpretarse como capacidad observada por la prueba, ya que el
script consulta el estado de playback cada 5 segundos. Por tanto, time_to_ready_ms
está cuantizado por el intervalo de polling y no representa con precisión subsegundo
el tiempo interno de procesamiento.

Para una tasa de entrada e, el cambio esperado de la cola puede expresarse como:

Δcola ≈ e - 3c

Cuando e > 3c, la cantidad de mensajes pendientes aumenta. Cuando la ráfaga termina
y e cae por debajo de 3c, los workers drenan la cola.

Este comportamiento se observó experimentalmente: durante la prueba la cola alcanzó
aproximadamente 5 mensajes pendientes, el worker escaló hasta 3 instancias y,
posteriormente, los mensajes pendientes y la edad del mensaje más antiguo volvieron
a cero.

### Verificación de la DLQ

Después de las pruebas de presentación y video se consultó la suscripción
`mooc-media-jobs-dlq-revision`.

La consulta devolvió `Listed 0 items`, por lo que no quedaron trabajos en la cola
de mensajes fallidos. Esto confirma que, pese a los reintentos transitorios
observados durante la ráfaga, todos los trabajos pendientes fueron finalmente
procesados y la cola se drenó por completo.

### Limite de escalamiento backend

El backend se limita a un máximo de 3 instancias de Cloud Run, con una concurrencia máxima de 40 solicitudes por instancia. Esta decisión también controla el consumo de conexiones hacia Cloud SQL: cada instancia del backend utiliza un pool de hasta 5 conexiones (`DB_MAX_CONNS=5`), por lo que, incluso con las 3 instancias activas, el backend utilizaría como máximo 15 conexiones simultáneas. De esta forma se mantiene margen para el worker, las migraciones y otras operaciones sobre la base de datos, evitando que el escalamiento horizontal del backend agote las conexiones disponibles.