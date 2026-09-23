# Procedimiento de métricas de infraestructura

Este documento define el procedimiento común para recolectar y relacionar
las métricas de infraestructura con las pruebas de capacidad de la Entrega 2.

## Convención de tiempo

Todas las corridas y métricas se registran en UTC.

Para cada corrida se debe registrar:

- nombre de la corrida;
- escenario;
- nivel de carga;
- hora de inicio UTC;
- hora de fin UTC;
- configuración de infraestructura;
- concurrencia de workers;
- observaciones relevantes.

Formato recomendado:

`escenario1-100vu-2026-09-25T15-00Z`

## Web Server

Registrar:

- CPU utilization;
- memoria utilizada;
- memoria disponible;
- uso de disco;
- tráfico de red recibido;
- tráfico de red enviado.

CPU y red se obtienen de las métricas nativas de Compute Engine.
Memoria y disco se obtienen mediante Google Cloud Ops Agent.

## Worker Server

Registrar:

- CPU utilization;
- memoria utilizada;
- memoria disponible;
- uso de disco;
- tráfico de red recibido;
- tráfico de red enviado.

También se registra:

- WORKER_CONCURRENCY;
- cantidad de workers activos;
- uso de memoria durante procesamiento multimedia.

## Cloud SQL

Registrar:

- CPU utilization;
- número de conexiones activas;
- utilización de almacenamiento;
- operaciones de lectura;
- operaciones de escritura;
- bytes leídos;
- bytes escritos.

La configuración de la instancia de Cloud SQL debe permanecer fija durante
las corridas comparables.

## Redis / asynq

Registrar:

- profundidad de la cola;
- cantidad de trabajos activos;
- cantidad de trabajos pendientes;
- cantidad de trabajos completados;
- cantidad de reintentos;
- cantidad de trabajos fallidos;
- antigüedad aproximada del trabajo más antiguo;
- tasa de procesamiento.

## Procedimiento por corrida

1. Registrar la hora UTC de inicio.
2. Registrar el escenario y nivel de carga.
3. Confirmar que no haya otra prueba ejecutándose.
4. Confirmar la configuración de Web Server, Worker Server y Cloud SQL.
5. Confirmar WORKER_CONCURRENCY.
6. Ejecutar la prueba.
7. Registrar la hora UTC de finalización.
8. Exportar o capturar las métricas del mismo intervalo de tiempo.
9. Guardar las evidencias con el mismo identificador de la corrida.
10. Relacionar las métricas con los resultados de k6.

## Evidencias

Para cada corrida se debe conservar:

- resultado original de k6;
- captura o exportación de métricas del Web Server;
- captura o exportación de métricas del Worker Server;
- captura o exportación de métricas de Cloud SQL;
- métricas de Redis/asynq;
- configuración efectiva de infraestructura;
- interpretación de los resultados.

## Criterio de comparación

Las métricas de infraestructura se analizan junto con:

- p50;
- p95;
- p99;
- throughput;
- porcentaje de errores;
- timeouts.

El objetivo es identificar si la degradación observada desde k6 coincide con
saturación de CPU, memoria, conexiones de base de datos, I/O o crecimiento
de la cola.