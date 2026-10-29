# Cloud SQL y pool de conexiones — Entrega 3

## Configuración observada

La API y el worker utilizan `pgxpool` para administrar las conexiones a PostgreSQL.

La configuración por defecto observada en el código es:

```text
DB_MAX_CONNS=5
DB_MIN_CONNS=0
```

Esta configuración se encuentra tanto en:

```text
cmd/api/main.go
cmd/worker/main.go
```

Por lo tanto, cada instancia de API y cada instancia de worker puede abrir como máximo 5 conexiones simultáneas a PostgreSQL.

## Validación del entorno E3

La instancia de Cloud SQL utilizada en la Entrega 3 fue verificada con la siguiente configuración:

```text
Motor: PostgreSQL 16
Tier: db-g1-small
Estado: RUNNABLE
max_connections: 50
```

Además, la API desplegada en `mooc-api-mig` reportó correctamente:

```json
{
  "status": "ready",
  "checks": {
    "postgres": "ok",
    "redis": "ok"
  }
}
```

Esto confirma que la réplica de API tiene conectividad correcta tanto con Cloud SQL como con Redis en el entorno de la Entrega 3.

## Escalamiento horizontal en Entrega 3

La arquitectura de la Entrega 3 permite escalar de forma independiente:

```text
API MIG:     1–3 instancias
Worker MIG:  1–3 instancias
```

Cada VM del grupo de workers utiliza:

```text
WORKER_CONCURRENCY=2
```

Esta concurrencia controla la cantidad de trabajos procesados simultáneamente por cada worker y es independiente del tamaño máximo del pool PostgreSQL (`DB_MAX_CONNS=5`).

Cada réplica ejecuta su propio pool de conexiones.

El número máximo teórico de conexiones puede calcularse como:

```text
Conexiones máximas =
(API replicas × API DB_MAX_CONNS)
+
(Worker replicas × Worker DB_MAX_CONNS)
```

Con la configuración actual:

```text
DB_MAX_CONNS = 5
```

el escenario máximo sería:

```text
3 APIs × 5 conexiones = 15
3 workers × 5 conexiones = 15
--------------------------------
Total máximo = 30 conexiones
```

## Relación con Cloud SQL

Durante la validación de la Entrega 3 se consultó directamente la instancia `mooc-postgres`, obteniendo:

```text
max_connections = 50
```

Por lo tanto, con la configuración actual:

```text
3 APIs × 5 conexiones = 15
3 workers × 5 conexiones = 15
Total máximo = 30 conexiones
```

la aplicación podría consumir teóricamente hasta:

```text
30 / 50 = 60 %
```

del límite global de conexiones de PostgreSQL.

El margen restante sería:

```text
50 - 30 = 20 conexiones
```

equivalente aproximadamente al 40 % del límite total.

Este margen permite reservar conexiones para:

- migraciones;
- administración;
- diagnóstico;
- mantenimiento;
- conexiones temporales;
- reconexiones;
- otros procesos auxiliares.

## Análisis de configuraciones alternativas

### DB_MAX_CONNS = 10

Con 3 APIs y 3 workers:

```text
6 procesos × 10 conexiones = 60 conexiones
```

Esto superaría el límite observado de:

```text
max_connections = 50
```

Por lo tanto, esta configuración no sería segura si ambos grupos escalan hasta 3 réplicas.

### DB_MAX_CONNS = 6

```text
6 procesos × 6 conexiones = 36 conexiones
```

Quedarían:

```text
50 - 36 = 14 conexiones
```

de margen.

Aunque es posible, reduce el margen disponible sin evidencia actual de que 5 conexiones por proceso sean insuficientes.

### DB_MAX_CONNS = 5

```text
6 procesos × 5 conexiones = 30 conexiones
```

Quedan:

```text
50 - 30 = 20 conexiones
```

libres.

Esta configuración conserva un margen mayor y se considera una opción adecuada como punto de partida para la Entrega 3.

## DB_MIN_CONNS

La configuración observada es:

```text
DB_MIN_CONNS=0
```

Esto permite que las réplicas no mantengan conexiones mínimas abiertas cuando no existe carga.

Si se utilizara:

```text
DB_MIN_CONNS=5
```

y existieran 6 procesos activos, podrían mantenerse hasta 30 conexiones abiertas incluso cuando el sistema se encuentra inactivo.

Por esta razón, mantener:

```text
DB_MIN_CONNS=0
```

es apropiado para una arquitectura con autoscaling.

## Relación entre autoscaling y pool

El escalamiento horizontal incrementa no solo la capacidad de procesamiento, sino también el número potencial de conexiones hacia PostgreSQL.

Por ejemplo:

```text
1 API = hasta 5 conexiones
2 APIs = hasta 10 conexiones
3 APIs = hasta 15 conexiones
```

Lo mismo ocurre con los workers.

Por lo tanto, cualquier modificación de `DB_MAX_CONNS` debe analizarse junto con el número máximo de réplicas permitido por los MIG.

Aumentar el tamaño del pool sin considerar el escalamiento horizontal podría provocar que varias réplicas consuman simultáneamente una cantidad excesiva de conexiones hacia Cloud SQL.

## Métricas a observar durante las pruebas

Durante los escenarios de carga se deben correlacionar:

- número de conexiones PostgreSQL;
- CPU de Cloud SQL;
- latencia de solicitudes;
- p95 y p99;
- errores de conexión;
- número de réplicas de API;
- número de réplicas de workers;
- profundidad de cola;
- throughput.

Si la latencia aumenta pero las conexiones y CPU de Cloud SQL permanecen bajas, no debe concluirse automáticamente que Cloud SQL es el cuello de botella.

Si las conexiones se acercan de forma sostenida al límite global o aparecen esperas para adquirir conexiones del pool, entonces debe evaluarse un ajuste del tamaño del pool o de la capacidad de la base de datos.

## Criterio de decisión

Se debería aumentar `DB_MAX_CONNS` únicamente si se observa:

1. espera significativa para adquirir conexiones;
2. Cloud SQL con capacidad disponible;
3. número de conexiones suficientemente lejos de `max_connections`;
4. mejora medible después de incrementar el pool.

El número de usuarios concurrentes por sí solo no justifica aumentar el tamaño del pool.

## Conclusión

La configuración actual de `DB_MAX_CONNS=5` y `DB_MIN_CONNS=0` es adecuada como configuración base para la Entrega 3.

Con el escalamiento máximo previsto de 3 APIs y 3 workers, el consumo máximo teórico sería de 30 conexiones, equivalente al 60 % de las 50 conexiones disponibles en Cloud SQL. Esto conserva un margen de aproximadamente 40 % para tareas administrativas, migraciones, mantenimiento y conexiones adicionales.

No se recomienda incrementar `DB_MAX_CONNS` sin evidencia obtenida durante las pruebas de carga. En particular, una configuración de 10 conexiones por proceso permitiría alcanzar hasta 60 conexiones, superando el límite actual de Cloud SQL.

La decisión de modificar el pool debe basarse en métricas de espera de conexiones, utilización de Cloud SQL, latencia, throughput y número de réplicas activas, y no únicamente en el número de usuarios concurrentes.