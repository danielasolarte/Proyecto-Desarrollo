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