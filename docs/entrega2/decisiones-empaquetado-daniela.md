# Decisiones y adaptaciones: empaquetado y configuración

Responsable: Daniela Solarte. Cubre la parte de empaquetado dentro de
"Decisiones y adaptaciones"; la ubicación de Redis, la migración a Cloud
Storage, la carga directa con URLs firmadas y la configuración de los
workers están en la parte de Andrés Jurado.

## Imágenes de contenedor

Se construyen dos imágenes, `mooc-e2-api` y `mooc-e2-worker`, para
`linux/amd64` (arquitectura de las VMs `e2-small`), antes del despliegue en
vez de compilar en la propia VM, para no gastar su memoria durante el
arranque. Se publican en Artifact Registry:

```text
us-central1-docker.pkg.dev/proyecto1-entrega2-desarrollo/mooc-e2/mooc-e2-api:latest
us-central1-docker.pkg.dev/proyecto1-entrega2-desarrollo/mooc-e2/mooc-e2-worker:latest
```

Se eligió Artifact Registry sobre Docker Hub porque ya está en el mismo
proyecto de GCP donde viven las VMs, sin credenciales adicionales que
gestionar (la autenticación de `docker pull` en cada VM se resuelve con la
cuenta de servicio de Compute Engine).

**Adaptación frente a la Entrega 1:** las imágenes ya no se construyen en
el sitio con `docker compose up --build`; el nombre de imagen es una
variable de entorno (`API_IMAGE`, `WORKER_IMAGE`) para poder cambiar de
registro sin tocar los archivos de compose.

## Un compose por máquina virtual

`docker-compose.web.yml` (API + Mailpit) y `docker-compose.worker.yml`
(worker + Redis + ClamAV) reemplazan el `docker-compose.yml` único de la
Entrega 1 para el despliegue en la nube, porque la Entrega 2 exige dos
máquinas virtuales fijas con responsabilidades separadas. El
`docker-compose.yml` original se conserva para desarrollo local
(`docker compose up -d --build`), sin cambios de comportamiento.

La API se publica solo en `127.0.0.1:8080` dentro del Web Server: el
proxy HTTPS con Caddy es el único punto de entrada público, nunca
la API directamente.

## Contrato de variables de entorno

Un solo conjunto de nombres de variable para local y para las VMs
(`.env.example`, `deploy/web.env.example`, `deploy/worker.env.example`),
para que un cambio de infraestructura sea un cambio de valores, no de
código ni de los archivos de compose.

**Adaptación registrada durante la entrega:** al migrar de AWS a GCP por
indicación del profesor, `S3_ENDPOINT` quedó apuntando por error a
`s3.amazonaws.com` en un cambio intermedio; se corrigió a
`storage.googleapis.com` (la interfaz compatible con S3 de Cloud Storage),
que es lo que permite reusar `internal/media/platform/s3storage.go` sin
tocar el código del módulo de multimedia. El placeholder del host de base
de datos se renombró de `CAMBIAR_HOST_RDS` (nomenclatura de AWS, ya no
aplica) a `CAMBIAR_HOST_CLOUDSQL`.

Variables nuevas frente a la Entrega 1, y por qué:

- `WORKER_CONCURRENCY`: antes fija en 5 dentro del código; ahora
  configurable para poder fijarla y registrarla en cada corrida del
  Escenario 2 sin recompilar.
- `DB_MAX_CONNS` / `DB_MIN_CONNS`: el tamaño del pool de PostgreSQL debe
  quedar registrado frente al límite de conexiones de la instancia de
  Cloud SQL elegida (`db-g1-small`), no asumido.
- `S3_REGION`: declarada en los archivos de ejemplo para cuando el código
  de `s3storage.go` la use; hoy el cliente no la necesita para hablar con
  el endpoint compatible con S3 de Cloud Storage.
- `AUTH_COOKIE_ENABLED`, `CSRF_ENABLED` y las variables `SESSION_COOKIE_*`
  / `CSRF_*`: contrato para clientes web con cookies (detalle en
  `docs/entrega2/secretos-y-seguridad.md`); los clientes HTTP y los
  scripts de k6 siguen sin necesitarlas porque usan `Authorization: Bearer`.

Los archivos `*.env.example` nunca llevan secretos reales: `S3_ACCESS_KEY`,
`S3_SECRET_KEY` y el usuario/clave de `DATABASE_URL` quedan como
placeholders (`CAMBIAR_...`); el `.env` real de cada VM se crea a mano ahí
mismo, con permisos `600`, y nunca se commitea.

## Migraciones

Se evaluaron dos formas de aplicar `migrations/` contra Cloud SQL: correr
la imagen oficial de `migrate/migrate` en Docker (sin instalar nada en la
máquina que ejecuta el comando) o instalar el binario `migrate` y correrlo
directo. Se adoptó la segunda opción (`deploy/run-migrations.ps1`, con
verificación de código de salida y reporte de la versión aplicada) porque
es la que ya se usó para aplicar y verificar las migraciones reales contra
la instancia de Cloud SQL de este proyecto.

## Diferencias frente al empaquetado de la Entrega 1

- Las imágenes ya no se contruyen implícitamente al levantar el compose:
  se construyen y publican antes, como paso explícito.
- `.dockerignore` se agregó para builds más rápidos y para no filtrar
  archivos innecesarios dentro de la imagen.
- El release evaluado se identifica con el tag `entrega-2` sobre el
  commit final, no solo con la rama.
