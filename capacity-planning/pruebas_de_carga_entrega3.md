# Pruebas de carga — Entrega 3

## 1. Objetivo

El objetivo de las pruebas de carga de la Entrega 3 es evaluar el comportamiento de la aplicación MOOC sobre la nueva arquitectura elástica desplegada en Google Cloud Platform.

A diferencia de la Entrega 2, donde la capacidad se evaluó sobre una configuración fija, en esta entrega la API y los workers se ejecutan en Managed Instance Groups (MIG) independientes con escalamiento automático entre 1 y 3 instancias.

Las pruebas buscan determinar:

el comportamiento del sistema ante niveles crecientes de carga;
la capacidad del autoscaling para responder ante aumentos y disminuciones de demanda;
el efecto de la nueva arquitectura sobre los resultados obtenidos en la Entrega 2;
el comportamiento de la cola multimedia durante el escalamiento;
la integridad de los trabajos durante el retiro de workers;
los principales cuellos de botella observados.

---

## 2. Entorno de pruebas

### 2.1 Infraestructura

La arquitectura utilizada para la Entrega 3 está compuesta por:

| Componente | Configuración |
|---|---|
| API | Managed Instance Group de 1 a 3 instancias |
| Workers | Managed Instance Group de 1 a 3 instancias |
| Tipo de máquina API | `e2-small` |
| Tipo de máquina worker | `e2-medium` |
| Concurrencia por worker | `2` |
| Base de datos | Cloud SQL PostgreSQL 16 |
| Tier Cloud SQL | `db-g1-small` |
| Redis | VM de servicios |
| ClamAV | VM de servicios |
| Archivos multimedia | Cloud Storage |
| Balanceador | HTTPS Load Balancer |
| Generador de carga | Equipo externo a las VM de la aplicación |
| Herramienta | k6 v2.3.0 mediante Docker Desktop |

La cola multimedia es compartida por todas las réplicas y se encuentra fuera del MIG de workers. De esta forma, la creación o eliminación de una instancia no elimina el estado pendiente del procesamiento.

### 2.2 Autoscaling de workers

El MIG de workers utiliza una métrica personalizada basada en:

```text

demanda = trabajos pendientes + trabajos activos

```

La configuración utilizada es:

| Parámetro | Valor |
|---|---:|
| Instancias mínimas | 1 |
| Instancias máximas | 3 |
| Objetivo | ~4 trabajos por instancia |
| Concurrencia por worker | 2 |
| Frecuencia del exportador | 30 s |
| Scale-in | Máximo 1 instancia cada 10 minutos |
| Shutdown timeout | Hasta 300 s |
| Cola | `media` |

La inclusión de los trabajos activos en la señal evita interpretar una cola sin trabajos pendientes como ausencia de carga cuando los workers todavía se encuentran procesando.

---

## 3. Metodología

Para conservar la comparabilidad con la Entrega 2 se mantienen, en los escenarios comparables:

k6 v2.3.0;
los mismos datos sintéticos;
la misma mezcla de operaciones;
las mismas pausas;
las mismas duraciones;
el mismo calentamiento;
los mismos perfiles multimedia;
concurrencia 2 por worker sobre `e2-medium`.

El generador se ejecuta fuera de las máquinas virtuales de la aplicación.

Todas las solicitudes HTTP dirigidas a la API deben ingresar a través del balanceador HTTPS. Las transferencias de archivos multimedia se realizan directamente contra Cloud Storage mediante las URL autorizadas por la aplicación.

Después del login, k6 utiliza el token Bearer correspondiente y descarta las cookies para evitar respuestas CSRF 403.

Las corridas comparadas deben repetirse. Los percentiles se reportan individualmente por corrida y no se promedian entre ejecuciones.

### 3.1 Métricas

Para cada corrida se registran, cuando corresponda:

p50;
p95;
p99;
throughput;
porcentaje de errores;
timeouts;
CPU;
memoria;
red;
disco;
conexiones a Cloud SQL;
profundidad de cola;
antigüedad del trabajo más antiguo;
trabajos activos;
trabajos pendientes;
reintentos;
número de instancias del MIG;
tiempo de escalamiento;
tiempo de procesamiento multimedia.

Para el procesamiento multimedia también se consideran:

tiempo de autorización de carga;
tiempo de transferencia;
tiempo de confirmación;
tiempo `upload → ready`;
errores de generación HLS;
errores de manifest;
errores de segmentos HLS.

---

# 4. Escenario 1 — Actividad académica

**Responsable:** Samara  
**Comparación:** Escenario 1 de la Entrega 2.

Este escenario reproduce la carga académica utilizada en la Entrega 2, pero dirige las solicitudes al balanceador HTTPS que distribuye las peticiones entre las réplicas del MIG de API.

## 4.1 Parámetros

Se repitió el Escenario 1 de la Entrega 2 con el mismo script y los mismos parámetros; solo cambia el destino (el balanceador HTTPS en lugar del Web Server fijo).

| Parámetro | Entrega 2 | Entrega 3 |
|---|---|---|
| Script | `k6/escenario1.js` | `k6/escenario1.js` (sin cambios en la carga; solo se agregó el código HTTP al mensaje de error del login de setup) |
| Runner | `k6/run-escenario1.ps1` (Docker) | `k6/run-escenario1.ps1 -Nativo` (k6 instalado en Windows, porque Docker Desktop/WSL falló en el equipo de prueba) |
| Versión de k6 | v2.3.0 | v2.3.0 |
| Niveles (VUs totales) | 10, 25, 50, 100 | 10, 25, 50, 100 |
| Mezcla por nivel | 50 % lectores, 30 % aprendices, 20 % quiz | Igual |
| Calentamiento / sostenido | 30 s / 180 s | 30 s / 180 s |
| Pausa entre niveles | 60 s | 60 s |
| Destino | `https://35.254.78.215.sslip.io/api/v1` (Web Server e2-small fijo) | `https://136.68.98.101.sslip.io/api/v1` (balanceador → `mooc-api-mig`, e2-small, 1 a 3 instancias, objetivo 60 % de CPU) |
| Generador de carga | Equipo Windows fuera de las VM | Equipo Windows fuera de las VM |
| Fecha | Entrega 2 (ver `pruebas_de_carga_entrega2.md`) | 2026-10-31, UTC 14:55–15:51; el nivel 50 se repitió de 17:03 a 17:08 porque la primera corrida se degradó por una pérdida de conexión del equipo generador (1,4 req/s en lugar de ~17) |

Comando:

```powershell
.\k6\run-escenario1.ps1 -Tag e3 -BaseUrl "https://136.68.98.101.sslip.io/api/v1" -Levels 10,25,50,100 -AdminEmail "<admin de prueba>" -AdminPassword "<clave>" -Nativo
```

El grupo de la API arrancó cada nivel con 1 instancia. Durante esta corrida no se registró la línea de tiempo del grupo; el comportamiento del autoscaling se mide aparte en el Escenario 3A (sección 6).

Resultados crudos: `k6/results/e1-e3-{10,25,50,100}.json` y `.log`. Entrega 2: `k6/results/e1-gcp-{10,25,50,100}.json`.

## 4.2 Resultados

Métricas globales de k6 (`http_req_duration` y `http_req_failed` de toda la corrida), las mismas que se usaron en el informe de la Entrega 2.

| VUs | Entrega | Peticiones | req/s | p50 | p95 | p99 | Máx | Errores HTTP | Checks | Journeys |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | E2 | 1 078 | 3,9 | 135 ms | 246 ms | 474 ms | 0,7 s | 0,19 % | 99,93 % | 197 |
| 10 | **E3** | 1 094 | 4,0 | 123 ms | 212 ms | 424 ms | 3,0 s | **0 %** | **100 %** | 200 |
| 25 | E2 | 2 520 | 9,1 | 126 ms | 220 ms | 411 ms | 0,6 s | 0,20 % | 99,85 % | 489 |
| 25 | **E3** | 2 563 | 9,4 | 116 ms | 154 ms | 399 ms | 1,7 s | **0 %** | **100 %** | 498 |
| 50 | E2 | 5 039 | 17,5 | 123 ms | 225 ms | 411 ms | 5,3 s | 0,20 % | 99,86 % | 1 005 |
| 50 | **E3** | 4 993 | 17,9 | 130 ms | 234 ms | 403 ms | 3,4 s | **0 %** | **100 %** | 996 |
| 100 | E2 | 6 808 | 23,4 | 123 ms | 1 049 ms | 29 998 ms | 30,0 s | 1,76 % | 98,57 % | 1 332 |
| 100 | **E3** | **10 082** | **35,5** | 116 ms | **147 ms** | **403 ms** | 3,3 s | **0 %** | **100 %** | **2 036** |

Los umbrales del script (`p(95)` por operación, `checks > 99 %`, `functional_errors < 1 %`, `dup_submit_* = 0`, `progress_regressions = 0`) se cumplieron en 25, 50 y 100 VUs. En 10 VUs, `course_preview` y `quiz_answer` superaron su umbral de p95 por pocos valores altos sobre una muestra pequeña; no hubo errores.

## 4.3 Análisis

- **De 10 a 50 VUs** el comportamiento es equivalente al de la Entrega 2: el throughput crece de forma lineal con la carga y la mediana se mantiene en 116–130 ms. La diferencia visible es la tasa de error, que pasa de ~0,2 % a **0 %** en todos los niveles, con 100 % de checks.
- **En 100 VUs desaparece el punto de saturación de la Entrega 2.** En la E2 el Web Server fijo se saturó: p95 de 1 049 ms, p99 en el timeout de 30 s y 1,76 % de errores. En la E3, con el mismo nivel, el p95 queda en 147 ms, el p99 en 403 ms y no hay errores, y el sistema atiende **48 % más peticiones** (10 082 frente a 6 808) y **53 % más journeys completos** (2 036 frente a 1 332).
- **Por qué mejora:** la API ya no comparte la VM con Caddy ni con el Mailpit publicado, Redis y ClamAV salieron a la VM de servicios, el balanceador termina TLS fuera de las VM, y el grupo puede sumar réplicas cuando la CPU pasa del 60 %. Todas las réplicas comparten sesiones (Redis) y estado de negocio (Cloud SQL), así que el balanceador reparte sin afinidad.
- **Costo del salto adicional:** el balanceador agrega un salto de red; no se observa en la mediana (116–130 ms en ambas entregas). Los máximos ocasionales de 1,7 a 3,4 s en la E3 son eventos aislados que no mueven el p99.
- **Correctitud bajo carga:** igual que en la E2, `dup_submit_double_grading = 0`, `dup_submit_inconsistent = 0` y `progress_regressions = 0` en todos los niveles: los reintentos de envío de quiz no generan calificaciones duplicadas aunque las solicitudes lleguen a réplicas distintas.

---

# 5. Escenario 2 — Carga y procesamiento multimedia

**Responsable:** Camila  
**Acompañamiento:** Mateo

## 5.1 Objetivo

El Escenario 2 evalúa el flujo completo de carga y procesamiento de recursos multimedia sobre la arquitectura elástica.

El escenario estudia simultáneamente:

1. solicitudes a la API;
2. transferencia del archivo hacia Cloud Storage;
3. creación de trabajos en la cola multimedia;
4. procesamiento por los workers;
5. transición del recurso hasta estado `ready`;
6. consumo del contenido HLS procesado.

La prueba permite comparar la arquitectura elástica de E3 contra el resultado corregido C4 de la Entrega 2.

---

## 5.2 Línea base de la Entrega 2

Durante la corrección C4 se identificó que el worker `e2-small`, que además alojaba Redis, no podía sostener adecuadamente el procesamiento multimedia.

La configuración se corrigió a:

```text

Worker: e2-medium

WORKER_CONCURRENCY=2

```

Con esta configuración se obtuvieron corridas con:

```text

278 / 278 checks correctos

300 / 300 checks correctos

0 % de errores HTTP

```

Esta configuración constituye la línea base para la comparación con E3.

Los valores detallados de latencias y tiempos de procesamiento de E2 deben copiarse de los resultados originales de C4 antes de cerrar el informe.

---

## 5.3 Perfiles de video

Se utilizan los mismos perfiles definidos en la corrección C4:

| Perfil | Resolución | Duración |
|---|---|---:|
| Corto | 640×360 | 15 s |
| Medio | 1280×720 | 30 s |
| Largo | 1920×1080 | 30 s |

Archivos esperados:

```text

media/corto_360p_15s.mp4

media/medio_720p_30s.mp4

media/largo_1080p_30s.mp4

```

---

## 5.4 Niveles de carga

El script `k6/escenario2.js` define los siguientes niveles:

| Nivel | Uploaders | Viewers | Duración |
|---|---:|---:|---:|
| Baseline | 1 | 2 | 2 min |
| L1 | 2 | 5 | 3 min |
| L2 | 4 | 10 | 3 min |
| L3 | 6 | 20 | 3 min |
| Peak | 8 | 30 | 3 min |

Entre corridas se conserva la pausa definida para permitir estabilización del sistema.

---

## 5.5 Estado inicial de E3

Antes de introducir carga se verificó el MIG de workers.

Resultado observado:

```text

Instancias: 1

Health: HEALTHY

Autoscaler: ACTIVE

Concurrencia: 2

Cola escuchada: media

demanda: 0

pendientes: 0

activos: 0

reintentos: 0

```

Durante la validación del arranque del worker se obtuvo:

```text

Tiempo desde creación de la VM hasta contenedor worker listo: 51 s

```

El health check del worker respondió:

```json

{"status":"ok"}

```

Esto establece el estado T0 de la prueba.

---

## 5.6 Resultados de E3

Las corridas oficiales se ejecutaron contra el balanceador HTTPS usando k6 v2.3.0 y los tres perfiles multimedia definidos para C4. Los resultados consolidados fueron:

| Nivel | Checks OK | Checks fallidos | Error HTTP | `upload → ready` promedio | `upload → ready` p95 | `upload → ready` máximo | HLS segmentos p95 |
|---|---:|---:|---:|---:|---:|---:|---:|
| Baseline | 106 | 0 | 0 % | 18.02 s | 33.60 s | 33.62 s | 2.63 s |
| L1 | 312 | 1 | 0.18 % | 26.89 s | 68.13 s | 99.25 s | 1.51 s |
| L2 | 544 | 0 | 0 % | 47.93 s | 143.94 s | 152.64 s | 2.68 s |
| L3 | 1024 | 0 | 0 % | 45.04 s | 100.00 s | 164.34 s | 3.24 s |

### Baseline

La corrida baseline finalizó con 106 de 106 checks correctos y 0 % de errores HTTP. Se procesaron correctamente las operaciones de autorización de carga, obtención de URL de parte, transferencia directa a Cloud Storage y confirmación. También se validaron reproducción, manifiesto y segmentos HLS sin fallos.

Métricas adicionales de baseline:

| Métrica | Resultado |
|---|---:|
| Peticiones HTTP | 203 |
| Throughput HTTP | 0.91 req/s |
| Latencia HTTP mediana | 50.08 ms |
| Latencia HTTP p95 | 1.51 s |
| Latencia HTTP p99 | 2.57 s |
| Autorización de upload promedio | 131 ms |
| Transferencia de upload promedio | 518 ms |
| Confirmación de upload promedio | 187 ms |
| HLS manifest p95 | 154 ms |
| HLS segmento p95 | 2.63 s |
| Errores HLS | 0 % |
| Fallos funcionales de upload | 0 % |

### Nivel L1

L1 completó 312 de 313 checks. Se presentó un único fallo en una transferencia `storage PUT`, equivalente a 0.18 % de requests HTTP fallidas.

El indicador `upload_functional_failures` fue 1.49 %, por debajo del threshold configurado de 5 %, por lo que la corrida cumplió los criterios funcionales definidos para el escenario.

| Métrica | Resultado |
|---|---:|
| Peticiones HTTP | 527 |
| Throughput HTTP | 1.73 req/s |
| Latencia HTTP mediana | 70.11 ms |
| Latencia HTTP p95 | 1.13 s |
| Latencia HTTP p99 | 1.81 s |
| `upload → ready` promedio | 26.89 s |
| `upload → ready` p95 | 68.13 s |
| Fallos funcionales de upload | 1.49 % |
| Errores HLS | 0 % |

### Nivel L2

L2 completó 544 checks sin fallos y con 0 % de errores HTTP.

El principal efecto de la mayor carga se observó en el tiempo asíncrono de procesamiento: el p95 de `upload → ready` aumentó hasta 143.94 s.

Esto muestra que, aunque el flujo HTTP se mantuvo funcional, el sistema comenzó a acumular espera en el procesamiento multimedia.

### Nivel L3

L3 completó 1024 checks sin fallos y con 0 % de errores HTTP.

Aunque la carga fue mayor que en L2, el p95 de `upload → ready` bajó de 143.94 s a 100.00 s.

Durante este período el MIG de workers incorporó capacidad adicional, por lo que la reducción observada es consistente con el efecto esperado del escalamiento horizontal.

### Interpretación de los resultados

La API y el flujo HLS se mantuvieron funcionales en todos los niveles probados. La degradación principal apareció en el tiempo `upload → ready`, no en errores HTTP sostenidos.

El crecimiento de `upload → ready` entre baseline y L2 muestra que el cuello de botella relevante es el procesamiento asíncrono multimedia. Sin embargo, la mejora observada entre L2 y L3, aun con mayor carga, coincide temporalmente con la incorporación de workers adicionales por el autoscaler.

Esto permite diferenciar claramente dos comportamientos:

- la latencia HTTP de la API permanece relativamente estable y con tasa de errores baja;
- el tiempo de procesamiento multimedia aumenta cuando se acumulan trabajos, pero puede recuperarse al aumentar horizontalmente el número de workers.

---

## 5.7 Comparación E2 vs. E3

La comparación se realiza contra la corrección C4 de la Entrega 2, que ya utilizaba `e2-medium` y concurrencia 2 para el worker.

De esta forma, el cambio principal estudiado en E3 es la capacidad de escalar horizontalmente.

| Aspecto | Entrega 2 | Entrega 3 | Observación |
|---|---|---|---|
| Worker inicial | 1 × `e2-medium` | 1 × `e2-medium` | Misma capacidad inicial |
| Concurrencia/worker | 2 | 2 | Se mantiene para comparabilidad |
| Workers máximos | 1 | 3 | E3 agrega escalamiento horizontal |
| Redis | Asociado al worker | VM de servicios | La cola queda desacoplada del ciclo de vida de los workers |
| Errores HTTP | 0 % en corridas C4 válidas | 0 % en baseline, L2 y L3; 0.18 % en L1 | E3 mantiene una tasa de errores baja |
| Checks | 278/278 y 300/300 en corridas C4 válidas | 106/106 baseline; 312/313 L1; 544/544 L2; 1024/1024 L3 | El flujo se mantiene funcional al crecer la carga |
| `upload → ready` | La E2 mostró degradación bajo carga y motivó el ajuste C4 | 33.60 s p95 baseline; 68.13 s L1; 143.94 s L2; 100.00 s L3 | Refleja acumulación de cola y posterior efecto del autoscaling |
| Escalamiento | No disponible | 1–3 workers automáticos | E3 puede añadir capacidad sin intervención manual |
| Retiro seguro | No aplica como grupo elástico | Validado en Scenario 3B | La cola y el trabajo sobreviven al retiro de una instancia |

Los resultados detallados de latencia de la corrección C4 de E2 deben mantenerse en el informe únicamente cuando se disponga de los archivos originales de esa entrega. No se estiman valores faltantes.

---

# 6. Escenario 3A — Autoscaling de la API

**Responsable:** Samara

## 6.1 Objetivo y configuración

Demostrar un ciclo completo de subida y bajada **automático** del grupo de la API (`mooc-api-mig`), con línea de tiempo, y que el balanceador excluye una instancia que falla el health check de tráfico y la reincorpora al recuperarse.

| Elemento | Valor |
|---|---|
| Grupo | `mooc-api-mig`, e2-small, zona us-central1-a, 1 a 3 instancias |
| Señal | CPU promedio del grupo, objetivo 60 % |
| Inicialización | 120 s |
| Reducción | 10 min de estabilización y máximo 1 VM retirada cada 600 s |
| Health check del balanceador | `mooc-api-hc-listo`: `GET /health/ready` cada 5 s, timeout 3 s, 2 aciertos / 2 fallos |
| Autorreparación | `mooc-api-hc-vida`: `GET /health` cada 10 s, timeout 5 s, 2 / 3, retraso inicial 180 s |
| Drenaje de conexiones | 30 s |

## 6.2 Carga y método

- **Carga:** `k6/escenario3a.js` con `k6/run-escenario3a.ps1` (k6 v2.3.0), ejecutor `ramping-arrival-rate` contra el balanceador: 5 req/s durante 2 min, subida a 30 req/s en 3 min, meseta de 12 min en 30 req/s y bajada a 0 en 1 min (18 min en total). Mezcla: 90 % `GET /catalog` y 10 % `POST /auth/login` con la cuenta de estudiante de prueba (bcrypt en el servidor, la operación que más CPU consume).
- **Línea de tiempo:** `deploy/gcp/e3/10-escenario3a.ps1 -Fase Monitor`, arrancado antes de la carga y durante 45 min. Cada ~15–30 s registra el tamaño objetivo del grupo, el tamaño recomendado por el autoscaler, las instancias y su acción, las instancias sanas en el balanceador, el reparto de 5 solicitudes por `X-Instance-Id` y la CPU del último minuto (Cloud Monitoring).
- **CPU:** `10-escenario3a.ps1 -Fase Cpu` (serie de `compute.googleapis.com/instance/cpu/utilization`, promedio por minuto).
- **Exclusión:** `deploy/gcp/e3/08-balanceador.ps1 -Fase Excluir -Segundos 60`, ejecutado durante la meseta con 2 instancias sanas.

Evidencias: `capacity-planning/results/e3-3a-timeline-20261031-124530.csv`, `e3-3a-cpu-20261031-133449.csv`, `e3-3a-exclusion-20261031-125402.csv` y `k6/results/e3a-20261031-124627.json` / `.log`.

## 6.3 Línea de tiempo (2026-10-31, UTC)

| Hora | Grupo | Evento | CPU por instancia (promedio del minuto) |
|---|---:|---|---|
| 17:45:30 | 1 | Inicio del monitor; `mooc-api-9gm4` sola | 9gm4 2,5 % |
| 17:46:27 | 1 | Inicio de la carga (5 req/s) | — |
| 17:48–17:50 | 1 | Subida a 30 req/s | 9gm4 16 % → 38 % |
| 17:50:46 | **1 → 2** | El autoscaler recomienda 2; se crea `mooc-api-6ssn` | 9gm4 38 % y subiendo |
| 17:51–17:52 | 2 | Pico de la primera instancia mientras arranca la segunda | 9gm4 64 % → 85 % |
| 17:51:44 | 2 | `6ssn` sana en el balanceador (**58 s** después de crearse); el tráfico se reparte | — |
| 17:54:36 | 2 | **Exclusión:** se bloquea Redis en `9gm4` (ver 6.4) | 9gm4 43 %, 6ssn 55 % |
| 17:54:46 | 2 | `9gm4` fuera del balanceador | — |
| 17:56:11 | **2 → 3** | El autoscaler recomienda 3; se crea `mooc-api-d14w`. `9gm4` vuelve a estar sana | 6ssn **95 %** (sola con el tráfico), 9gm4 5 % |
| 17:57:06 | 3 | `d14w` sana en el balanceador (**55 s**): tres instancias reciben tráfico | 6ssn 67 %, 9gm4 34 %, d14w 54 % |
| 17:58–18:04 | 3 | Meseta repartida entre las tres | 27 % a 46 % cada una |
| ~18:04:30 | 3 | Fin de la carga | baja a ~3 % en todas |
| 18:08:13 | **3 → 2** | El autoscaler recomienda 2 (≈ 4 min de CPU baja + estabilización) | ~3 % |
| 18:09:35 | 2 | `9gm4` sale del balanceador (drenaje) | — |
| 18:10:02 | 2 | `9gm4` retirada | — |
| 18:18:36 | **2 → 1** | Segunda reducción, **10 min 23 s** después de la primera (`scale-in-control`: 1 VM cada 600 s) | ~3 % |
| 18:19:59 | 1 | `d14w` sale del balanceador (drenaje) | — |
| 18:20:26 | 1 | `d14w` retirada; queda `6ssn` | 2,5 % |
| 18:30:26 | 1 | Fin del monitor, grupo estable en 1 | 2,6 % |

Resultado de la carga (k6, 18 min): 26 231 peticiones (24,3 req/s promedio), p50 124 ms, p95 236 ms, p99 325 ms, máximo 5,5 s; **0,04 % de errores** (11 de 26 231, todos en `POST /auth/login`) y 19 iteraciones descartadas por el ejecutor de llegadas.

## 6.4 Exclusión de una instancia que falla el health check

Con dos instancias sanas se bloqueó, en `mooc-api-9gm4`, la salida a Redis (`iptables -I OUTPUT -p tcp --dport 6379 -j REJECT`). Así `/health/ready` responde 503 (falla el ping a Redis), pero `/health` sigue en 200: el balanceador debe sacarla y la autorreparación **no** debe recrearla. El script envía 5 solicitudes cada ~3 s a `/api/v1/catalog` y registra qué instancia responde.

| Hora (UTC) | Fase | Observación |
|---|---|---|
| 17:54:06–17:54:22 | Antes | Reparto entre `6ssn` (11) y `9gm4` (14), todas 200 |
| 17:54:36 | Bloqueo | Redis bloqueado en `9gm4` |
| 17:54:44 | | Última respuesta de `9gm4` (~8 s después del bloqueo) |
| 17:54:54 | | El balanceador reporta `9gm4 = UNHEALTHY` |
| 17:54:47–17:55:40 | Excluida | 80 respuestas de `6ssn`, ninguna de `9gm4`, **todas 200** |
| 17:55:53 | Desbloqueo | Redis desbloqueado |
| 17:56:07 | Recuperada | `9gm4` vuelve a recibir tráfico; `HEALTHY` a las 17:56:12 (~14 s) |
| 17:56:13–17:56:33 | Recuperada | Reparto de nuevo entre las dos (35 y 15) |

La VM `9gm4` no fue recreada (siguió en el grupo hasta la reducción de las 18:10), lo que confirma la separación entre el health check de tráfico y el de autorreparación.

## 6.5 Análisis

- **Subida automática:** el grupo pasó de 1 a 2 cuando la CPU de la única instancia cruzó la zona del objetivo (38 % y subiendo, con pico de 85 % mientras arrancaba la segunda), y de 2 a 3 cuando la instancia que quedó sola durante la exclusión llegó a 95 %. **La segunda subida estuvo influida por la prueba de exclusión**, que concentró todo el tráfico en una sola réplica; sin ella, la meseta de 30 req/s probablemente se habría sostenido con 2 instancias (cada una entre 27 % y 46 % con 3).
- **Tiempo hasta atender:** 55–58 s desde la creación de la VM hasta recibir tráfico del balanceador (imagen de máquina con el contenedor precargado + 2 aciertos del health check). Es coherente con los 25–28 s de arranque del contenedor medidos en `03-grupo-api.ps1 -Fase Probar`, más el arranque del sistema operativo y la confirmación del health check.
- **Bajada automática y controlada:** la primera reducción llegó ~4 min después del fin de la carga y la segunda 10 min 23 s después, respetando `scale-in-control` (1 VM cada 600 s). En ambas, la instancia salió primero del balanceador y se eliminó ~27–30 s después: el drenaje de conexiones (30 s) y el apagado ordenado de la API (`/health/ready` en 503, espera de 10 s y hasta 20 s más) evitan cortar solicitudes en curso.
- **Errores:** los 11 errores (0,04 %) fueron todos logins. El login necesita Redis (sesiones) y el catálogo no; coinciden con la ventana de ~8 s de la exclusión en que `9gm4`, ya sin Redis, todavía recibía tráfico antes de que el balanceador la marcara como no sana (2 fallos × 5 s). Es el costo esperado de la detección por health check y queda dentro del umbral (< 5 %).
- **Sesiones entre réplicas:** las sesiones viven en Redis (VM de servicios) y no hay afinidad en el balanceador; la verificación `08-balanceador.ps1 -Fase Sesion` (login, `/me`, CSRF 403 sin encabezado, logout 204 y `/me` 401) pasó a través del balanceador.
- **Límite:** en el máximo de 3 instancias el autoscaler queda en `CAPPED_AT_MAX_NUM_REPLICAS`; más carga se traduciría en mayor latencia, no en rechazos del autoscaler. En esta prueba la meseta con 3 réplicas se mantuvo con p95 de 236 ms.

---

## 7.4 Línea de tiempo del autoscaling

La prueba se ejecutó generando carga multimedia con el nivel `peak`. El exportador de cola permitió observar la demanda en intervalos de 30 segundos.

Una muestra representativa de la corrida fue:

| Hora UTC | Demanda | Pendientes | Activos | Evento |
|---|---:|---:|---:|---|
| 03:34:33 | 1 | 0 | 1 | Comienzan trabajos activos |
| 03:35:33 | 7 | 3 | 4 | Aumenta la presión sobre la cola |
| 03:36:03 | 6 | 2 | 4 | Workers procesando a máxima concurrencia observada para 2 instancias |
| 03:36:33 | 8 | 4 | 4 | Se mantiene acumulación |
| 03:37:03 | 8 | 2 | 6 | Se observa capacidad equivalente a 3 workers × concurrencia 2 |
| 03:38:03 | 7 | 3 | 4 | La cola comienza a drenar |
| 03:39:03 | 4 | 0 | 4 | Sin pendientes, pero todavía hay trabajos activos |
| 03:40:03 | 1 | 0 | 1 | Último trabajo en ejecución |
| 03:40:33 | 0 | 0 | 0 | Procesamiento terminado |

Las operaciones del MIG también registraron creación y eliminación automática de instancias. Entre los eventos observados se encuentran:

```text
2026-10-28T20:17:44-07:00  insert  mooc-worker-f0sv  cloudservices.gserviceaccount.com
2026-10-28T20:31:06-07:00  delete  mooc-worker-f0sv  cloudservices.gserviceaccount.com
2026-10-28T20:36:28-07:00  insert  mooc-worker-zp1h  cloudservices.gserviceaccount.com
```

La creación por `cloudservices.gserviceaccount.com` confirma que las nuevas instancias fueron creadas por la infraestructura administrada y no mediante escalamiento manual.

Durante la corrida se observaron hasta tres workers en el MIG. El nuevo worker pasó por los estados de verificación de salud hasta quedar `HEALTHY / NONE` y comenzó a contribuir al procesamiento de la cola.

Después de finalizar la carga y drenar completamente la cola, el MIG redujo nuevamente su capacidad hasta el mínimo configurado de una instancia, validando el ciclo completo de scale-up y scale-down.

---

## 7.5 Retiro controlado con trabajo activo

Para validar el retiro seguro se ejecutó una segunda carga `peak`.

Durante esta prueba la cola llegó nuevamente a registrar trabajos activos.

Justo antes del retiro se observó:

```text
2026/10/29 03:48:03
demanda=3
pendientes=0
activos=3
reintentos=0
```

Con trabajo todavía activo se solicitó el retiro de:

```text
mooc-worker-zp1h
```

Las operaciones registradas fueron:

```text
2026-10-28T20:44:50-07:00  compute.instanceGroupManagers.deleteInstances  mooc-worker-mig  camilafajardo32@gmail.com
2026-10-28T20:44:52-07:00  delete  mooc-worker-zp1h  167186676477@cloudservices.gserviceaccount.com
2026-10-28T20:45:16-07:00  insert  mooc-worker-pdxt  167186676477@cloudservices.gserviceaccount.com
```

Durante el retiro, `mooc-worker-zp1h` permaneció temporalmente en estado `DELETING` mientras las otras instancias seguían saludables.

El MIG incorporó automáticamente `mooc-worker-pdxt`, que posteriormente quedó `HEALTHY / NONE`.

La cola llegó después a:

```text
demanda=0
pendientes=0
activos=0
reintentos=0
```

Este comportamiento demuestra que el retiro no interrumpió el servicio de procesamiento y que la capacidad necesaria se conservó mientras finalizaban los trabajos.

---

## 7.6 Reconciliación final

La reconciliación se realizó a nivel de cola y estado final del procesamiento.

| Métrica | Resultado observado |
|---|---:|
| Pendientes finales | 0 |
| Activos finales | 0 |
| Reintentos finales | 0 |
| Instancias con error al finalizar | 0 |
| Cola drenada | Sí |
| Procesamiento continuó durante el retiro | Sí |
| Capacidad de reemplazo creada automáticamente | Sí |
| Estado final del MIG | Regreso a capacidad mínima |

No se observaron trabajos residuales después del drenaje de la cola ni fallos que dejaran recursos permanentemente en procesamiento.

Tampoco se observaron evidencias de trabajos duplicados durante la ejecución.

Los logs conservados para esta prueba se encuentran en:

```text
docs/entrega3/evidencias-escenario3b/
```

e incluyen estados del MIG, operaciones de inserción/eliminación y snapshots de la cola antes y después del retiro.

---

## 7.7 Resultado del Scenario 3B

El Scenario 3B fue satisfactorio.

Se comprobó el ciclo completo:

```text
aumento de carga
→ crecimiento de demanda en la cola
→ creación automática de nuevos workers
→ nuevo worker HEALTHY y procesando
→ drenaje de la cola
→ retiro controlado con trabajo activo
→ mantenimiento de capacidad mediante reemplazo
→ pendientes=0 y activos=0
→ scale-down automático
→ retorno a la capacidad mínima
```

La prueba demuestra que el procesamiento multimedia no depende del ciclo de vida de una única VM.

La cola compartida permanece en la VM de servicios y el MIG puede crear o retirar workers sin perder la continuidad del procesamiento.

---

# 8. Tabla comparativa general E2 vs. E3

| Aspecto | Entrega 2 | Entrega 3 | Resultado |
|---|---|---|---|
| API | VM fija | MIG 1–3 | Escenario 3A: ciclo automático 1 → 3 → 1 con 55–58 s hasta atender y reducción de 1 VM cada 10 min |
| Workers | 1 worker fijo | MIG 1–3 | Escalamiento horizontal validado |
| Concurrencia worker | 2 | 2 por VM | Se mantiene la comparabilidad |
| Redis | Asociado al worker | VM de servicios | Cola desacoplada del ciclo de vida del worker |
| Cola | Compartida localmente con worker | Compartida externa al MIG | Permite reemplazo y retiro de workers sin perder la cola |
| Escalamiento | No | Automático | Validado en Scenario 3B |
| Balanceo API | No elástico | HTTPS Load Balancer | Reparto entre réplicas por `X-Instance-Id`; exclusión de una réplica no sana en ~8–18 s sin errores en el catálogo |
| Escenario 1 (100 VUs) | p95 1 049 ms, p99 30 s, 1,76 % errores | p95 147 ms, p99 403 ms, 0 % errores | +48 % de peticiones y +53 % de journeys; desaparece la saturación de la E2 |
| Procesamiento multimedia | Capacidad fija | Capacidad horizontal 1–3 workers | Mejora la capacidad de respuesta ante acumulación de trabajos |
| Retiro de worker | No evaluado como grupo elástico | Retiro controlado con trabajo activo | Validado |
| Estado final de cola | Dependiente del worker fijo | `pendientes=0`, `activos=0`, `reintentos=0` | Drenaje completo observado |

## 8.1 Conclusiones 
Las pruebas de la Entrega 3 muestran que la principal mejora de arquitectura para el flujo multimedia no consiste en aumentar la concurrencia dentro de una única VM, sino en mantener `WORKER_CONCURRENCY=2` y permitir que el MIG aumente horizontalmente el número de workers.

Scenario 2 mostró que el sistema mantiene el flujo funcional desde la autorización del upload hasta el consumo HLS bajo niveles crecientes de carga.

La degradación más visible se presentó en el tiempo asíncrono `upload → ready`, mientras que la tasa de errores HTTP permaneció en 0 % en baseline, L2 y L3 y en 0.18 % para L1.

Scenario 3B confirmó que el autoscaler responde a la demanda real de la cola, incorpora nuevas instancias de forma autónoma, mantiene el procesamiento durante el retiro de un worker y posteriormente reduce la capacidad cuando la demanda vuelve a cero.

En la API, el Escenario 1 mostró que la nueva arquitectura elimina el punto de saturación observado en la Entrega 2 a 100 VUs (p95 de 1 049 ms a 147 ms, errores de 1,76 % a 0 %), y el Escenario 3A confirmó el ciclo completo de autoscaling por CPU (1 → 3 → 1) y la exclusión automática, por el balanceador, de una réplica que deja de estar lista.

Estos resultados son coherentes con la decisión de arquitectura de separar Redis y la cola del MIG de workers: la cola permanece disponible aunque una VM de procesamiento sea creada, reemplazada o retirada.

## 8.2 Pendientes

- Métricas C4 adicionales de E2 para el Escenario 2, si el equipo decide incluir percentiles que no están presentes en la evidencia actualmente disponible.

Las secciones de los Escenarios 1, 2, 3A y 3B quedan completas con las corridas y evidencias de la Entrega 3.
