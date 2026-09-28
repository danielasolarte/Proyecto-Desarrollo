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

Modelo de componentes (mÃ³dulos, worker y comunicaciÃ³n sÃ­ncrona/asÃ­ncrona):

- [Modelo de componentes](modelo-componentes.md)

Detalle de red, firewall y HTTPS:

- [Modelo de despliegue, red y Web Server](modelo-despliegue-red-web.md)
- [Fuente del diagrama de red](red-gcp.mmd)

Seguridad y manejo de secretos:

- [Secretos y seguridad](secretos-y-seguridad.md)

Calidad, observabilidad, CI y evidencia E2E:

- [Calidad, observabilidad y CI](calidad-observabilidad-ci.md)

Decisiones y adaptaciones de empaquetado y configuraciÃ³n:

- [Decisiones y adaptaciones: empaquetado (Daniela)](decisiones-empaquetado-daniela.md)

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
- Subred regional: `mooc-e2-subnet`.
- Web Server (`mooc-e2-web`): zona `us-central1-a`, IP interna `10.20.0.3`,
  IP externa estatica `35.254.78.215`.
- Worker Server (`mooc-e2-worker`): zona `us-central1-a`, IP interna
  `10.20.0.2`, IP externa `34.28.33.182`.
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
  `capacity-planning/pruebas_de_carga_entrega2.md` (Escenario 1).

## Red y seguridad de acceso

El despliegue usa una VPC personalizada (`mooc-e2-vpc`) con subred regional
`mooc-e2-subnet`. El Web Server es el unico punto de entrada publico por
HTTP/HTTPS y Caddy termina TLS con Let's Encrypt. Redis se mantiene en el
Worker Server y solo acepta trafico interno desde instancias con etiqueta
`web-server`. El Worker Server conserva IP externa para poder salir hacia
Cloud SQL por IP publica mientras no este completada la conexion privada por
Private Service Access; aun asi, no hay regla publica hacia Redis, ClamAV ni
puertos internos de la aplicacion. La administracion SSH se realiza por IAP
con origen `35.235.240.0/20`.

## Capacidad, costo y limitaciones

### Capacidad observada â€“ Escenario 1

El Escenario 1 evaluÃ³ el comportamiento del Web Server y de la capa de
persistencia bajo una carga acadÃ©mica concurrente. Las pruebas se ejecutaron
contra el despliegue real en GCP mediante k6, utilizando niveles de 10, 25,
50 y 100 usuarios virtuales (VUs).

La carga mantuvo una mezcla constante de operaciones: 50 % de lectura,
30 % de operaciones de progreso y 20 % de interacciÃ³n con quizzes.

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
fallos HTTP tambiÃ©n aumenta de 0.198 % a 1.763 %.

Por lo tanto, para la configuraciÃ³n evaluada, el punto de degradaciÃ³n se
encuentra entre 50 y 100 usuarios concurrentes. El nivel de 50 VUs puede
considerarse una carga estable para el despliegue actual, mientras que
100 VUs ya evidencia saturaciÃ³n y aumento significativo de latencia.

### RelaciÃ³n con Cloud SQL

La instancia utilizada para las pruebas es Cloud SQL for PostgreSQL 16 con
una configuraciÃ³n `db-g1-small`, 1 vCPU, 1.7 GB de memoria y un lÃ­mite
observado de:

```text
max_connections = 50
```

### MÃ©tricas de Cloud SQL durante el Escenario 1

Las mÃ©tricas de Cloud SQL fueron revisadas durante las dos ventanas aproximadas
de ejecuciÃ³n del Escenario 1, entre las 16:00 y las 17:00 del 27 de septiembre.

Durante este intervalo, el uso de CPU de la instancia se mantuvo bajo en tÃ©rminos
generales. Se observaron incrementos moderados asociados a la carga de las
pruebas, pero el consumo permaneciÃ³ muy por debajo de la capacidad total de la
vCPU disponible, sin evidencias de saturaciÃ³n.

De forma similar, el nÃºmero total de conexiones activas permaneciÃ³ ampliamente
por debajo del lÃ­mite configurado de la instancia de 50 max connections

Incluso durante los periodos de mayor actividad, Cloud SQL conservÃ³ margen
disponible tanto en CPU como en nÃºmero de conexiones.
Por lo tanto, las mÃ©tricas observadas no muestran evidencia de que Cloud SQL
haya alcanzado su capacidad mÃ¡xima durante el Escenario 1.
Sin embargo, la API utiliza un pool de conexiones limitado mediante:
DB_MAX_CONNS=5
DB_MIN_CONNS=0

Esto significa que puede existir espera dentro de la propia aplicaciÃ³n antes de
que las solicitudes lleguen a PostgreSQL. En consecuencia, un nÃºmero bajo de
conexiones observado en Cloud SQL no permite descartar completamente una
posible contenciÃ³n en el pool de conexiones de la API.

### MÃ©tricas de la VM Web durante el Escenario 1

El Escenario 1 fue ejecutado en dos ventanas aproximadas durante la tarde del
27 de septiembre: una entre las 16:00 y 16:30 y otra entre las 16:30 y 17:00.

Durante la primera ejecuciÃ³n, la utilizaciÃ³n de CPU de la instancia
`mooc-e2-web` permaneciÃ³ baja durante la mayor parte de la prueba, generalmente
en valores de un dÃ­gito, con algunos incrementos puntuales.

Durante la segunda ejecuciÃ³n se observa una carga mayor sobre la VM. Entre
aproximadamente las 16:45 y las 17:00, la utilizaciÃ³n de CPU aumenta de forma
mÃ¡s sostenida y alcanza valores cercanos al 35 %â€“40 %.

Aunque la segunda ejecuciÃ³n incrementÃ³ claramente el uso de CPU respecto a la
primera, la instancia no alcanzÃ³ niveles cercanos a saturaciÃ³n. Por lo tanto,
las mÃ©tricas observadas no muestran evidencia de que la CPU del Web Server haya
sido el lÃ­mite fÃ­sico principal durante el Escenario 1.

Al combinar este resultado con las mÃ©tricas de Cloud SQL, donde tampoco se
observÃ³ agotamiento del lÃ­mite global de conexiones ni saturaciÃ³n de CPU, la
degradaciÃ³n observada bajo mayor concurrencia parece ocurrir antes de alcanzar
los lÃ­mites fÃ­sicos de CPU de la VM Web o de la instancia de base de datos.

Una hipÃ³tesis relevante continÃºa siendo la contenciÃ³n en recursos internos de
la aplicaciÃ³n, especialmente el pool de conexiones PostgreSQL configurado con:

```text
DB_MAX_CONNS=5
DB_MIN_CONNS=0
```
Con este lÃ­mite, solicitudes concurrentes pueden quedar esperando una conexiÃ³n
disponible aunque Cloud SQL todavÃ­a tenga capacidad libre. Esta hipÃ³tesis no
puede confirmarse Ãºnicamente con las mÃ©tricas actuales y requerirÃ­a
instrumentaciÃ³n adicional del pool de conexiones.

### Costos observados

Durante la ejecuciÃ³n de la Entrega 2 se revisÃ³ el consumo acumulado del proyecto
`proyecto1-entrega2-desarrollo` en Google Cloud Billing.

Los costos observados fueron:

| Servicio | Costo por uso | Ahorros/crÃ©ditos | Subtotal |
|---|---:|---:|---:|
| Cloud SQL | USD 3.45 | -USD 3.45 | USD 0.00 |
| Networking | USD 0.01 | -USD 0.01 | USD 0.00 |

El principal componente de costo observado fue Cloud SQL. Aunque el costo por
uso acumulado alcanzÃ³ USD 3.45, los crÃ©ditos o ahorros aplicados compensaron el
valor durante el periodo analizado, por lo que el subtotal facturado mostrado
por Billing fue de USD 0.00.

TambiÃ©n se configurÃ³ un presupuesto mensual de:

```text
USD 20
```

con alertas de gasto real en:
50 %
75 %
90 %

AdemÃ¡s, para Compute Engine y Cloud Storage no se mostraban costo desglosado en la captura disponible al momento de la revisiÃ³n.
