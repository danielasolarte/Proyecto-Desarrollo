# Matriz de verificación del frontend · Entrega 4 (Cloud Run)

Responsable: Samara. Repite la matriz de la Entrega 3 ([`docs/entrega3/matriz-verificacion-frontend.md`](../entrega3/matriz-verificacion-frontend.md)) sobre el despliegue en Cloud Run, y agrega lo que cambia en esta entrega: el frontend ahora es una imagen con nginx que sirve la SPA y reenvía `/api/*` al backend, y el procesamiento de archivos pasa por Pub/Sub.

- **Frontend:** https://mooc-frontend-yufxihdm4q-uc.a.run.app
- **Backend:** https://mooc-backend-yufxihdm4q-uc.a.run.app (el navegador no lo llama directamente: todo pasa por `/api` del frontend)
- **Fecha de la verificación:** _completar_
- **Commit desplegado** (el que muestra `/version.json`): _completar_
- **Cuentas:** profesor y estudiante de prueba creadas por Camila con la cuenta de administrador (no se escriben aquí las claves).

## Convenciones

| Símbolo | Significado |
|---|---|
| ✅ | Verificado en Cloud Run |
| ❌ | Falla en Cloud Run (se describe en Hallazgos) |
| 🟡 | Implementado, no incluido en el recorrido de esta verificación |
| ⬜ | Pendiente de verificar |

## 1. Lo nuevo de la Entrega 4

| Id | Qué se verifica | Cómo | Resultado esperado | Estado | Observación |
|---|---|---|---|---|---|
| C01 | Versión desplegada | Abrir `/version.json` | Muestra el commit de la imagen desplegada | ⬜ | |
| C02 | Rutas profundas de la SPA | Abrir directamente `/catalogo` y recargar con F5 en `/cursos/<id>` | Carga la aplicación (nginx devuelve `index.html` con 200) | ⬜ | |
| C03 | Mismo origen | F12 → Red: las llamadas a la API van a `https://mooc-frontend-…/api/v1/...` | Ninguna solicitud del navegador va al dominio del backend; sin errores de CORS | ⬜ | |
| C04 | Cookies de sesión | F12 → Aplicación → Cookies, después del login | `mooc_session` con `Secure` y `HttpOnly`; `mooc_csrf` con `Secure` | ⬜ | |
| C05 | Procesamiento por Pub/Sub | El profesor sube un video y un PDF (F36) | Cada archivo pasa a «Disponible» sin intervención; el trabajo aparece en los registros del worker | ⬜ | |
| C06 | Worker privado | Abrir la URL del worker en el navegador | 403: el worker no se puede usar desde Internet (detalle en [`evidencias.md`](evidencias.md) §5) | ⬜ | |

## 2. Autenticación y navegación por rol

| Id | Rol | Flujo | Ruta | Resultado esperado | Estado | Observación |
|---|---|---|---|---|---|---|
| F01 | Estudiante | Inicio de sesión | `/login` | Inicia sesión y llega a `/mis-cursos` | ⬜ | |
| F02 | Profesor | Inicio de sesión | `/login` | Inicia sesión y llega a `/profesor/cursos` | ⬜ | |
| F03 | Estudiante | Restricción por rol | `/profesor/cursos` | Redirige a «Esta sección pertenece a otro rol» | ⬜ | |
| F04 | Profesor | Restricción por rol | `/mis-cursos` | Redirige a «Esta sección pertenece a otro rol» | ⬜ | |
| F05 | Usuario autenticado | Cierre de sesión | Global | La sesión se elimina y vuelve al login | ⬜ | |

## 3. Flujo del profesor

| Id | Flujo | Pantalla | Resultado esperado | Estado | Observación |
|---|---|---|---|---|---|
| F31 | Ver mis cursos | `/profesor/cursos` | Lista los cursos del profesor o el estado vacío | ⬜ | |
| F32 | Crear curso | `/profesor/cursos/nuevo` | Crea el curso y abre el editor | ⬜ | |
| F33 | Crear módulos y unidades | Editor de curso | Se agregan módulo y unidad | ⬜ | |
| F34 | Crear recurso de texto | Editor de curso | Se guarda y publica el texto | ⬜ | |
| F35 | Crear quiz | Editor de quiz | Quiz con pregunta, opciones y respuesta correcta | ⬜ | |
| F36 | Cargar video y PDF | Carga de archivos | Los dos llegan a «Disponible» (ver C05) | ⬜ | |
| F37 | Publicar curso | Editor de curso | Versión 1 publicada; aparece en el catálogo | ⬜ | |

## 4. Flujo del estudiante

| Id | Flujo | Ruta | Resultado esperado | Estado | Observación |
|---|---|---|---|---|---|
| F06 | Ver catálogo | `/catalogo` | Aparece el curso publicado en F37 | ⬜ | |
| F07 | Ver detalle | `/cursos/:id` | Información y temario del curso | ⬜ | |
| F08 | Inscribirse | `/cursos/:id` | «Quedaste inscrito» y botón «Ir al curso» | ⬜ | |
| F09 | Mis cursos | `/mis-cursos` | Aparece el curso inscrito | ⬜ | |
| F10 | Entrar al curso | `/aprender/:cursoId` | Módulos, unidades y recursos | ⬜ | |
| F12 | Leer texto | `/aprender/:cursoId` | Se muestra el contenido publicado | ⬜ | |
| F14 | Completar recurso | `/aprender/:cursoId` | El recurso queda completado y sube el progreso | ⬜ | |
| F19 | Ver video | `/aprender/:cursoId` | Se reproduce el HLS generado por el worker y registra progreso | ⬜ | |
| F20 | Ver PDF | `/aprender/:cursoId` | Se muestra el PDF en el visor | ⬜ | |
| F25 | Abrir quiz | `/quiz/:quizId` | Inicia o recupera el intento | ⬜ | |
| F27 | Enviar quiz | `/quiz/:quizId` | Se califica una sola vez | ⬜ | |
| F28 | Ver resultado | `/quiz/:quizId` | Puntaje, porcentaje y aprobación | ⬜ | |
| F29 | Recargar resultado | `/quiz/:quizId` | Recargar no crea otro intento | ⬜ | |
| F11 | Progreso del curso | `/aprender/:cursoId` | El porcentaje refleja lo completado | ⬜ | |

Imagen, audio, presentación, archivo, enlace e iframe (F13, F15–F18, F21–F24) siguen el mismo código de la E3 y no cambiaron en esta entrega; quedan 🟡 salvo que se prueben en este recorrido.

## 5. Hallazgos

_Describir aquí cualquier ❌, con la ruta, lo que pasó y cómo se resolvió._

## 6. Evidencias

Capturas en `docs/entrega4/evidencias/`, con prefijo `fe-`:

| Archivo | Qué muestra |
|---|---|
| `fe-01-version.png` | `/version.json` con el commit (C01) |
| `fe-02-mismo-origen.png` | Pestaña Red con las llamadas a `/api` en el dominio del frontend (C03) |
| `fe-03-cookies.png` | Cookies `Secure` y `HttpOnly` (C04) |
| `fe-04-disponible.png` | Video y PDF en «Disponible» en el editor (C05, F36) |
| `fe-05-video.png` | Video reproduciéndose como estudiante (F19) |
| `fe-06-quiz.png` | Resultado del quiz (F28) |
| `fe-07-sin-permiso.png` | Estudiante en una ruta de profesor (F03) |

## 7. Resultado

_Completar al final: cuántos flujos ✅, cuántos 🟡 y si el recorrido profesor → estudiante funciona de punta a punta en Cloud Run._
