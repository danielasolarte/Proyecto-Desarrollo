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
  infraestructura de `docs/metricas-infra-entrega2.md`):

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
