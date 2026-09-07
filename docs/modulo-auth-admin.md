# Modulo: Identidad y Administracion (Persona 2)

Este modulo implementa el alcance backend de identidad, sesiones y administracion
de usuarios para la entrega 1.

## Que cubre

- Migracion `000002_create_auth_admin_tables` con `users`, `sessions` y `audit_log`.
- Registro publico de estudiantes en estado `pending_verification`.
- Envio de token de verificacion por SMTP a Mailpit y endpoint para activar la cuenta.
- Login con password hasheado usando bcrypt.
- Sesiones opacas revocables: el hash del token queda en Postgres y el usuario autenticado se cachea en Redis.
- Middleware real de autenticacion que acepta `Authorization: Bearer <token>` o `X-Session-Token`.
- Recuperacion de clave con token temporal enviado a Mailpit.
- Bootstrap del primer admin con `POST /api/v1/auth/bootstrap-admin`, bloqueado si ya existe un admin activo.
- CRUD administrativo basico: listar usuarios, crear profesores, actualizar rol/estado/nombre y eliminar logicamente.
- Revocacion de sesiones desde usuario actual o desde administracion.
- Auditoria en `audit_log` para registro, login, verificacion, recuperacion y operaciones admin.
- Proteccion del ultimo administrador activo y bloqueo de autodegradacion/suspension del admin actual.
- OpenAPI de los endpoints de identidad y administracion.
- Pruebas de casos de uso en `internal/auth/usecase`.

## Endpoints principales

Registro de estudiante:

```bash
curl -X POST localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"ana@example.com","password":"claveSegura123","full_name":"Ana Perez"}'
```

Verificacion de correo:

```bash
curl -X POST localhost:8080/api/v1/auth/verify-email \
  -H "Content-Type: application/json" \
  -d '{"token":"TOKEN_DE_MAILPIT"}'
```

Login:

```bash
curl -X POST localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"ana@example.com","password":"claveSegura123"}'
```

Crear primer admin:

```bash
curl -X POST localhost:8080/api/v1/auth/bootstrap-admin \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"claveSegura123","full_name":"Admin MOOC"}'
```

Crear profesor desde admin:

```bash
curl -X POST localhost:8080/api/v1/admin/users/teachers \
  -H "Authorization: Bearer TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d '{"email":"profe@example.com","password":"claveSegura123","full_name":"Profe Uno"}'
```

## Variables de entorno

- `DATABASE_URL`: conexion PostgreSQL. Por defecto usa `postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable`.
- `REDIS_ADDR`: direccion Redis. Por defecto `localhost:6379`.
- `SMTP_ADDR`: direccion SMTP de Mailpit. Por defecto `localhost:1025`.
- `MAIL_FROM`: remitente de correos. Por defecto `no-reply@mooc.local`.

## Simplificaciones de entrega 1

- Los tokens de verificacion se devuelven tambien como `verification_token_dev` para facilitar pruebas con curl/Postman; en demo se puede comprobar igualmente el correo en Mailpit.
- La auditoria guarda metadata JSON basica y expone los ultimos 200 eventos.
- Las sesiones se validan contra Redis como cache y contra Postgres como fuente persistente.
