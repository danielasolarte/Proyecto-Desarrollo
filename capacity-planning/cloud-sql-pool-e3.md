# Cloud SQL y pool de conexiones — Entrega 3

## Configuración observada

La API y el worker utilizan `pgxpool` para administrar las conexiones a PostgreSQL.

La configuración por defecto observada en el código es:

DB_MAX_CONNS=5
DB_MIN_CONNS=0


Esta configuración se encuentra tanto en:
cmd/api/main.go
cmd/worker/main.go

Por lo tanto, cada instancia de API y cada instancia de worker puede abrir como máximo 5 conexiones simultáneas a PostgreSQL.

## Escalamiento horizontal en Entrega 3
La arquitectura de la Entrega 3 permite escalar de forma independiente:
API MIG:     1–3 instancias
Worker MIG:  1–3 instancias

Cada réplica ejecuta su propio pool de conexiones.

El número máximo teórico de conexiones puede calcularse como:
Conexiones máximas =
(API replicas × API DB_MAX_CONNS)
+
(Worker replicas × Worker DB_MAX_CONNS)

Con la configuración actual:
DB_MAX_CONNS = 5

el escenario máximo sería:
3 APIs × 5 conexiones = 15
3 workers × 5 conexiones = 15

Total máximo = 30 conexiones

## Relación con Cloud SQL
En la Entrega 2 se observó:
max_connections = 50

Si este límite se mantiene en la instancia utilizada para la Entrega 3:
30 / 50 = 60 %

La aplicación podría consumir como máximo aproximadamente el 60 % del límite global de conexiones.

El margen restante sería:
50 - 30 = 20 conexiones

equivalente al 40 % del límite.

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
6 procesos × 10 conexiones = 60 conexiones
Esto superaría el límite observado de:
max_connections = 50
Por lo tanto, esta configuración no sería segura si ambos grupos escalan hasta 3 réplicas.

### DB_MAX_CONNS = 6
6 procesos × 6 conexiones = 36 conexiones
Quedarían:
50 - 36 = 14 conexiones de margen.
Aunque es posible, reduce el margen disponible sin evidencia actual de que 5 conexiones por proceso sean insuficientes.

### DB_MAX_CONNS = 5
6 procesos × 5 conexiones = 30 conexiones
Quedan 20 conexiones libres.
Esta configuración conserva un margen mayor y se considera una opción adecuada como punto de partida para la Entrega 3.

## DB_MIN_CONNS
La configuración observada es:
DB_MIN_CONNS=0

Esto permite que las réplicas no mantengan conexiones mínimas abiertas cuando no existe carga.

Si se utilizara:
DB_MIN_CONNS=5

y existieran 6 procesos activos, podrían mantenerse hasta 30 conexiones abiertas incluso cuando el sistema se encuentra inactivo.

Por esta razón, mantener:
DB_MIN_CONNS=0

es apropiado para una arquitectura con autoscaling.

## Relación entre autoscaling y pool
El escalamiento horizontal incrementa no solo la capacidad de procesamiento, sino también el número potencial de conexiones hacia PostgreSQL.

Ejemplo:
1 API = hasta 5 conexiones
2 APIs = hasta 10 conexiones
3 APIs = hasta 15 conexiones

Lo mismo ocurre con los workers.

Por lo tanto, cualquier modificación de DB_MAX_CONNS debe analizarse junto con el número máximo de réplicas permitido por los MIG.


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

## Criterio de decisión
Se debería aumentar DB_MAX_CONNS únicamente si se observa:
1. espera significativa para adquirir conexiones;
2. Cloud SQL con capacidad disponible;
3. número de conexiones lejos de max_connections;
4. mejora medible después de incrementar el pool.
El número de usuarios concurrentes por sí solo no justifica aumentar el tamaño del pool.