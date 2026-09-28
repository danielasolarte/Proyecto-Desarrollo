# Estimación de costos

## Contexto

Proveedor: Google Cloud Platform
Proyecto: proyecto1-entrega2-desarrollo
Región: us-central1
Fecha de estimación: 2026-09-27 (actualizado el mismo día con evidencia real de despliegue)

## Recursos considerados

| Recurso | Servicio GCP | Configuración efectiva | Estado |
|---|---|---|---|
| Web Server | Compute Engine `mooc-e2-web` | `e2-small`, IP externa `35.254.78.215`, IP interna `10.20.0.3`, zona `us-central1-a`, red `mooc-e2-vpc`/subred `mooc-e2-subnet` | Implementado |
| Worker Server | Compute Engine `mooc-e2-worker` | `e2-small`, IP externa `34.28.33.182`, IP interna `10.20.0.2`, zona `us-central1-a`, red `mooc-e2-vpc`/subred `mooc-e2-subnet` | Implementado |
| Base de datos | Cloud SQL for PostgreSQL | PostgreSQL 16, instancia `mooc-postgres`, `db-g1-small`, 1 vCPU, 1.7 GB RAM, 10 GB SSD, zona única, sin HA, IP pública `34.42.6.180` | Implementado |
| Multimedia | Cloud Storage | Bucket `mooc-e2-media-proyecto1-entrega2`, región `us-central1`, clase Standard | Implementado |
| IP externa | External IP | `35.254.78.215` (Web Server, estática) | Implementado |
| Red | VPC | VPC personalizada `mooc-e2-vpc` / subred `mooc-e2-subnet` (`10.20.0.0/24`), tal como propone `modelo-despliegue-samara.md`. Ambas VMs (Web Server y Worker Server) quedaron en esta red desde el 27 de septiembre (antes estaban temporalmente en la red `default`, ver historial de este documento) | Implementado |
| Transferencia | Network Egress | Pendiente de medir durante las corridas del Escenario 1 | Pendiente |

## Desviaciones registradas frente al diseño de red original

- **Resuelta el 27 de septiembre:** inicialmente se había desplegado sobre
  la red `default` (modo automático) en vez de la VPC personalizada. El
  equipo migró ambas VMs a `mooc-e2-vpc`/`mooc-e2-subnet`
  (`10.20.0.0/24`) el mismo día de la entrega, alineando el despliegue con
  `modelo-despliegue-samara.md`. Al recrearse las VMs en la nueva red,
  cambiaron tanto las IPs internas (Web Server `10.20.0.3`, Worker Server
  `10.20.0.2`) como la IP externa del Worker Server (`34.28.33.182`); la
  IP externa estática del Web Server (`35.254.78.215`) no cambió. Esto
  obligó a actualizar `REDIS_ADDR` en el `.env` del Web Server y las redes
  autorizadas de Cloud SQL con las IPs nuevas.
- `mooc-e2-worker` sigue con IP externa (`34.28.33.182`), a diferencia de lo
  recomendado ("Worker Server sin IP publica, con salida mediante Cloud NAT").
  Redis y ClamAV siguen protegidos por las reglas de firewall por etiqueta,
  pero esto es una exposición mayor a la diseñada originalmente y queda
  anotado para la sustentación.

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
  necesitan conectarse (equipo de desarrollo, Web Server y Worker Server).
  Tras la migración a `mooc-e2-vpc` (ver arriba), las redes autorizadas se
  actualizaron con las IPs externas nuevas de ambas VMs.
- No se migró a conexión privada (Cloud SQL Auth Proxy / IP privada) por
  límite de tiempo; se mantiene IP pública con redes autorizadas y SSL
  requerido.

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
