# Modelo de despliegue: red, Web Server y seguridad

Responsable: Samara Martinez.

## Region y proyecto

- Proveedor: Google Cloud Platform.
- Proyecto: `proyecto1-entrega2-desarrollo`.
- Region: `us-central1`.
- Zona usada en las plantillas: `us-central1-a`.

## Componentes de red

| Recurso | Nombre propuesto | Proposito |
|---|---|---|
| VPC | `mooc-e2-vpc` | Red privada del despliegue |
| Subred | `mooc-e2-subnet` | Subred regional para Web Server y Worker Server |
| Rango privado | `mooc-e2-private-services` | Private Service Access para Cloud SQL con IP privada |
| IP estatica | `mooc-e2-web-ip` | IP publica fija del Web Server |
| Etiqueta web | `web-server` | Target de reglas publicas y origen permitido hacia Redis |
| Etiqueta worker | `worker-server` | Target de Redis interno |

CIDR sugerido:

```text
10.20.0.0/24
```

## Reglas de firewall

| Regla | Entrada | Origen | Destino | Motivo |
|---|---|---|---|---|
| `mooc-e2-allow-web` | TCP 80, 443 | `0.0.0.0/0` | `web-server` | Acceso publico HTTPS |
| `mooc-e2-allow-redis-from-web` | TCP 6379 | `web-server` | `worker-server` | Sesiones y cola asynq |
| `mooc-e2-allow-iap-ssh` | TCP 22 | `35.235.240.0/20` | web/worker | Administracion por IAP |

Si el equipo decide administrar por SSH directo, reemplazar IAP por una regla TCP 22 limitada a las IPs publicas del equipo.

## Web Server

Configuracion efectiva esperada:

- Compute Engine `e2-small`.
- 2 vCPU.
- 2 GiB de RAM.
- Disco persistente de 30 GiB.
- IP externa estatica.
- Docker y Docker Compose.
- Caddy como proxy HTTPS.

El compose publica la API solo en loopback:

```text
127.0.0.1:8080:8080
```

Caddy escucha en 80/443 y reenvia:

```text
https://<dominio> -> 127.0.0.1:8080
```

Mailpit queda solo local:

```text
127.0.0.1:8025
```

## HTTPS

Sin balanceador no se usan certificados administrados de Google. Se usa Caddy con Let's Encrypt.

Dominio sugerido:

```text
<IP_EXTERNA>.sslip.io
```

Evidencia esperada:

```bash
curl -I https://<dominio>/health
```

Debe responder `200 OK` con certificado valido.

## Worker Server

La decision documentada para esta entrega es Worker Server sin IP publica.

Motivo:

- Redis no soporta password en el cliente actual del proyecto.
- ClamAV y Redis no deben exponerse a Internet.
- La API accede a Redis por IP interna.
- La salida a Internet se resuelve con Cloud NAT si la subred no tiene IP publica y se necesita descargar imagenes o firmas.

Si por presupuesto no se usa Cloud NAT, alternativa aceptada y documentada: VM con IP publica temporal para bootstrap, sin reglas de entrada desde Internet, y luego remover o restringir la IP.

## Comandos reproducibles

Plantillas:

```text
deploy/gcp/00-variables.example.ps1
deploy/gcp/01-network-firewall.ps1
deploy/gcp/02-web-server.ps1
deploy/gcp/03-worker-server.ps1
deploy/gcp/04-deploy-web.ps1
deploy/caddy/Caddyfile.web
```

Orden:

```powershell
. .\deploy\gcp\00-variables.example.ps1
.\deploy\gcp\01-network-firewall.ps1
.\deploy\gcp\02-web-server.ps1
.\deploy\gcp\03-worker-server.ps1
.\deploy\gcp\04-deploy-web.ps1 -Domain "<IP>.sslip.io" -AcmeEmail "correo@ejemplo.com"
```

Los scripts no contienen secretos reales.
