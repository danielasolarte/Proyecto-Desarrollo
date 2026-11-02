# Deuda técnica — Entrega 3

Inventario priorizado de la deuda técnica conocida al cierre de la Entrega 3. Reúne el análisis estático de SonarQube (sección 2) y los hallazgos del equipo durante el despliegue y las pruebas de carga (sección 3). Cada ítem indica dónde está, por qué importa, qué se propone y a quién le corresponde.

**Prioridad**
- **Alta:** riesgo de seguridad o de pérdida de servicio en el entorno desplegado. Debe atenderse antes de un uso real.
- **Media:** afecta la operación, la mantenibilidad o la confiabilidad, pero hay una mitigación vigente.
- **Baja:** higiene del repositorio o mejora deseable sin impacto inmediato.

**Esfuerzo estimado:** S (< 1 día), M (1–3 días), L (> 3 días).

---

## 1. Resumen

| Prioridad | Cantidad | Temas principales |
|---|---:|---|
| Alta | 4 | Token de verificación expuesto, login sin límite de intentos, `mooc-servicios` como punto único de falla, correo no configurado en la nube |
| Media | 11 | Pruebas automáticas incompletas, observabilidad sin recolector, Cloud SQL sin HA, despliegue manual desde Windows, cuentas de prueba en la base desplegada, rutas de la SPA con 404, entre otros |
| Baja | 9 | Duplicación de código, archivos grandes, README desactualizado, higiene de `.gitignore`, credenciales por defecto en k6, entre otros |

---

## 2. Análisis estático (SonarQube)

El análisis se ejecuta en GitHub Actions con SonarQube Cloud en cada push a `main` (paso `SonarQube Cloud` del job `backend-ci` en `.github/workflows/ci.yml`), con perfil y quality gate *Sonar way*. Recibe la cobertura de Go (`go test -coverprofile`) y la del frontend (Vitest con `@vitest/coverage-v8`, reporte lcov).

| Métrica | Meta del enunciado | Línea base (primer análisis, 31/10/2026) | Último análisis (01/11/2026, rama `main`) | Cumple |
|---|---|---|---|---|
| Quality gate | — | Passed | **Passed** (todas las condiciones) | ✅ |
| Líneas de código analizadas | — | 14 k | 14 k (backend Go + frontend React/TypeScript) | — |
| Duplicación | ≤ 20 % | 2,3 % | **2,3 %** | ✅ |
| Mantenibilidad (rating) | C o mejor | A (1 issue) | **A** (0 issues) | ✅ |
| Confiabilidad (rating) | — | C (3 issues, severidad media) | **A** (0 issues: corregidos, ver D23) | ✅ |
| Seguridad (rating) | — | B (4 issues, severidad baja) | **B** (4 issues, severidad baja, ver D24) | — |
| Issues totales | — | 7 | **4** (todos de seguridad) | — |
| Cobertura de pruebas | — | 5,3 % (solo Go) | **21,3 %** (Go + frontend) | — |
| Deuda técnica total (mantenibilidad) | registrar | 5 min | **0 min** (ratio 0,0 %) | ✅ registrada |

Lectura:
- Las dos metas del enunciado se cumplen con margen: la duplicación (2,3 %) está muy por debajo del 20 % y la mantenibilidad es A. La deuda de mantenibilidad que mide SonarQube pasó de 5 minutos a 0; la deuda relevante del proyecto es la de diseño y operación de la sección 3, que el análisis estático no ve.
- **Confiabilidad:** los 3 issues de severidad media del primer análisis estaban en el frontend y se corrigieron; el rating subió de C a A (D23, cerrado).
- **Seguridad:** quedan 4 issues de severidad baja, relacionados con el PATH del contenedor del worker; se aceptan con justificación y quedan diferidos (D24).
- **Cobertura:** era de 5,3 % porque SonarQube solo recibía la de Go. Al enviar también la del frontend, la quality gate falló por *Coverage on New Code* (0 % sobre 8 líneas nuevas del frontend que sí tenían pruebas); se agregó el reporte de Vitest al CI y tres pruebas (409 al inscribirse, foco al renombrar y confirmación de eliminar en el editor), y la gate volvió a *Passed* con cobertura general de **21,3 %**. Sigue siendo la métrica más débil por los módulos de Go sin pruebas (D13).
- La configuración del análisis está en `sonar-project.properties`: se analizan `cmd/`, `internal/` y `frontend/src/`; las pruebas cuentan como pruebas y quedan fuera `k6/`, `deploy/` y `postman/` (herramientas, no producto).

---

## 3. Inventario

### 3.1 Seguridad

| Id | Prioridad | Hallazgo | Dónde | Impacto | Propuesta | Esfuerzo | Responsable |
|---|---|---|---|---|---|---|---|
| D01 | **Alta** | `POST /auth/register` devuelve siempre `verification_token_dev`, también en la nube. Cualquiera puede registrarse y verificar un correo que no controla. | `internal/auth/http/handler.go` (`Register`) | Se pierde la garantía de que el correo pertenece al usuario. | Devolver el token solo con una variable explícita de desarrollo (por ejemplo `AUTH_DEV_TOKENS=true`), apagada en las plantillas de la nube. Ajustar `frontend/scripts/crear-cuentas.mjs`, que ya activa al estudiante por el endpoint de administración si el token no llega. | S | Samara |
| D02 | **Alta** | El login no limita intentos por cuenta ni por IP. | `internal/auth/usecase/auth.go` (`Login`), `internal/auth/http/handler.go` | Fuerza bruta de contraseñas. Además, cada intento ejecuta bcrypt (la operación que más CPU consume, como se vio en el Escenario 3A), así que una ráfaga de logins puede forzar el escalamiento del grupo de la API. | Limitador por IP y por correo con contadores en Redis (ventana deslizante) y respuesta 429; opcionalmente una política de Cloud Armor en el balanceador. | M | Samara |
| D03 | Media | El script `09-admin-prueba.ps1` promueve cuentas a administrador con un `UPDATE` directo en Cloud SQL, sin pasar por la API ni dejar registro en `audit_log`. Se hizo porque nadie tenía la clave del administrador de la base desplegada. | `deploy/gcp/e3/09-admin-prueba.ps1`, `deploy/gcp/remoto/promover-admin-prueba.sh` (E2) | Cambios de rol sin trazabilidad; cuentas de prueba (`@mooc.test`) con rol admin en la base desplegada. | Guardar la credencial del administrador inicial en Secret Manager, crear los administradores siguientes por `POST /admin/users/admins` y desactivar las cuentas de prueba al cerrar la entrega. | S | Samara |
| D04 | Media | Redis sin contraseña. | VM `mooc-servicios`, `deploy/docker-compose.servicios.yml` | Mitigado: solo las etiquetas `mooc-api` y `mooc-worker` llegan a 6379 por firewall. Un error de firewall lo dejaría abierto dentro de la VPC. | Activar `requirepass` (o ACL) con la clave en Secret Manager, o migrar a Memorystore con AUTH. | S | Mateo |
| D05 | Baja | El CORS del bucket de medios admite `http://localhost:5173` además del dominio del balanceador. | `deploy/gcp/e3/06-cors-bucket.ps1` | Bajo: las URL siguen firmadas, pero el origen de desarrollo no debería estar en el bucket desplegado. | Usar un bucket de desarrollo aparte o quitar el origen local al cerrar. | S | Daniela |

### 3.2 Disponibilidad y operación

| Id | Prioridad | Hallazgo | Dónde | Impacto | Propuesta | Esfuerzo | Responsable |
|---|---|---|---|---|---|---|---|
| D06 | **Alta** | `mooc-servicios` aloja Redis (cola y sesiones), ClamAV y el exportador de la cola. Es punto único de falla. | `docs/entrega3/arquitectura.md` §10.5 | Si cae, la API pierde sesiones (`/health/ready` en 503) y los workers no procesan ni escalan. El AOF y la IP fija permiten recrearla sin perder la cola. | Memorystore para Redis (con réplica) y ClamAV dentro de la imagen del worker o en un grupo propio. | L | Mateo |
| D07 | **Alta** | El correo no está configurado en la nube: la plantilla de la API no define `SMTP_ADDR` (el valor por defecto es `localhost:1025`, el Mailpit local). | `deploy/gcp/e3/config/api.config`, `03-grupo-api.ps1 -Fase Plantilla` | En la nube no llegan los correos de verificación ni de recuperación de contraseña. Hoy se compensa con D01, que debe cerrarse. | Proveedor SMTP real (SendGrid, Mailgun o Gmail SMTP) con credenciales en Secret Manager. | M | Samara |
| D08 | Media | Cloud SQL `db-g1-small` en una sola zona, sin alta disponibilidad y con `max_connections=50`. | `docs/entrega3/arquitectura.md` §10 | Si cae, toda la aplicación queda fuera (la API sale del balanceador). Con 3 API y 3 workers se usan hasta 30 de las 50 conexiones. | Instancia con HA regional y tamaño dedicado si se mantiene el sistema; revisar `DB_MAX_CONNS` por réplica al subir los límites de los grupos. | M | Daniela |
| D09 | Media | Las alertas de `deploy/observability/alerts.yml` (reglas de Prometheus de la E2) no se evalúan: en la E3 no hay Prometheus que lea `/metrics`, y `/metrics` no se publica por el balanceador. | `deploy/observability/alerts.yml`, `cmd/api/main.go` | Errores 5xx, latencia alta o caída de la API no generan alertas. | Llevar las alertas a Cloud Monitoring (métricas del balanceador: tasa de 5xx y latencia por backend) o desplegar Managed Service for Prometheus. | M | Daniela |
| D10 | Media | El despliegue es manual y depende de Windows: scripts de PowerShell, imagen del contenedor construida con Docker Desktop local e imagen de máquina construida desde el equipo de quien despliega. | `deploy/gcp/e3/*.ps1` | Despliegues difíciles de repetir por otra persona; durante la entrega fallaron Docker Desktop/WSL y una actualización de Windows en equipos del equipo. | Pipeline (GitHub Actions o Cloud Build) que construya contenedor e imagen de máquina por commit. El enunciado no exige despliegue continuo. | L | Daniela |
| D11 | Media | Las rutas internas de la SPA (`/catalogo`, `/cursos/:id`) responden 404 con el contenido de `index.html`, porque el bucket usa `index.html` como página de error. | `deploy/gcp/e3/08-balanceador.ps1` (fase Bucket) | La aplicación funciona en el navegador, pero el código 404 confunde monitoreo, buscadores y herramientas de prueba. | `customErrorResponsePolicy` en el mapa de URL (404 del bucket → `/index.html` con 200), disponible en el balanceador `EXTERNAL_MANAGED`. | S | Samara |
| D12 | Media | Datos de prueba en la base desplegada: cuentas `@mooc.test`, cursos y estudiantes sembrados por k6 y sesiones creadas por miles de logins de carga. | Cloud SQL `mooc-postgres` | Los datos reales y los de prueba se mezclan; la tabla de sesiones crece. | Base separada para pruebas de carga o script de limpieza por dominio `@mooc.test`, y depuración periódica de sesiones vencidas. | S | Samara, Camila |

### 3.3 Calidad del código y pruebas

| Id | Prioridad | Hallazgo | Dónde | Impacto | Propuesta | Esfuerzo | Responsable |
|---|---|---|---|---|---|---|---|
| D13 | Media | Las pruebas automáticas de Go cubren auth, cursos y health, pero no quizzes, progreso, insignias, catálogo ni multimedia (no hay archivos `_test.go` en esos módulos). | `internal/quizzes`, `internal/progress`, `internal/badges`, `internal/catalog`, `internal/media` | Las reglas más delicadas (calificación en servidor, idempotencia del submit, cálculo de progreso, procesamiento HLS) solo se verifican con Postman y k6. | Pruebas de casos de uso con repositorios en memoria, empezando por el submit idempotente y el cálculo de `approved`. | L | Camila, Mateo |
| D14 | Media | El análisis de SonarQube corre en el CI y la *quality gate* pasa, pero no está configurada como check requerido, así que no bloquea un merge. | `.github/workflows/ci.yml` | Las regresiones de calidad se ven en el panel pero no detienen un pull request. | Activar la *quality gate* de SonarQube Cloud como check requerido en `main`. | S | Samara |
| D15 | Baja | Duplicación entre módulos: cada uno tiene su propio `http/errors.go` con el mapeo de errores de dominio a HTTP, y los scripts de k6 repiten los mismos helpers (`login`, `hdr`, `json`, manejo de cookies). | `internal/*/http/errors.go`, `k6/*.js` | Más código que mantener; cambios de formato de error que hay que repetir en varios lugares. | Mapeo común en `internal/platform/apierror` y un módulo `k6/lib/comun.js`. | M | Daniela |
| D16 | Baja | Archivos muy largos con varias responsabilidades: `courses/http/handler.go` (~21 KB), `courses/postgres/course_repository.go` (~21 KB), `quizzes/http/handler.go` (~19 KB), `media/worker/processor.go` y `progress/usecase/service.go` (~15 KB). | `internal/...` | Más difíciles de revisar y probar; SonarQube suele reportarlos por complejidad cognitiva. | Separar por subrecurso (autoría, editor, publicación; preguntas e intentos; etapas del procesador). | M | Daniela, Camila, Mateo |
| D17 | Baja | `FakeAuthMiddleware` sigue en el código, aunque el middleware real reemplazó a los encabezados falsos desde la E1. | `internal/platform/authctx` | Código muerto que podría usarse por error. | Eliminarlo junto con las pruebas antiguas que lo usen. | S | Samara |
| D18 | Baja | Los scripts de k6 traen credenciales por defecto (`admin.k6@mooc.test` / `Admin12345`). | `k6/escenario1.js`, `k6/escenario2.js`, `k6/full-project.js` | Valores de prueba en el código; en la nube no sirven y confunden (pasó en el Escenario 1 de la E3). | Exigirlos por variable de entorno y fallar con un mensaje claro si faltan. | S | Samara |
| D23 | ~~Media~~ **Cerrado** | SonarQube reportó 3 issues de confiabilidad de severidad media en el frontend (rating C). | Pestaña *Issues* de SonarQube Cloud, filtro *Reliability* | Posibles errores en tiempo de ejecución en casos borde. | Corregidos en esta entrega; la confiabilidad quedó en A. | S | Daniela |
| D24 | Baja | SonarQube reporta 4 issues de seguridad de severidad baja (rating B), por el PATH del contenedor del worker. | Pestaña *Issues* de SonarQube Cloud, filtro *Security* | Bajo; endurecimiento recomendado. | Aceptados con justificación y diferidos: fijar un PATH explícito en `deploy/Dockerfile.worker` en la siguiente iteración. | S | Mateo |

### 3.4 Repositorio y documentación

| Id | Prioridad | Hallazgo | Dónde | Impacto | Propuesta | Esfuerzo | Responsable |
|---|---|---|---|---|---|---|---|
| D19 | Baja | El README mezcla las tres entregas: la parte de la E1 dice que multimedia (Persona 3), los Dockerfiles y el CI están pendientes, cosas que ya existen. | `README.md` | Confunde a quien llega al repositorio. | Reorganizarlo por entrega, con el estado actual arriba y enlaces a `docs/entrega3/`. | S | Daniela |
| D20 | Baja | Archivos que no deberían estar en el repositorio o que el `.gitignore` no cubre: `.DS_Store` versionados, el archivo vacío `sh` en la raíz y carpetas locales sin ignorar (`Video y doc/`, `Claude outputs/`). | raíz, `internal/.DS_Store`, `internal/media/.DS_Store` | Riesgo de subir archivos pesados (el video pesa ~300 MB) o personales. | Borrar los archivos sobrantes y agregar `.DS_Store`, `Video y doc/` y `Claude outputs/` al `.gitignore`. | S | Samara |
| D21 | Baja | El dominio `sslip.io` depende de un DNS público gratuito. | Balanceador (`mooc-e3-cert`) | Si el servicio falla o cambia, el certificado deja de renovarse. | Dominio propio con zona en Cloud DNS. | S | Daniela |
| D22 | Baja | Heredado de la E2: 3 filas de `media_assets` en estado `failed` y objetos sin fila asociada en Cloud Storage. | `docs/entrega2/integridad-objetos.md` | Ruido en la reconciliación de trabajos. | Script de limpieza o registro definitivo; confirmar estado con Camila y Mateo. | S | Camila, Mateo |

---

## 4. Deuda saldada en la Entrega 3

Ítems que la Entrega 2 dejó abiertos y que esta entrega cerró:

- Redis y ClamAV dentro de la VM del worker (causa del colapso de C4) → `mooc-servicios`, fuera de los grupos.
- `/health` sin revisar dependencias → `/health/ready` revisa Postgres y Redis y lo usa el balanceador; `/health` queda para la autorreparación.
- API sin apagado ordenado y asynq con `ShutdownTimeout` por defecto → apagado con drenaje (503 en `/health/ready`, 10 s de espera y hasta 20 s más) y `WORKER_SHUTDOWN_TIMEOUT_SECONDS=300`.
- Mailpit publicado por Caddy en el Web Server → no existe en la plantilla de los grupos (ver D07 para el correo real).
- Migraciones desde un portátil → una sola ejecución desde `mooc-servicios` con bloqueo de golang-migrate.
- Imagen de la API anterior a cookies y CSRF (C6) → imágenes fijadas por commit; la sesión y el CSRF se verificaron a través del balanceador (`08-balanceador.ps1 -Fase Sesion`).

Del inventario de esta entrega ya quedaron cerrados **D19** (README reorganizado) y **D23** (issues de confiabilidad de SonarQube), y avanzó **D13** (la cobertura del frontend ya se mide: 21,3 % en total).

---

## 5. Plan sugerido

1. **Antes de cualquier uso real:** D01, D02, D07 (seguridad y correo) y D03 (cuentas de prueba).
2. **Siguiente iteración:** D06 y D08 (puntos únicos de falla), D09 (alertas), D13 y D14 (pruebas y *quality gate*).
3. **Mantenimiento continuo:** D10–D12 y D15–D22.
