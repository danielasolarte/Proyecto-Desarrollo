# Operación y recuperación

## Objetivo

Este documento describe los procedimientos utilizados para operar, respaldar y
recuperar la base de datos PostgreSQL administrada en Cloud SQL, así como para
desplegar, reiniciar y reconstruir los servicios en las VMs del Web Server y
del Worker Server.

La configuración documentada corresponde al entorno de la Entrega 2.

## Cloud SQL

Proveedor: Google Cloud Platform
Servicio: Cloud SQL for PostgreSQL
Región: us-central1
Versión mayor: PostgreSQL 16
Tipo de máquina: db-g1-small
Alta disponibilidad: deshabilitada
SSL: requerido
Instancia: `mooc-postgres`
Redes autorizadas: la IP externa del Web Server (`mooc-e2-web`) debe estar en
la lista de redes autorizadas de la instancia (Cloud SQL > `mooc-postgres` >
Conexiones > Redes autorizadas); sin esto, las conexiones desde el Web Server
se cuelgan (no se rechazan, simplemente no responden) hasta agotar el
timeout del cliente.

## Migraciones

Las migraciones se encuentran en:

```text
migrations/
```

y se aplican con `golang-migrate`. Si no está instalado:

```bash
go install -tags "postgres" github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Para aplicar todas las migraciones pendientes contra la base de la Entrega 2
(ajustar usuario, clave, host e IP a los reales de `mooc-postgres`):

```bash
migrate -database "postgres://mooc-postgres:<clave>@<IP_publica_cloud_sql>:5432/mooc?sslmode=require" -path migrations up
```

Para revertir la última migración aplicada, cambiar `up` por `down 1`.

## Secretos y variables de entorno

Ningún archivo `.env` real se sube al repositorio (ver `.gitignore`). Los
archivos versionados son solo plantillas:

- `deploy/web.env.example` -> se copia como `.env` en `mooc-e2-web`, en
  `/opt/mooc/deploy/.env`.
- `deploy/worker.env.example` -> se copia como `.env` en `mooc-e2-worker`, en
  `/opt/mooc/deploy/.env`.

Ambos contienen, entre otros: la cadena de conexión a Cloud SQL
(`DATABASE_URL`), la dirección de Redis (`REDIS_ADDR`), las credenciales HMAC
del bucket de Cloud Storage (`S3_ACCESS_KEY`/`S3_SECRET_KEY`) y los parámetros
del pool de conexiones (`DB_MAX_CONNS`/`DB_MIN_CONNS`). Estos valores reales
solo existen en el `.env` de cada VM (creado manualmente por SSH) y no deben
compartirse fuera del equipo ni commitearse.

## Despliegue y reinicio: Web Server (`mooc-e2-web`)

El primer despliegue está automatizado en `deploy/gcp/04-deploy-web.ps1`
(cargar antes `deploy/gcp/00-variables.example.ps1`):

```powershell
. deploy/gcp/00-variables.example.ps1
./deploy/gcp/04-deploy-web.ps1 -Domain "<ip-externa-con-guiones-o-puntos>.sslip.io" -AcmeEmail "<correo>"
```

El script copia el compose y la plantilla de `.env` a la VM, instala el
`Caddyfile` con el dominio indicado, autentica Docker contra Artifact
Registry con un token de la cuenta que ejecuta el script (evita depender de
los permisos de la cuenta de servicio de la VM, que causó
`Unauthenticated request` la primera vez que se probó a mano) y levanta los
servicios. Tras la primera corrida, `/opt/mooc/deploy/.env` en la VM queda
con los valores de ejemplo; hay que editarlo por SSH con los valores reales
y reiniciar (ver más abajo).

Para reconectarse manualmente por SSH: Consola de GCP > Compute Engine >
`mooc-e2-web` > SSH, o `gcloud compute ssh mooc-e2-web --zone us-central1-a`.

Caddy corre como servicio systemd nativo (no en Docker), configurado en
`/etc/caddy/Caddyfile` con el dominio `sslip.io` correspondiente a la IP
externa de la VM (formato con puntos, ej. `35.254.78.215.sslip.io`, no con
guiones). Para recargarlo tras un cambio de configuración:

```bash
sudo systemctl reload caddy
```

Reinicio simple (sin reconstruir):

```bash
cd /opt/mooc/deploy && sudo docker compose restart
```

Reconstrucción completa (por ejemplo tras publicar una imagen nueva):

```bash
cd /opt/mooc/deploy
sudo docker compose pull
sudo docker compose up -d --force-recreate
```

Verificación de salud:

```bash
curl -s http://127.0.0.1:8080/health   # API directamente, dentro de la VM
curl -s https://<dominio-sslip>/health # a través de Caddy, desde afuera
```

## Despliegue y reinicio: Worker Server (`mooc-e2-worker`)

El despliegue de esta VM está automatizado en
`deploy/gcp/05-deploy-worker.ps1` (cargar antes
`deploy/gcp/00-variables.example.ps1`). El script copia
`deploy/docker-compose.worker.yml` y `deploy/worker.env.example` a la VM, los
mueve a `/opt/mooc/deploy`, autentica Docker contra Artifact Registry con un
token de la cuenta que ejecuta el script (evita depender de los permisos de
la cuenta de servicio de la VM, que causó `Unauthenticated request` la
primera vez que se probó a mano) y levanta los tres servicios con
`docker compose up -d`:

```powershell
. deploy/gcp/00-variables.example.ps1
./deploy/gcp/05-deploy-worker.ps1
```

Tras la primera corrida, `/opt/mooc/deploy/.env` en la VM queda con los
valores de ejemplo (`deploy/worker.env.example`); hay que editarlo por SSH
con los valores reales (`DATABASE_URL`, `S3_*`, etc.) y reiniciar:

```bash
cd /opt/mooc/deploy
sudo docker compose -f docker-compose.worker.yml restart
```

Este compose levanta tres servicios: `worker` (proceso Go), `redis` (cola
asynq y sesiones) y `clamav` (escaneo de archivos subidos). Verificación:

```bash
sudo docker compose ps
sudo docker compose logs worker --tail=50
redis-cli -h 127.0.0.1 ping   # debe responder PONG
```

**Nota de red:** el diseño documentado en
`docs/entrega2/modelo-despliegue-red-web.md` especifica que el Worker Server no
debe tener IP pública (egreso solo por Cloud NAT). En el despliegue actual
`mooc-e2-worker` sí tiene IP externa; esto está registrado como desviación en
`docs/entrega2/costos-entrega2.md` y no debe usarse para exponer los puertos 6379
(Redis) ni ningún otro puerto hacia Internet.

## Respaldo de Cloud SQL

Cloud SQL realiza respaldos automáticos diarios por defecto para la instancia
`mooc-postgres` (configurable en Cloud SQL > `mooc-postgres` > Copias de
seguridad). Para un respaldo manual antes de un cambio riesgoso:

```bash
gcloud sql backups create --instance=mooc-postgres
```

Para restaurar, usar Cloud SQL > `mooc-postgres` > Copias de seguridad >
Restaurar, seleccionando el respaldo deseado.
