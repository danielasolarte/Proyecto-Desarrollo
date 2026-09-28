# Estimación de costos

## Contexto

Proveedor: Google Cloud Platform  
Proyecto: `proyecto1-entrega2-desarrollo`  
Región principal: `us-central1`  
Zona de las VMs: `us-central1-a`  
Fecha de actualización: 2026-09-27

Esta estimación corresponde al despliegue de la Entrega 2 del proyecto MOOC.
El objetivo es registrar la infraestructura utilizada, el costo observado
durante las pruebas y las principales decisiones que afectan capacidad y costo.

Los valores de facturación corresponden al estado observado antes del cierre
definitivo de los recursos y pueden variar ligeramente después de ejecutar las
últimas pruebas del Escenario 2.

---

## Recursos desplegados

| Recurso | Servicio GCP | Configuración actual | Estado |
|---|---|---|---|
| Web Server | Compute Engine | `mooc-e2-web`, zona `us-central1-a` | Implementado |
| Worker Server | Compute Engine | `mooc-e2-worker`, zona `us-central1-a`, IP interna `10.20.0.2`, IP externa `34.28.33.182` | Implementado |
| Base de datos | Cloud SQL for PostgreSQL | PostgreSQL 16, `db-g1-small`, 1 vCPU, 1.7 GB RAM, 10 GB SSD, sin HA | Implementado |
| Multimedia | Cloud Storage | Bucket `mooc-e2-media-proyecto1-entrega2`, región `us-central1`, clase Standard | Implementado |
| Red | VPC | `mooc-e2-vpc` | Implementado |
| Subred | VPC subnet | `mooc-e2-subnet`, rango `10.20.0.0/24` | Implementado |
| Cloud NAT | Network Services | Cloud NAT configurado para la red del despliegue | Implementado |
| Redis | Contenedor en Worker VM | Redis 7 | Implementado |
| ClamAV | Contenedor en Worker VM | `clamav/clamav:stable` | Implementado |
| Worker multimedia | Contenedor en Worker VM | Imagen `mooc-e2-worker:latest`, concurrencia 5 | Implementado |
| Proxy HTTPS | Caddy | Ejecutándose en Web Server | Implementado |

---

## Cloud SQL

La base de datos administrada utiliza Cloud SQL for PostgreSQL 16 en
`us-central1`.

Configuración observada:

- PostgreSQL 16.
- Tipo de máquina: `db-g1-small`.
- 1 vCPU.
- 1.7 GB de memoria.
- 10 GB de almacenamiento SSD.
- Instancia de una sola zona.
- Alta disponibilidad deshabilitada.
- Backups automáticos habilitados.
- SSL requerido para conexiones externas.
- IP pública de Cloud SQL: `34.42.6.180`.

La instancia fue utilizada para ejecutar las migraciones del proyecto y para
las pruebas de carga de la Entrega 2.

---

## Límite y control de conexiones

La instancia de Cloud SQL reportó:

```text
max_connections = 50
```

La API y el worker permiten configurar sus pools de PostgreSQL mediante:
DB_MAX_CONNS
DB_MIN_CONNS

La configuración base utilizada es:
DB_MAX_CONNS=5
DB_MIN_CONNS=0

Por lo tanto, API y worker podrían utilizar hasta 10 conexiones de base de
datos en conjunto bajo esta configuración.
Esto mantiene un margen respecto al límite global de 50 conexiones de Cloud
SQL para tareas administrativas, migraciones y otros procesos.
El worker también utiliza:
WORKER_CONCURRENCY=5

La concurrencia del worker puede ajustarse independientemente del pool de
PostgreSQL, pero debe mantenerse alineada con la capacidad de CPU, memoria,
Redis, Cloud Storage y Cloud SQL.
Resultados de capacidad relacionados con costos
Durante el Escenario 1 se realizaron pruebas con diferentes niveles de carga.
VUs	Throughput aproximado	p95	p99	Fallos HTTP
10	3.90 req/s	246 ms	474 ms	0.186 %
25	9.08 req/s	220 ms	411 ms	0.198 %
50	17.54 req/s	225 ms	411 ms	0.198 %
100	23.38 req/s	1 049 ms	29 998 ms	1.763 %


Entre 10 y 50 VUs el comportamiento fue estable.
En 100 VUs se observó un incremento importante de latencia y errores, aunque
las métricas de infraestructura no mostraron saturación física evidente en
Cloud SQL ni en CPU de la VM Web.
Por esta razón, no se justifica aumentar automáticamente el tamaño de la
infraestructura únicamente por los resultados de 100 VUs.
Antes de escalar recursos se recomienda instrumentar la aplicación y el pool de
conexiones para identificar con mayor precisión el origen de la contención.
Este enfoque evita aumentar costos de infraestructura sin evidencia de que CPU,
memoria o capacidad de Cloud SQL sean realmente el cuello de botella.
Métricas de Cloud SQL observadas
Durante las ejecuciones del Escenario 1, aproximadamente entre las 16:00 y las
17:00 del 27 de septiembre, Cloud SQL mantuvo un uso de CPU bajo.
Se observaron incrementos moderados durante las pruebas, pero la utilización se
mantuvo ampliamente por debajo de la capacidad disponible de la vCPU.
El número total de conexiones también permaneció muy por debajo del límite:
max_connections = 50

Por lo tanto, durante el Escenario 1 no se observó evidencia de saturación de
Cloud SQL por CPU ni por agotamiento del límite global de conexiones.
Métricas de la VM Web
Durante el Escenario 1 la VM mooc-e2-web mostró niveles bajos y moderados de
utilización de CPU.
En la primera ventana de prueba, entre aproximadamente las 16:00 y las 16:30,
la CPU se mantuvo mayoritariamente en valores de un dígito.
Durante la segunda ventana, entre aproximadamente las 16:30 y las 17:00, se
observó un aumento de carga, con valores cercanos al 35 %–40 % en los momentos
de mayor actividad.
No se observó saturación sostenida de CPU.
No fue posible obtener métricas de utilización de memoria de la VM Web desde
Cloud Monitoring durante la prueba. Por esta razón, el análisis de capacidad
no permite descartar completamente la memoria como posible factor de
degradación.
Worker Server
El Worker Server mooc-e2-worker ejecuta mediante Docker Compose:
- worker multimedia;
- Redis 7;
- ClamAV.
El worker fue verificado en ejecución con:
Worker de multimedia con concurrencia 5
Worker de multimedia escuchando en la cola 'media'

Redis respondió correctamente:
PONG

Durante la validación previa a las pruebas, las colas mostraron:
pending = 0
active  = 0

lo cual es esperado cuando no existen trabajos multimedia en ejecución.
La medición definitiva de profundidad de cola, throughput y tiempo de
procesamiento se completa con el Escenario 2.
Cloud Storage
El almacenamiento multimedia utiliza el bucket:
mooc-e2-media-proyecto1-entrega2

Configuración observada:
- región: us-central1;
- clase: Standard;
- acceso público general deshabilitado;
- utilizado para almacenar archivos multimedia y resultados procesados por el
  worker.
El costo de Cloud Storage dependerá principalmente de:
- cantidad de objetos almacenados;
- tamaño de archivos multimedia;
- operaciones de lectura/escritura;
- transferencia de datos.
Debido al volumen reducido de un entorno académico de pruebas, se espera que
Cloud Storage represente una fracción menor del costo total frente a Cloud SQL
y Compute Engine.
Red desplegada
La infraestructura utiliza una VPC dedicada:
VPC: mooc-e2-vpc
Subred: mooc-e2-subnet
CIDR: 10.20.0.0/24

La migración desde la red default cambió las direcciones internas y externas
de las VMs.
El Worker Server quedó con:
IP interna: 10.20.0.2
IP externa: 34.28.33.182

La red incluye reglas de firewall específicas para comunicación entre Web,
Worker y servicios necesarios.
Cloud NAT fue configurado como mecanismo de salida para recursos privados.
Sin embargo, durante la entrega el Worker Server conserva también una IP
externa, por lo que Cloud NAT no constituye actualmente su única ruta de salida
a Internet.
Respaldo y recuperación
Además de los backups automáticos de Cloud SQL, se realizó una prueba manual de
respaldo y restauración.
El procedimiento fue:
1. generar un backup mediante pg_dump;
2. validar el archivo mediante pg_restore --list;
3. crear una base temporal;
4. restaurar el backup mediante pg_restore;
5. verificar las tablas restauradas;
6. eliminar la base temporal después de la prueba.
El procedimiento completo se encuentra documentado en:
docs/entrega2/operacion-recuperacion.md

Esto permite demostrar que el respaldo no solo fue generado, sino que su
recuperación fue probada.
El archivo binario del backup se conserva fuera del repositorio Git para evitar
versionar datos de la base de datos.
Presupuesto y alertas
Se configuró un presupuesto mensual de:
USD 20

con alertas de gasto real en:
50 %
75 %
90 %

El presupuesto funciona como mecanismo de monitoreo y alerta.
No constituye un límite automático de consumo ni detiene los recursos cuando se
alcanza alguno de los umbrales.
Costos observados
Durante la ejecución de la Entrega 2 se revisó el consumo acumulado del proyecto
proyecto1-entrega2-desarrollo en Google Cloud Billing.
Los costos observados fueron:
Servicio	Costo por uso	Ahorros/créditos	Subtotal
Cloud SQL	USD 3.45	-USD 3.45	USD 0.00
Networking	USD 0.01	-USD 0.01	USD 0.00


El principal componente de costo observado fue Cloud SQL.
Aunque el costo por uso acumulado alcanzó USD 3.45, los créditos o ahorros
aplicados compensaron el valor durante el periodo analizado y el subtotal
mostrado por Billing fue de USD 0.00.
Networking presentó un costo por uso de USD 0.01, igualmente compensado por
créditos o ahorros.
En la captura de Billing disponible al momento de la revisión no se mostraba un
costo desglosado para Compute Engine ni Cloud Storage.
Esto no implica necesariamente que estos servicios tengan un costo definitivo
de USD 0.00, ya que algunos cargos pueden aparecer posteriormente en Billing.
Los valores observados corresponden al estado del proyecto antes del cierre
definitivo de la entrega.
Supuestos de costo
La solución fue diseñada como un entorno académico y no como una plataforma de
producción de alta disponibilidad.
Los principales supuestos utilizados son:
- región principal us-central1;
- dos VMs de Compute Engine;
- una instancia pequeña de Cloud SQL;
- sin alta disponibilidad de Cloud SQL;
- almacenamiento multimedia en Cloud Storage Standard;
- Redis y ClamAV ejecutados dentro del Worker Server;
- sin balanceador de carga;
- sin autoscaling;
- recursos activos únicamente durante desarrollo, pruebas y sustentación;
- eliminación o detención de recursos después de la entrega.
Estas decisiones reducen el costo, pero también limitan disponibilidad,
tolerancia a fallos y capacidad máxima.
Limitaciones
Cloud SQL sin alta disponibilidad
Cloud SQL utiliza una sola zona y no tiene alta disponibilidad.
Esto reduce costos, pero introduce un punto único de falla para la capa de
persistencia.
En un entorno productivo debería evaluarse una configuración regional con alta
disponibilidad y una política formal de recuperación.
Pool de conexiones
La API y el worker utilizan pools limitados a cinco conexiones por servicio.
Esta configuración protege una instancia pequeña de Cloud SQL frente a una
cantidad excesiva de conexiones.
Sin embargo, bajo alta concurrencia también puede introducir espera dentro de
la aplicación antes de acceder a PostgreSQL.
Las métricas actuales no permiten confirmar que este pool haya sido el cuello
de botella del Escenario 1.
Se recomienda instrumentar tiempos de adquisición de conexiones antes de
aumentar DB_MAX_CONNS.
Worker con IP pública
Aunque la solución dispone de una VPC dedicada y Cloud NAT, el Worker Server
conserva una dirección IP pública durante la entrega.
Esto simplifica ciertas tareas de configuración y acceso, pero incrementa la
superficie de exposición de la infraestructura.
Una evolución recomendada sería retirar la IP pública del worker y utilizar
únicamente conectividad privada y Cloud NAT para tráfico saliente.
Memoria de la VM Web no observable
Durante el Escenario 1 no fue posible obtener métricas de utilización de
memoria de mooc-e2-web.
Por lo tanto, no fue posible descartar completamente presión de memoria como
factor de degradación.
El análisis se realizó con las métricas disponibles de CPU, conexiones,
latencia y errores.
Escalabilidad vertical limitada
Las máquinas utilizadas corresponden a un entorno pequeño de pruebas.
Si la carga aumenta significativamente, los recursos que deben observarse antes
de escalar son:
- CPU Web Server;
- memoria Web Server;
- CPU Worker Server;
- profundidad de cola Redis/asynq;
- tiempos de procesamiento multimedia;
- conexiones PostgreSQL;
- CPU e I/O de Cloud SQL.
El escalamiento debe hacerse a partir de evidencia de métricas y no únicamente
del número de usuarios concurrentes.
Persistencia de Redis
Redis se ejecuta dentro del Worker Server.
Esta configuración es suficiente para la entrega académica, pero no ofrece las
mismas garantías de disponibilidad que un servicio administrado.
Una falla o recreación de la VM podría afectar el estado de trabajos en cola.
En un entorno productivo se debería evaluar un servicio administrado como
Memorystore o una estrategia de persistencia y recuperación más robusta.
Estrategia de escalamiento
Si el sistema necesitara soportar una carga superior a la evaluada, el orden
recomendado de análisis sería:
1. identificar el recurso realmente saturado mediante Cloud Monitoring;
2. instrumentar adquisición y uso del pool PostgreSQL;
3. ajustar gradualmente DB_MAX_CONNS si se confirma espera por conexiones;
4. medir nuevamente Cloud SQL después de cada cambio;
5. aumentar CPU o memoria de la Web VM solo si las métricas muestran
   saturación;
6. ajustar WORKER_CONCURRENCY según CPU y profundidad de cola;
7. escalar horizontalmente workers si los trabajos multimedia se acumulan;
8. aumentar la capacidad de Cloud SQL únicamente cuando CPU, memoria, I/O o
   conexiones indiquen que la instancia actual se aproxima a su límite;
9. evaluar alta disponibilidad para un entorno productivo.
Esta estrategia evita incrementar costos sin evidencia de beneficio.
Relación costo-capacidad
Los resultados de la Entrega 2 muestran que el despliegue actual puede manejar
cargas moderadas con recursos pequeños y un costo limitado.
En el Escenario 1, el sistema mantuvo un comportamiento estable hasta 50 VUs
sin saturar CPU de Web Server ni Cloud SQL.
La degradación observada en 100 VUs ocurrió antes de alcanzar los límites
físicos observables de estas máquinas.
Por esta razón, aumentar inmediatamente el tamaño de las VMs o Cloud SQL no
sería la primera acción recomendada.
La decisión de escalar debe estar respaldada por instrumentación adicional y
nuevas pruebas de carga.
Cierre de costos
Antes de eliminar los recursos del proyecto se debe realizar una última
revisión de Google Cloud Billing para registrar el costo acumulado final.
Después de finalizar:
- Escenario 1;
- Escenario 2;
- métricas;
- documentación;
- video;
- commit y push final;
se debe proceder al cierre de recursos.
En particular, la instancia de Cloud SQL debe eliminarse después de conservar
un respaldo válido.
También debe documentarse cuáles recursos permanecen activos después de la
entrega y cuáles podrían continuar generando costos.
La eliminación de recursos constituye parte del cierre operativo de la Entrega
2 y debe realizarse únicamente después de verificar que no se requieren más
pruebas.

### Resultados de capacidad del Escenario 2

El Escenario 2 evaluó el procesamiento multimedia asíncrono mediante el
Worker Server desplegado en GCP, utilizando los niveles `baseline` y `l1`.

Los resultados observados fueron:

| Métrica | Baseline | L1 |
|---|---:|---:|
| Peticiones HTTP | 226 | 322 |
| Fallos HTTP | 0 % | 2.17 % |
| Latencia HTTP p95 | 924 ms | 1 108 ms |
| Latencia HTTP p99 | 1 898 ms | ~60 000 ms |
| Upload → ready promedio | 38.9 s | 46.5 s |
| Upload → ready p95 | 120.9 s | 138.9 s |
| Errores HLS | 0 % | 3.88 % |

El flujo multimedia funcionó correctamente en `baseline`, pero al aumentar la
carga al nivel `l1` se observó degradación en los tiempos de procesamiento,
aparición de timeouts y errores HLS.

Durante estas pruebas, la VM `mooc-e2-worker` presentó una utilización de CPU
significativamente mayor que la observada en otros componentes. Se registraron
periodos sostenidos de alta utilización y múltiples picos durante las tareas de
procesamiento multimedia.

En contraste, Cloud SQL mantuvo un uso de CPU aproximadamente entre 8 % y 10 %
y el número de conexiones permaneció ampliamente por debajo del límite de:

```text
max_connections = 50
```

Por lo tanto, el principal límite de capacidad observado en el Escenario 2 se
encuentra en el Worker Server y no en Cloud SQL.
Desde la perspectiva de costo-capacidad, esto implica que un eventual
escalamiento debería priorizar la capa de procesamiento multimedia antes que
aumentar el tamaño de la base de datos.
Las alternativas a evaluar serían:
1. ajustar WORKER_CONCURRENCY;
2. aumentar CPU y/o memoria del Worker Server;
3. distribuir trabajos entre múltiples workers si la cola crece de forma
   sostenida;
4. mantener Cloud SQL en la configuración actual mientras continúe mostrando
   capacidad disponible.