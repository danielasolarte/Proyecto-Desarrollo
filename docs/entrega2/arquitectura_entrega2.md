# Arquitectura Entrega 2

Este documento consolida la arquitectura desplegada para la Entrega 2. Las secciones de red y Web Server fueron preparadas por Samara Martinez.

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

- [Modelo de despliegue de Samara](modelo-despliegue-samara.md)
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
- IP estatica del Web Server (`mooc-e2-web`): `35.254.78.215` (interna
  `10.128.0.3`).
- IP interna del Worker Server (`mooc-e2-worker`): `10.128.0.2` (tambien
  tiene IP externa `34.61.4.185`, desviacion registrada; ver
  `docs/costos-entrega2.md`).
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

## Desviaciones frente al diseno de red original

Se desplego sobre la red `default` (modo automatico) en vez de la VPC
personalizada `mooc-e2-vpc`, y el Worker Server quedo con IP externa en vez
de solo salida por Cloud NAT. Detalle completo en
`docs/costos-entrega2.md`.
