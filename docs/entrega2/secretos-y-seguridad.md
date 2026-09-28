# Secretos y seguridad de acceso

## Contrato de autenticacion y CSRF

La API mantiene compatibilidad con los scripts y colecciones existentes mediante:

```text
Authorization: Bearer <token>
```

Para clientes web con cookies se habilita con variables de entorno:

```text
AUTH_COOKIE_ENABLED=true
CSRF_ENABLED=true
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_SAMESITE=Lax
SESSION_COOKIE_NAME=mooc_session
CSRF_COOKIE_NAME=mooc_csrf
CSRF_HEADER_NAME=X-CSRF-Token
```

Flujo:

1. `POST /api/v1/auth/login` autentica al usuario.
2. La respuesta mantiene `token` para clientes HTTP y, si `AUTH_COOKIE_ENABLED=true`, tambien devuelve `csrf_token`.
3. La API setea una cookie `mooc_session` con `Secure`, `HttpOnly` y `SameSite`.
4. La API setea una cookie `mooc_csrf` legible por el cliente.
5. Toda peticion autenticada por cookie con metodo no seguro debe enviar:

```text
X-CSRF-Token: <valor de csrf_token o cookie mooc_csrf>
```

Las peticiones con `Authorization: Bearer` no requieren CSRF porque no dependen de cookies automaticas del navegador.

## Secretos

Los secretos no se versionan. En cada VM se usa:

```text
/opt/mooc/deploy/.env
```

Permisos requeridos:

```bash
sudo chmod 600 /opt/mooc/deploy/.env
```

Valores sensibles:

- `DATABASE_URL`
- `S3_ACCESS_KEY`
- `S3_SECRET_KEY`
- credenciales SMTP reales si se reemplaza Mailpit
- tokens o llaves de administracion

Los archivos `*.env.example` conservan solo placeholders.

## Revision antes del tag

Ejecutar:

```powershell
git grep -n -I "password\\|secret\\|access_key\\|BEGIN PRIVATE KEY\\|AIza\\|postgres://" -- . ":(exclude)*.example" ":(exclude)go.sum"
```

Revisar manualmente capturas, video y README para no publicar:

- contrasenas reales;
- llaves HMAC;
- IPs administrativas personales si el equipo prefiere mantenerlas privadas;
- tokens de sesion;
- cadenas completas de conexion con usuario y clave.
