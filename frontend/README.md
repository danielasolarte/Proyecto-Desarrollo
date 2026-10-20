# Base del frontend · Entrega 3

React + Vite + TypeScript. Incluye login/logout reales, restauración de sesión con
`GET /api/v1/me`, menú y protección de rutas por rol, cliente HTTP y estados comunes.
Los módulos de cursos, multimedia, progreso y quizzes tienen espacios de integración;
todavía no ejecutan los recorridos funcionales de la entrega.

## Ejecutar en local

Requisitos: Node 20 o superior y backend con migraciones aplicadas (ver README raíz).
Desde la raíz, si no existe `.env`, copiar `.env.example` a `.env`. No sobrescribir
una configuración existente. Con Docker Desktop iniciado:

```powershell
docker compose -f docker-compose.yml -f deploy/docker-compose.frontend-local.yml up -d --build
```

El segundo archivo activa cookies y CSRF para desarrollo HTTP. En nube se requiere
HTTPS y `SESSION_COOKIE_SECURE=true`. Conserva los nombres `mooc_session`, `mooc_csrf`
y `X-CSRF-Token`, que usa el cliente compartido. No uses ese override en nube.

Si la base es nueva, aplicar las migraciones una sola vez según el README raíz.
Después:

```powershell
cd frontend
npm ci
npm run dev
```

Abrir **http://localhost:5173**. El proxy envía `/api` a `http://localhost:8080`.
Para cambiar el backend local, copiar `.env.example` a `.env.local` y ajustar
`API_PROXY_TARGET`. No es necesario habilitar CORS. Acceder siempre por el puerto
del frontend para que las cookies queden en el mismo origen.

## Cuentas de prueba (local o cloud)

El script usa endpoints existentes y verifica el login de profesor y estudiante.
Puede repetirse: reutiliza usuarios existentes, comprueba su rol y estado y no
cambia contraseñas. Si ya existe un administrador, proporcionar sus credenciales;
en una instalación vacía crea el primero. Activa únicamente el estudiante de prueba
pendiente de verificación mediante el endpoint administrativo. No reactiva cuentas
suspendidas ni cambia roles.

Desde `frontend/`, definir las variables de esta lista en la sesión de PowerShell:

| Variable | Contenido |
| --- | --- |
| `MOOC_API_URL` | `http://localhost:8080/api/v1` (predeterminado) o URL HTTPS de la API cloud |
| `MOOC_ADMIN_EMAIL`, `MOOC_ADMIN_PASSWORD` | Administrador existente o primer administrador |
| `MOOC_TEACHER_EMAIL`, `MOOC_TEACHER_PASSWORD` | Profesor de prueba |
| `MOOC_STUDENT_EMAIL`, `MOOC_STUDENT_PASSWORD` | Estudiante de prueba |

Para introducir contraseñas sin escribirlas en el historial:

```powershell
$env:MOOC_ADMIN_EMAIL = Read-Host 'Correo administrador'
$env:MOOC_TEACHER_EMAIL = Read-Host 'Correo profesor de prueba'
$env:MOOC_STUDENT_EMAIL = Read-Host 'Correo estudiante de prueba'
$env:MOOC_ADMIN_PASSWORD = [System.Net.NetworkCredential]::new('', (Read-Host 'Clave administrador' -AsSecureString)).Password
$env:MOOC_TEACHER_PASSWORD = [System.Net.NetworkCredential]::new('', (Read-Host 'Clave profesor' -AsSecureString)).Password
$env:MOOC_STUDENT_PASSWORD = [System.Net.NetworkCredential]::new('', (Read-Host 'Clave estudiante' -AsSecureString)).Password
npm run cuentas
Remove-Item Env:MOOC_ADMIN_PASSWORD, Env:MOOC_TEACHER_PASSWORD, Env:MOOC_STUDENT_PASSWORD
```

En nube, definir primero `$env:MOOC_API_URL` apuntando al balanceador HTTPS.
El script no imprime ni guarda tokens o contraseñas. Informa errores reales del
backend y cierra las sesiones que crea. No envía credenciales mediante HTTP remoto.

## Contrato para integrar pantallas

- Usar `cliente<T>('/ruta', { method, body })` de `src/api/cliente.ts`.
  La base `/api/v1` se agrega automáticamente. No usar `fetch` directamente en pantallas.
- El cliente incluye cookies, envía CSRF para POST/PUT/PATCH/DELETE y admite respuestas 204.
  Los errores exponen `status`, `code` y `message`. Un 401 autenticado vuelve al login.
- `useSesion()` expone `usuario`, `cargando`, `ingresar`, `salir` y `recargar`.
  Roles reales del backend: `student`, `teacher`, `admin`. No se guardan tokens en Web Storage.
- `RutaProtegida` bloquea vistas por rol; el backend sigue validando permisos.
- Reutilizar `Cargando`, `Vacio`, `ErrorEstado` (también exportado como `Error`),
  `Exito` y `Procesando` de `src/componentes/estados/Estados.tsx`.

| Responsable | Carpetas reservadas | Rutas / componentes |
| --- | --- | --- |
| Samara | `src/auth`, `src/api`, `src/paginas/Login.tsx` | `/login`, sesión, menú, estados |
| Daniela | `src/paginas/catalogo`, `src/paginas/profesor` | `/catalogo`, `/cursos/:id`, `/profesor/cursos`, crear/editar |
| Mateo | `src/componentes/media` | `CargaArchivo`, `VisorRecurso` |
| Camila | `src/paginas/estudiante`, `src/componentes/quiz` | `/mis-cursos`, `/aprender/:cursoId`, `/quiz/:quizId`, `EditorQuiz` |

Sustituir los elementos `Inicio` de `src/App.tsx` por las pantallas reales dentro
del guard de su rol. Acordar props de componentes multimedia y quiz entre sus autores.
El administrador tiene una página de entrada reservada; no se implementa aquí un CRUD.

## Verificación y compilación

```powershell
npm test
npm run build
```

Los tests comprueban cookies/CSRF, errores, restauración de sesión, login/logout y
bloqueo de un estudiante al entrar a profesor. Usan respuestas simuladas: no sustituyen
la prueba conectada al backend y la nube. El pipeline ejecuta estos checks con `npm ci`.

`npm run build` genera `dist/`. Para identificar una compilación:
`$env:APP_COMMIT = git rev-parse --short HEAD` antes de construir. La versión aparece
en el pie. La publicación al bucket, `version.json` y el balanceador se implementarán
en el siguiente frente; este cambio no publica infraestructura. El hosting deberá
resolver las rutas de la SPA hacia `index.html` sin capturar `/api/*`.
