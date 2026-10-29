# Pruebas de carga — Entrega 3

## 1. Objetivo

El objetivo de las pruebas de carga de la Entrega 3 es evaluar el comportamiento de la aplicación MOOC sobre la nueva arquitectura elástica desplegada en Google Cloud Platform.

A diferencia de la Entrega 2, donde la capacidad se evaluó sobre una configuración fija, en esta entrega la API y los workers se ejecutan en Managed Instance Groups (MIG) independientes con escalamiento automático entre 1 y 3 instancias.

Las pruebas buscan determinar:

- el comportamiento del sistema ante niveles crecientes de carga;
- la capacidad del autoscaling para responder ante aumentos y disminuciones de demanda;
- el efecto de la nueva arquitectura sobre los resultados obtenidos en la Entrega 2;
- el comportamiento de la cola multimedia durante el escalamiento;
- la integridad de los trabajos durante el retiro de workers;
- los principales cuellos de botella observados.

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

- k6 v2.3.0;
- los mismos datos sintéticos;
- la misma mezcla de operaciones;
- las mismas pausas;
- las mismas duraciones;
- el mismo calentamiento;
- los mismos perfiles multimedia;
- concurrencia 2 por worker sobre `e2-medium`.

El generador se ejecuta fuera de las máquinas virtuales de la aplicación.

Todas las solicitudes HTTP dirigidas a la API deben ingresar a través del balanceador HTTPS. Las transferencias de archivos multimedia se realizan directamente contra Cloud Storage mediante las URL autorizadas por la aplicación.

Después del login, k6 utiliza el token Bearer correspondiente y descarta las cookies para evitar respuestas CSRF 403.

Las corridas comparadas deben repetirse. Los percentiles se reportan individualmente por corrida y no se promedian entre ejecuciones.

### 3.1 Métricas

Para cada corrida se registran, cuando corresponda:

- p50;
- p95;
- p99;
- throughput;
- porcentaje de errores;
- timeouts;
- CPU;
- memoria;
- red;
- disco;
- conexiones a Cloud SQL;
- profundidad de cola;
- antigüedad del trabajo más antiguo;
- trabajos activos;
- trabajos pendientes;
- reintentos;
- número de instancias del MIG;
- tiempo de escalamiento;
- tiempo de procesamiento multimedia.

Para el procesamiento multimedia también se consideran:

- tiempo de autorización de carga;
- tiempo de transferencia;
- tiempo de confirmación;
- tiempo `upload → ready`;
- errores de generación HLS;
- errores de manifest;
- errores de segmentos HLS.

---

# 4. Escenario 1 — Actividad académica

**Responsable:** Samara  
**Comparación:** Escenario 1 de la Entrega 2.

Este escenario reproduce la carga académica utilizada en la Entrega 2, pero dirige las solicitudes al balanceador HTTPS que distribuye las peticiones entre las réplicas del MIG de API.

## 4.1 Parámetros

> Pendiente de completar con los valores entregados por Samara.

## 4.2 Resultados

| Nivel | Instancias API | p50 | p95 | p99 | Throughput | Errores |
|---|---:|---:|---:|---:|---:|---:|
| Baseline | TBD | TBD | TBD | TBD | TBD | TBD |
| Nivel 1 | TBD | TBD | TBD | TBD | TBD | TBD |
| Nivel 2 | TBD | TBD | TBD | TBD | TBD | TBD |
| Nivel 3 | TBD | TBD | TBD | TBD | TBD | TBD |

## 4.3 Análisis

> Pendiente de resultados de Samara.

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

### Baseline

| Métrica | Resultado |
|---|---:|
| Uploaders | 1 |
| Viewers | 2 |
| Instancias worker iniciales | 1 |
| Instancias worker máximas | TBD |
| p50 | TBD |
| p95 | TBD |
| p99 | TBD |
| Throughput | TBD |
| Errores HTTP | TBD |
| `upload → ready` p95 | TBD |
| Reintentos | TBD |
| Timeouts | TBD |

### Nivel L1

| Métrica | Resultado |
|---|---:|
| Uploaders | 2 |
| Viewers | 5 |
| Instancias worker iniciales | TBD |
| Instancias worker máximas | TBD |
| p50 | TBD |
| p95 | TBD |
| p99 | TBD |
| Throughput | TBD |
| Errores HTTP | TBD |
| `upload → ready` p95 | TBD |
| Reintentos | TBD |

### Nivel L2

| Métrica | Resultado |
|---|---:|
| Uploaders | 4 |
| Viewers | 10 |
| Instancias worker máximas | TBD |
| p50 | TBD |
| p95 | TBD |
| p99 | TBD |
| Throughput | TBD |
| Errores HTTP | TBD |
| `upload → ready` p95 | TBD |
| Reintentos | TBD |

### Nivel L3

| Métrica | Resultado |
|---|---:|
| Uploaders | 6 |
| Viewers | 20 |
| Instancias worker máximas | TBD |
| p50 | TBD |
| p95 | TBD |
| p99 | TBD |
| Throughput | TBD |
| Errores HTTP | TBD |
| `upload → ready` p95 | TBD |
| Reintentos | TBD |

---

## 5.7 Comparación E2 vs. E3

| Métrica | E2 | E3 | Observación |
|---|---:|---:|---|
| Worker inicial | 1 × e2-medium | 1 × e2-medium | Misma capacidad inicial |
| Concurrencia/worker | 2 | 2 | Se mantiene para comparabilidad |
| Workers máximos | 1 | 3 | E3 permite escalamiento horizontal |
| Redis | Worker | VM de servicios | E3 desacopla la cola del worker |
| Errores HTTP | 0 % en corridas C4 válidas | TBD | TBD |
| p95 HTTP | TBD | TBD | TBD |
| p99 HTTP | TBD | TBD | TBD |
| `upload → ready` | TBD | TBD | TBD |
| Cola máxima | TBD | TBD | TBD |
| Reintentos | TBD | TBD | TBD |

### Interpretación

> Completar después de las corridas.

El análisis debe indicar si el escalamiento horizontal permitió absorber una cola mayor o reducir el tiempo de espera en comparación con E2. También debe diferenciar claramente entre la latencia HTTP de la API y el tiempo asíncrono de procesamiento multimedia.

---

# 6. Escenario 3A — Autoscaling de API

**Responsable:** Samara  
**Acompañamiento:** Daniela

Este escenario no tiene equivalente directo en E2.

Debe documentar el ciclo:

```text
carga
→ aumento de CPU
→ decisión del autoscaler
→ creación de nueva instancia
→ health check
→ incorporación al balanceador
→ reducción de carga
→ retiro de instancia
```

## 6.1 Resultados

> Pendiente de resultados de Samara.

---

# 7. Escenario 3B — Autoscaling de workers y retiro seguro

**Responsable:** Camila  
**Acompañamiento:** Mateo

## 7.1 Objetivo

Este escenario evalúa específicamente el comportamiento dinámico del MIG de workers.

Debe demostrar:

1. aumento de trabajos en la cola;
2. aumento automático del número de workers;
3. procesamiento autónomo por una nueva instancia;
4. drenaje de la cola;
5. reducción automática de instancias;
6. retiro de un worker mientras existe trabajo activo;
7. finalización o reencolado seguro del trabajo;
8. ausencia de pérdidas o duplicados.

---

## 7.2 Estado inicial

El estado inicial observado antes de la carga fue:

```text
T0
Workers: 1
Estado: HEALTHY
Autoscaler: ACTIVE
demanda: 0
pendientes: 0
activos: 0
reintentos: 0
WORKER_CONCURRENCY: 2
```

El worker fue capaz de arrancar autónomamente y comenzar a consumir la cola `media`.

Tiempo de arranque medido:

```text
51 s
```

---

## 7.3 Observabilidad

Durante todo el escenario se utilizan dos monitores independientes.

### MIG de workers

Se consulta periódicamente:

```powershell
gcloud compute instance-groups managed list-instances mooc-worker-mig `
  --zone=us-central1-a `
  --format="table(name.basename(),status,instanceHealth[0].detailedHealthState,currentAction)"
```

### Cola multimedia

La VM de servicios expone cada 30 segundos:

```text
demanda
pendientes
activos
reintentos
antiguedad_s
```

mediante el contenedor `cola-exportador`.

---

## 7.4 Línea de tiempo del autoscaling

Completar durante la corrida real:

| Tiempo | Demanda | Pendientes | Activos | Workers | Evento |
|---|---:|---:|---:|---:|---|
| T0 | 0 | 0 | 0 | 1 | Estado inicial |
| T1 | TBD | TBD | TBD | 1 | Inicio de carga |
| T2 | TBD | TBD | TBD | 1 | Autoscaler detecta demanda |
| T3 | TBD | TBD | TBD | 2 | Segunda VM creada |
| T4 | TBD | TBD | TBD | 2 | Nuevo worker HEALTHY |
| T5 | TBD | TBD | TBD | 3 | Tercera VM creada, si aplica |
| T6 | TBD | TBD | TBD | TBD | Cola comienza a drenar |
| T7 | 0 | 0 | TBD | TBD | Sin trabajos pendientes |
| T8 | 0 | 0 | 0 | TBD | Procesamiento terminado |
| T9 | 0 | 0 | 0 | 2 | Primer scale-down |
| T10 | 0 | 0 | 0 | 1 | Estado estable final |

El comportamiento esperado según la política configurada es aproximadamente:

```text
scale-up: del orden de 1.5–2 min bajo demanda suficiente
scale-down: espera sin demanda + máximo 1 VM retirada cada 10 min
```

Estos tiempos se deben contrastar con los valores realmente observados.

---

## 7.5 Retiro controlado con trabajo activo

La segunda parte de 3B consiste en observar el retiro de una instancia mientras existe trabajo en ejecución.

Antes del retiro se registra:

```text
worker objetivo
media_asset/job asociado
estado del trabajo
trabajos activos
trabajos pendientes
```

Durante el retiro se verifica que el worker reciba apagado ordenado.

La configuración permite hasta:

```text
ShutdownTimeout = 300 s
```

Si el trabajo no alcanza a terminar durante el período de apagado, Asynq debe devolverlo a la cola para que otra réplica pueda procesarlo.

---

## 7.6 Reconciliación final

Al finalizar el escenario debe cumplirse:

```text
trabajos aceptados
=
trabajos procesados correctamente
+
trabajos fallidos con estado diagnosticable
```

y además:

```text
trabajos perdidos = 0
derivados duplicados = 0
trabajos activos residuales = 0
trabajos pendientes residuales = 0
```

Tabla de reconciliación:

| Métrica | Resultado |
|---|---:|
| Trabajos creados | TBD |
| Trabajos `ready` | TBD |
| Trabajos `failed` | TBD |
| Pendientes finales | TBD |
| Activos finales | TBD |
| Reintentos | TBD |
| Trabajos perdidos | TBD |
| Derivados duplicados | TBD |

---

## 7.7 Resultado del Scenario 3B

> Pendiente de corrida real.

El escenario se considera satisfactorio únicamente si se observa una nueva instancia del MIG consumiendo trabajos de manera autónoma y posteriormente el sistema reduce nuevamente su capacidad sin perder ni duplicar trabajos.

---

# 8. Tabla comparativa general E2 vs. E3

| Aspecto | Entrega 2 | Entrega 3 | Resultado |
|---|---|---|---|
| API | VM fija | MIG 1–3 | TBD |
| Workers | 1 worker fijo | MIG 1–3 | TBD |
| Concurrencia worker | 2 | 2 por VM | Igual |
| Redis | Asociado al worker | VM de servicios | Desacoplado |
| Cola | Compartida localmente con worker | Compartida externa al MIG | Mejor tolerancia al retiro |
| Escalamiento | No | Automático | TBD |
| Balanceo API | No elástico | HTTPS Load Balancer | TBD |
| Procesamiento multimedia | Capacidad fija | Escala horizontalmente