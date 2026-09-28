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

## Diferencias frente a Entrega 1

- PostgreSQL local se reemplaza por Cloud SQL.
- MinIO local se reemplaza por Cloud Storage.
- La API se publica mediante HTTPS en un Web Server.
- Redis queda en Worker Server y se protege por firewall.
- Los secretos salen del repositorio y se inyectan con archivos `.env` protegidos.

## Evidencias del despliegue real (2026-09-27)

## Evidencias del despliegue real (2026-09-27)

- Dominio HTTPS del Web Server:
  `https://35.254.78.215.sslip.io`.
- Web Server: `mooc-e2-web`, desplegado en `us-central1-a`.
- Worker Server: `mooc-e2-worker`.
- IP interna actual del Worker Server: `10.20.0.2`.
- IP externa actual del Worker Server: `34.28.33.182`.
- VPC dedicada: `mooc-e2-vpc`.
- Subred: `mooc-e2-subnet`.
- Rango de subred: `10.20.0.0/24`.
- Instancia Cloud SQL: `mooc-postgres`, IP pública `34.42.6.180`.
- Bucket de Cloud Storage:
  `mooc-e2-media-proyecto1-entrega2`.

El Worker Server ejecuta mediante Docker Compose:

- worker multimedia;
- Redis 7;
- ClamAV.

El worker fue verificado escuchando correctamente la cola `media` con
`WORKER_CONCURRENCY=5`, y Redis respondió correctamente a `PING`.

- Evidencia de `curl`:

```text
$ curl -v https://35.254.78.215.sslip.io/health
HTTP/1.1 200 OK
Content-Type: application/json
{"status":"ok"}
```

- Resultado de pruebas Postman/k6 sobre la URL cloud: ver
  `capacity-planning/pruebas_de_carga_entrega2.md` (Escenario 1).


## Red desplegada

La infraestructura fue migrada a una VPC dedicada:

- VPC: `mooc-e2-vpc`.
- Subred: `mooc-e2-subnet`.
- CIDR: `10.20.0.0/24`.

Web Server y Worker Server se encuentran dentro de esta red y utilizan
comunicación interna para los servicios que no requieren exposición pública.

Se configuró Cloud NAT como mecanismo de salida para recursos privados. Sin
embargo, durante la entrega el Worker Server conserva también una IP externa,
por lo que Cloud NAT no constituye actualmente su única ruta de salida a
Internet.

# Capacidad, costo y limitaciones

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

Las métricas de Cloud SQL fueron revisadas durante las dos ventanas aproximadas
de ejecución del Escenario 1, entre las 16:00 y las 17:00 del 27 de septiembre.

Durante este intervalo, el uso de CPU de la instancia se mantuvo bajo en términos
generales. Se observaron incrementos moderados asociados a la carga de las
pruebas, pero el consumo permaneció muy por debajo de la capacidad total de la
vCPU disponible, sin evidencias de saturación.

De forma similar, el número total de conexiones activas permaneció ampliamente
por debajo del límite configurado de la instancia de 50 max connections

Incluso durante los periodos de mayor actividad, Cloud SQL conservó margen
disponible tanto en CPU como en número de conexiones.
Por lo tanto, las métricas observadas no muestran evidencia de que Cloud SQL
haya alcanzado su capacidad máxima durante el Escenario 1.
Sin embargo, la API utiliza un pool de conexiones limitado mediante:
DB_MAX_CONNS=5
DB_MIN_CONNS=0

Esto significa que puede existir espera dentro de la propia aplicación antes de
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
más sostenida y alcanza valores cercanos al 35 %–40 %.

Aunque la segunda ejecución incrementó claramente el uso de CPU respecto a la
primera, la instancia no alcanzó niveles cercanos a saturación. Por lo tanto,
las métricas observadas no muestran evidencia de que la CPU del Web Server haya
sido el límite físico principal durante el Escenario 1.

Al combinar este resultado con las métricas de Cloud SQL, donde tampoco se
observó agotamiento del límite global de conexiones ni saturación de CPU, la
degradación observada bajo mayor concurrencia parece ocurrir antes de alcanzar
los límites físicos de CPU de la VM Web o de la instancia de base de datos.

Una hipótesis relevante continúa siendo la contención en recursos internos de
la aplicación, especialmente el pool de conexiones PostgreSQL configurado con:

```text
DB_MAX_CONNS=5
DB_MIN_CONNS=0
```
Con este límite, solicitudes concurrentes pueden quedar esperando una conexión
disponible aunque Cloud SQL todavía tenga capacidad libre. Esta hipótesis no
puede confirmarse únicamente con las métricas actuales y requeriría
instrumentación adicional del pool de conexiones.

### Capacidad observada – Escenario 2

El Escenario 2 evaluó el flujo multimedia completo, incluyendo autorización de
cargas, transferencia hacia almacenamiento, procesamiento asíncrono por el
worker y consumo posterior de contenido HLS.

Se ejecutaron inicialmente dos niveles de carga: `baseline` y `l1`.

| Métrica | Baseline | L1 |
|---|---:|---:|
| Peticiones HTTP | 226 | 322 |
| Fallos HTTP | 0 % | 2.17 % |
| Latencia HTTP p95 | 924 ms | 1 108 ms |
| Latencia HTTP p99 | 1 898 ms | ~60 000 ms |
| Upload → ready promedio | 38.9 s | 46.5 s |
| Upload → ready p95 | 120.9 s | 138.9 s |
| Timeouts esperando estado ready | 1 | 1 |
| Errores HLS | 0 % | 3.88 % |
| Latencia promedio del manifest HLS | 76.6 ms | 83.2 ms |
| Latencia promedio de segmentos HLS | 823 ms | 573 ms |

En el nivel `baseline` el sistema completó el flujo multimedia sin errores HTTP
ni errores HLS. Sin embargo, ya se observó un recurso que no alcanzó el estado
`ready` dentro del tiempo esperado y una latencia p95 de aproximadamente 121
segundos entre la carga y la disponibilidad final.

Al incrementar la carga al nivel `l1`, se evidenció degradación. La tasa de
fallos HTTP aumentó a 2.17 % y varias solicitudes alcanzaron el timeout de
60 segundos.

Los timeouts se concentraron principalmente en consultas de estado multimedia
(`/media`) y reproducción (`/playback`). Esto indica que, bajo mayor
concurrencia, el sistema comienza a presentar dificultades para responder
mientras se ejecuta procesamiento multimedia concurrente.

A pesar de esta degradación, la generación y consumo de HLS continuó
funcionando para la mayoría de los recursos. El nivel baseline no presentó
errores HLS y el nivel `l1` presentó una tasa de error aproximada de 3.88 %.

Por lo tanto, el Escenario 2 muestra que el flujo multimedia es funcional bajo
carga base, pero la capacidad actual del Worker Server y del procesamiento
asíncrono comienza a degradarse desde `l1`.

El principal síntoma observado no es una degradación significativa en la
lectura de objetos ya generados desde Cloud Storage, sino un aumento en los
tiempos necesarios para que los recursos alcancen el estado `ready` y la
aparición de timeouts en la API durante periodos de procesamiento concurrente.

#### Limitación observada en niveles superiores

A partir de niveles de carga superiores a `l1`, el equipo reportó pérdida de
disponibilidad de la VM Worker.

Los resultados de `baseline` y `l1` ya muestran señales previas de degradación,
como aumento del tiempo `upload → ready`, aparición de timeouts y errores HLS.

Sin embargo, con la evidencia disponible no es posible atribuir la caída de la
VM a una causa específica como agotamiento de memoria o saturación de CPU.

Por lo tanto, la caída se documenta como una limitación de capacidad del
Worker Server bajo cargas superiores a `l1`, cuya causa exacta requeriría
métricas adicionales de CPU, memoria y logs del sistema durante el momento de
la falla.

#### Entorno de ejecución del Escenario 2

El script de carga del Escenario 2 fue ejecutado desde Google Cloud Shell.

La API utilizada durante la prueba se encontraba disponible en
`http://localhost:8080/api/v1` dentro de ese entorno, pero estaba configurada
para utilizar los servicios remotos desplegados en GCP, incluyendo el Worker
Server, Redis, Cloud SQL y Cloud Storage.

Por esta razón, los resultados relacionados con procesamiento multimedia,
tiempos `upload → ready`, comportamiento de la cola y generación de contenido
HLS reflejan el comportamiento de la infraestructura remota del Worker Server.

Sin embargo, las métricas de latencia HTTP de esta prueba no se utilizan para
evaluar directamente la capacidad de la VM `mooc-e2-web`, ya que la API no fue
ejecutada sobre esa instancia durante el escenario.

#### Consumo de CPU del Worker Server

Durante la ejecución de las pruebas `baseline` y `l1` del Escenario 2 se
revisaron las métricas de CPU de la instancia `mooc-e2-worker`.

A diferencia del Escenario 1, donde las instancias mantuvieron amplio margen
de CPU, durante el procesamiento multimedia se observó un incremento
significativo de utilización del Worker Server.

Durante el intervalo de ejecución aparecieron múltiples picos superiores al
200 % en la métrica reportada y un pico cercano al 300 %, además de periodos
sostenidos alrededor del 100 %.

La escala utilizada por la métrica puede representar utilización acumulada
entre múltiples vCPU, por lo que valores superiores al 100 % no deben
interpretarse como un porcentaje simple de una única CPU. Sin embargo, el
comportamiento evidencia una utilización intensiva del recurso de cómputo
durante el procesamiento multimedia.

Este patrón es consistente con las operaciones ejecutadas por el worker,
principalmente análisis antivirus y transcodificación HLS mediante FFmpeg,
que son tareas intensivas en CPU.

La presión observada coincide además con la degradación registrada entre
`baseline` y `l1`: el tiempo promedio de `upload → ready` aumentó de
aproximadamente 38.9 s a 46.5 s, el p95 aumentó de aproximadamente 120.9 s a
138.9 s y comenzaron a aparecer timeouts y errores HLS.

Por lo tanto, las métricas permiten identificar la capacidad de procesamiento
del Worker Server como una limitación relevante del Escenario 2. A medida que
aumenta la concurrencia de trabajos multimedia, la VM debe ejecutar
simultáneamente el worker, FFmpeg, ClamAV y Redis, incrementando la presión
sobre los recursos disponibles.

#### Limitación observada en niveles superiores

A partir de niveles de carga superiores a `l1`, el equipo reportó pérdida de
disponibilidad del Worker Server.

Las métricas disponibles muestran una presión importante de CPU durante las
pruebas multimedia, con periodos de utilización elevada y múltiples picos
durante el procesamiento concurrente. Esto permite identificar la capacidad de
cómputo del Worker Server como un factor relevante en la degradación observada.

No se dispone de evidencia suficiente para afirmar que la pérdida de
disponibilidad haya sido causada exclusivamente por CPU, ya que no se cuenta
con una medición equivalente de memoria durante el instante exacto de la falla.

Sin embargo, la combinación de alta utilización de CPU, incremento del tiempo
`upload → ready`, aparición de timeouts y pérdida de disponibilidad bajo
niveles superiores indica que el Worker Server constituye el principal límite
de capacidad observado en el Escenario 2.

#### Métricas de Cloud SQL durante el Escenario 2

Durante el mismo intervalo de ejecución de las pruebas `baseline` y `l1` se
revisaron las métricas de Cloud SQL.

El uso de CPU de la instancia se mantuvo aproximadamente entre 8 % y 10 %,
con incrementos puntuales moderados, sin aproximarse a niveles de saturación.

El número total de conexiones aumentó gradualmente durante las pruebas, pero
permaneció ampliamente por debajo del límite configurado de:

```text
max_connections = 50
```
Por lo tanto, no se observa evidencia de que Cloud SQL haya sido el recurso
limitante durante el Escenario 2.
Este comportamiento contrasta con el Worker Server, donde durante el mismo
intervalo se observaron incrementos importantes en utilización de CPU y
periodos de carga sostenida asociados al procesamiento multimedia.
La combinación de ambas métricas permite localizar la principal presión de
capacidad en la capa de procesamiento asíncrono del Worker Server y no en la
base de datos.
En consecuencia, para mejorar la capacidad del Escenario 2, la primera acción
no debería ser aumentar el tamaño de Cloud SQL. Resulta más apropiado revisar
la capacidad del Worker Server, la concurrencia configurada y el consumo de
recursos generado por FFmpeg, ClamAV y los demás procesos ejecutados en esa VM.

#### Conclusión de capacidad del Escenario 2

Las pruebas muestran que el flujo multimedia es funcional en `baseline` y
continúa operando bajo `l1`, aunque con degradación en tiempos de procesamiento,
aparición de timeouts y algunos errores HLS.

Las métricas de infraestructura muestran que, durante estas ejecuciones,
Cloud SQL conservó amplio margen tanto en CPU como en conexiones, mientras que
el Worker Server presentó una utilización significativamente mayor durante los
periodos de procesamiento multimedia.

Por lo tanto, el Worker Server constituye el principal límite de capacidad
observado en el Escenario 2.

Para aumentar la capacidad del flujo multimedia se recomienda evaluar, en este
orden:

1. reducir o ajustar `WORKER_CONCURRENCY`;
2. monitorear CPU y memoria durante cada trabajo de transcodificación;
3. aumentar recursos de CPU/memoria de la VM Worker si se confirma saturación;
4. distribuir los trabajos entre múltiples workers si la cola crece de forma
   sostenida;
5. mantener Cloud SQL en su configuración actual mientras sus métricas continúen
   mostrando capacidad disponible.

### Costos observados

Durante la ejecución de la Entrega 2 se revisó el consumo acumulado del proyecto
`proyecto1-entrega2-desarrollo` en Google Cloud Billing.

Los costos observados fueron:

| Servicio | Costo por uso | Ahorros/créditos | Subtotal |
|---|---:|---:|---:|
| Cloud SQL | USD 3.45 | -USD 3.45 | USD 0.00 |
| Networking | USD 0.01 | -USD 0.01 | USD 0.00 |

El principal componente de costo observado fue Cloud SQL. Aunque el costo por
uso acumulado alcanzó USD 3.45, los créditos o ahorros aplicados compensaron el
valor durante el periodo analizado, por lo que el subtotal facturado mostrado
por Billing fue de USD 0.00.

También se configuró un presupuesto mensual de:

```text
USD 20
```

con alertas de gasto real en:
50 %
75 %
90 %

Además, para Compute Engine y Cloud Storage no se mostraban costo desglosado en la captura disponible al momento de la revisión.
