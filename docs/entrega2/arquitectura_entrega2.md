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

## Evidencias pendientes de completar

Cuando los recursos reales esten activos, registrar:

- dominio HTTPS final;
- IP estatica del Web Server;
- IP interna del Worker Server;
- nombre de instancia Cloud SQL;
- bucket de Cloud Storage;
- capturas o salidas de `curl https://<dominio>/health`;
- resultado de pruebas Postman/k6 sobre la URL cloud.

## Capacidad, costo y limitaciones

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
