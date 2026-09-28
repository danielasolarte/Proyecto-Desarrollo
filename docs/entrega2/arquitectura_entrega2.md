# Arquitectura Entrega 2

Este documento consolida la arquitectura desplegada para la Entrega 2, incluyendo red, Web Server, Worker Server, servicios administrados y controles de seguridad.

## Componentes

- API modular en Go con Echo.
- Worker en Go con asynq.
- Redis para sesiones y cola.
- ClamAV para escaneo.
- Cloud SQL for PostgreSQL para persistencia transaccional.
- Cloud Storage como almacenamiento de objetos compatible con S3.
- Caddy como proxy inverso HTTPS en Web Server.

## Despliegue

El despliegue usa dos maquinas virtuales fijas:

- Web Server: API, Mailpit de laboratorio y proxy HTTPS.
- Worker Server: worker, Redis y ClamAV.

PostgreSQL se ejecuta en Cloud SQL y los objetos se almacenan en Cloud Storage.

Modelo de componentes (módulos, worker y comunicación síncrona/asíncrona):

- [Modelo de componentes](modelo-componentes.md)

Detalle de red, firewall y HTTPS:

- [Modelo de despliegue, red y Web Server](modelo-despliegue-red-web.md)
- [Fuente del diagrama de red](red-gcp.mmd)

Seguridad y manejo de secretos:

- [Secretos y seguridad](secretos-y-seguridad.md)

Decisiones y adaptaciones de empaquetado y configuración:

- [Decisiones y adaptaciones: empaquetado (Daniela)](decisiones-empaquetado-daniela.md)

## Diferencias frente a Entrega 1

- PostgreSQL local se reemplaza por Cloud SQL.
- MinIO local se reemplaza por Cloud Storage.
- La API se publica mediante HTTPS en un Web Server.
- Redis queda en Worker Server y se protege por firewall.
- Los secretos salen del repositorio y se inyectan con archivos `.env` protegidos.

## Evidencias del despliegue real (2026-09-27)

- Dominio HTTPS: `https://35.254.78.215.sslip.io` (Caddy con certificado
  Let's Encrypt).
- VPC personalizada: `mooc-e2-vpc`.
- Subred regional: `mooc-e2-subnet`.
- Web Server (`mooc-e2-web`): zona `us-central1-a`, IP interna `10.20.0.3`,
  IP externa estatica `35.254.78.215`.
- Worker Server (`mooc-e2-worker`): zona `us-central1-a`, IP interna
  `10.20.0.2`, IP externa `34.28.33.182`.
- Instancia Cloud SQL: `mooc-postgres` (IP publica `34.42.6.180`).
- Bucket de Cloud Storage: `mooc-e2-media-proyecto1-entrega2`.
- Evidencia de `curl`:

```text
$ curl -v https://35.254.78.215.sslip.io/health
HTTP/1.1 200 OK
Content-Type: application/json
{"status":"ok"}
```

- Resultado de pruebas Postman/k6 sobre la URL cloud: ver
  `capacity-planning/pruebas_de_carga_entrega2.md` (Escenario 1).

## Red y seguridad de acceso

El despliegue usa una VPC personalizada (`mooc-e2-vpc`) con subred regional
`mooc-e2-subnet`. El Web Server es el unico punto de entrada publico por
HTTP/HTTPS y Caddy termina TLS con Let's Encrypt. Redis se mantiene en el
Worker Server y solo acepta trafico interno desde instancias con etiqueta
`web-server`. El Worker Server conserva IP externa para poder salir hacia
Cloud SQL por IP publica mientras no este completada la conexion privada por
Private Service Access; aun asi, no hay regla publica hacia Redis, ClamAV ni
puertos internos de la aplicacion. La administracion SSH se realiza por IAP
con origen `35.235.240.0/20`.
