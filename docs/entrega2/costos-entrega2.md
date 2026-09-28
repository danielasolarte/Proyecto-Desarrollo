# Estimación de costos

## Contexto

Proveedor: Google Cloud Platform
Proyecto: proyecto1-entrega2-desarrollo
Región: us-central1
Fecha de estimación: 2026-09-27 (actualizado el mismo día con evidencia real de despliegue)

## Recursos considerados

| Recurso | Servicio GCP | Configuración efectiva | Estado |
|---|---|---|---|
| Web Server | Compute Engine `mooc-e2-web` | `e2-small`, IP externa `35.254.78.215`, IP interna `10.128.0.3`, zona `us-central1-a` | Implementado |
| Worker Server | Compute Engine `mooc-e2-worker` | `e2-small`, IP interna `10.128.0.2`, zona `us-central1-a` | Implementado |
| Base de datos | Cloud SQL for PostgreSQL | PostgreSQL 16, instancia `mooc-postgres`, `db-g1-small`, 1 vCPU, 1.7 GB RAM, 10 GB SSD, zona única, sin HA, IP pública `34.42.6.180` | Implementado |
| Multimedia | Cloud Storage | Bucket `mooc-e2-media-proyecto1-entrega2`, región `us-central1`, clase Standard | Implementado |
| IP externa | External IP | `35.254.78.215` (Web Server, estática) | Implementado |
| Red | VPC | Red `default` (modo automático) en vez de una VPC personalizada; decisión del equipo por tiempo, documentada como desviación frente a `modelo-despliegue-samara.md` | Implementado (desviación registrada) |
| Transferencia | Network Egress | Pendiente de medir durante las corridas del Escenario 1 | Pendiente |

## Desviaciones registradas frente al diseño de red original

- Se usó la red `default` (modo automático) en vez de la VPC personalizada
  `mooc-e2-vpc` / subred `mooc-e2-subnet` (`10.20.0.0/24`) que propone
  `modelo-despliegue-samara.md`. Las reglas de firewall de la aplicación
  (`mooc-e2-allow-web`, `mooc-e2-allow-redis-from-web`, `mooc-e2-allow-iap-ssh`)
  sí se crearon sobre esta red y funcionan igual.
- `mooc-e2-worker` quedó con IP externa (`34.61.4.185`), a diferencia de lo
  recomendado ("Worker Server sin IP publica, con salida mediante Cloud NAT").
  Redis y ClamAV siguen protegidos por las reglas de firewall por etiqueta,
  pero esto es una exposición mayor a la diseñada originalmente y queda
  anotado para la sustentación.
- Las reglas automáticas de la red `default` (`default-allow-ssh`,
  `default-allow-rdp`, `default-allow-internal`, `default-allow-icmp`) se
  aplican a todas las instancias del proyecto; no se restringieron ni
  eliminaron por límite de tiempo.

## Evidencia del despliegue

- Health check público: `https://35.254.78.215.sslip.io/health` responde
  `200 OK` con `{"status":"ok"}` (verificado 2026-09-27).
- La API corre en Docker Compose nativo sobre `mooc-e2-web`
  (`/opt/mooc/deploy/docker-compose.web.yml`), con Caddy como servicio
  systemd (no dockerizado) haciendo de proxy HTTPS con certificado de
  Let's Encrypt para el dominio `35.254.78.215.sslip.io`.

## Cloud SQL

La base de datos administrada utiliza Cloud SQL for PostgreSQL 16 en
`us-central1`, instancia `mooc-postgres`.

Configuración efectiva:

- Tipo de máquina: `db-g1-small`.
- 1 vCPU.
- 1.7 GB de memoria.
- 10 GB de almacenamiento SSD.
- Instancia en una sola zona.
- Alta disponibilidad deshabilitada.
- Backups automáticos habilitados.
- Conexión SSL requerida.
- IP pública `34.42.6.180`, con red autorizada restringida a las IPs que
  necesitan conectarse (equipo de desarrollo y Web Server).
- La conexión privada queda condicionada a migrar a la VPC personalizada;
  no se hizo por la decisión de usar la red `default` (ver arriba).

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
```

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
