# EstimaciÃ³n de costos

## Contexto

Proveedor: Google Cloud Platform
Proyecto: proyecto1-entrega2-desarrollo
RegiÃ³n: us-central1
Fecha de estimaciÃ³n: 2026-09-27 (actualizado el mismo dÃ­a con evidencia real de despliegue)

## Recursos considerados

| Recurso | Servicio GCP | ConfiguraciÃ³n efectiva | Estado |
|---|---|---|---|
| Web Server | Compute Engine `mooc-e2-web` | `e2-small`, zona `us-central1-a`, IP interna `10.20.0.3`, IP externa estatica `35.254.78.215`, conectado a `mooc-e2-subnet` | Implementado |
| Worker Server | Compute Engine `mooc-e2-worker` | `e2-small`, zona `us-central1-a`, IP interna `10.20.0.2`, IP externa `34.28.33.182`, conectado a `mooc-e2-subnet` | Implementado |
| Base de datos | Cloud SQL for PostgreSQL | PostgreSQL 16, instancia `mooc-postgres`, `db-g1-small`, 1 vCPU, 1.7 GB RAM, 10 GB SSD, zona Ãºnica, sin HA, IP pÃºblica `34.42.6.180` | Implementado |
| Multimedia | Cloud Storage | Bucket `mooc-e2-media-proyecto1-entrega2`, regiÃ³n `us-central1`, clase Standard | Implementado |
| IP externa | External IP | `35.254.78.215` (Web Server, estÃ¡tica) | Implementado |
| Red | VPC | VPC personalizada `mooc-e2-vpc` con subred `mooc-e2-subnet`; reglas de firewall por etiquetas para web, Redis e IAP | Implementado |
| Transferencia | Network Egress | Pendiente de medir durante las corridas del Escenario 1 | Pendiente |

## Red y seguridad implementada

- Se usa la VPC personalizada `mooc-e2-vpc` y la subred regional `mooc-e2-subnet`.
- Las reglas de firewall de la aplicacion limitan la exposicion: `mooc-e2-allow-web` publica solo 80/443 hacia el Web Server, `mooc-e2-allow-redis-from-web` permite Redis solo desde `web-server` hacia `worker-server`, y `mooc-e2-allow-iap-ssh` limita SSH al rango de IAP.
- Redis y ClamAV no tienen reglas de entrada publicas.
- El Worker Server conserva IP externa (`34.28.33.182`) como exposicion operacional controlada porque la conexion actual a Cloud SQL usa IP publica; no existen reglas de entrada publicas hacia Redis, ClamAV ni puertos internos de la aplicacion.

## Evidencia del despliegue

- Health check pÃºblico: `https://35.254.78.215.sslip.io/health` responde
  `200 OK` con `{"status":"ok"}` (verificado 2026-09-27).
- La API corre en Docker Compose nativo sobre `mooc-e2-web`
  (`/opt/mooc/deploy/docker-compose.web.yml`), con Caddy como servicio
  systemd (no dockerizado) haciendo de proxy HTTPS con certificado de
  Let's Encrypt para el dominio `35.254.78.215.sslip.io`.

## Cloud SQL

La base de datos administrada utiliza Cloud SQL for PostgreSQL 16 en
`us-central1`, instancia `mooc-postgres`.

ConfiguraciÃ³n efectiva:

- Tipo de mÃ¡quina: `db-g1-small`.
- 1 vCPU.
- 1.7 GB de memoria.
- 10 GB de almacenamiento SSD.
- Instancia en una sola zona.
- Alta disponibilidad deshabilitada.
- Backups automÃ¡ticos habilitados.
- ConexiÃ³n SSL requerida.
- IP pÃºblica `34.42.6.180`, con red autorizada restringida a las IPs que
  necesitan conectarse (equipo de desarrollo y Web Server).
- La conexion privada queda condicionada a completar Private Service Access
  para Cloud SQL en la VPC personalizada.

Las migraciones del proyecto fueron aplicadas correctamente sobre Cloud SQL y
el esquema fue verificado.

TambiÃ©n se realizÃ³ una prueba de respaldo y recuperaciÃ³n:

1. Se generÃ³ un backup mediante `pg_dump`.
2. Se validÃ³ el archivo con `pg_restore --list`.
3. Se creÃ³ una base temporal.
4. Se restaurÃ³ el backup sobre la base temporal.
5. Se verificaron las tablas restauradas.
6. La base temporal fue eliminada despuÃ©s de la prueba.

## Control de conexiones

La API y el worker permiten configurar el pool de PostgreSQL mediante:

- `DB_MAX_CONNS`
- `DB_MIN_CONNS`

La configuraciÃ³n utilizada como punto de partida es:

```text
DB_MAX_CONNS=5
DB_MIN_CONNS=0
```

La instancia de Cloud SQL reporta:

```text
max_connections = 50
```
