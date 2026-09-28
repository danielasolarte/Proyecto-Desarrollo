# Estimación de costos

## Contexto

Proveedor: Google Cloud Platform  
Proyecto: proyecto1-entrega2-desarrollo  
Región: us-central1  
Fecha de estimación: 2026-09-27

Esta estimación corresponde al despliegue de la Entrega 2. Los valores de
Compute Engine, Cloud Storage y transferencia se completarán cuando los
recursos finales del Web Server y Worker Server estén disponibles.

## Recursos considerados

| Recurso | Servicio GCP | Configuración actual | Estado |
|---|---|---|---|
| Web Server | Compute Engine | Pendiente de creación/configuración final | Bloqueado por infraestructura |
| Worker Server | Compute Engine | Pendiente de creación/configuración final | Bloqueado por infraestructura |
| Base de datos | Cloud SQL for PostgreSQL | PostgreSQL 16, db-g1-small, 1 vCPU, 1.7 GB RAM, 10 GB SSD, zona única, sin HA | Implementado |
| Disco Web | Persistent Disk | Pendiente | Bloqueado |
| Disco Worker | Persistent Disk | Pendiente | Bloqueado |
| Multimedia | Cloud Storage | Bucket pendiente de creación | Bloqueado |
| IP externa | External IP | Depende de la configuración final de las VMs | Bloqueado |
| Transferencia | Network Egress | Depende de las pruebas finales | Pendiente |

## Cloud SQL

La base de datos administrada utiliza Cloud SQL for PostgreSQL 16 en
`us-central1`.

Configuración efectiva:

- Tipo de máquina: `db-g1-small`.
- 1 vCPU.
- 1.7 GB de memoria.
- 10 GB de almacenamiento SSD.
- Instancia en una sola zona.
- Alta disponibilidad deshabilitada.
- Backups automáticos habilitados.
- Conexión SSL requerida.
- Durante desarrollo se utilizó IP pública con una red autorizada `/32`.
- La conexión privada queda condicionada a la infraestructura de red final.

Las migraciones del proyecto fueron aplicadas correctamente sobre Cloud SQL y
el esquema fue verificado.

También se realizó una prueba de respaldo y recuperación:

1. Se generó un backup mediante `pg_dump`.
2. Se validó el archivo con `pg_restore --list`.
3. Se creó una base temporal.
4. Se restauró el backup sobre la base temporal.
5. Se verificaron las tablas restauradas.
6. La base temporal fue eliminada después de la prueba.

## Control de conexiones

La API y el worker permiten configurar el pool de PostgreSQL mediante:

- `DB_MAX_CONNS`
- `DB_MIN_CONNS`

La configuración utilizada como punto de partida es:

```text
DB_MAX_CONNS=5
DB_MIN_CONNS=0

La instancia de Cloud SQL reporta:

```text
max_connections = 50
```

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