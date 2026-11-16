# Matriz de verificación del frontend · Entrega 4 (Cloud Run)

Responsable: Samara. Repite la matriz de la Entrega 3 ([`docs/entrega3/matriz-verificacion-frontend.md`](../entrega3/matriz-verificacion-frontend.md)) sobre el despliegue en Cloud Run, y agrega lo que cambia en esta entrega: el frontend ahora es una imagen con nginx que sirve la SPA y reenvía `/api/*` al backend, y el procesamiento de archivos pasa por Pub/Sub.

- **Frontend:** https://mooc-frontend-yufxihdm4q-uc.a.run.app
- **Backend:** https://mooc-backend-yufxihdm4q-uc.a.run.app (el navegador no lo llama directamente: todo pasa por `/api` del frontend)
- **Fecha de la verificación:** 14 y 15 de noviembre de 2026, en el navegador integrado de la app de Claude (Chromium 156)
- **Commit desplegado** (el que muestra `/version.json`): `84fd454` (imagen del 2026-11-09T17:05:47Z)
- **Cuentas:** `profesor.prueba@mooc.local` y `estudiante.prueba@mooc.local`, creadas por Camila con la cuenta de administrador (no se escriben aquí las claves).
- **Curso de prueba:** «Matriz E4 Cloud Run» (`2353c165-95d4-4f47-b32c-2c40071e4c18`), con texto, quiz, video y PDF.

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
| C01 | Versión desplegada | Abrir `/version.json` | Muestra el commit de la imagen desplegada | ✅ | `{"commit":"84fd454","fecha_utc":"2026-11-09T17:05:47Z"}` |
| C02 | Rutas profundas de la SPA | Abrir directamente `/catalogo` y recargar con F5 en `/cursos/<id>` | Carga la aplicación (nginx devuelve `index.html` con 200) | ✅ | `/catalogo` y `/cursos/abc123` responden 200 `text/html` con la SPA |
| C03 | Mismo origen | F12 → Red: las llamadas a la API van a `https://mooc-frontend-…/api/v1/...` | Ninguna solicitud del navegador va al dominio del backend; sin errores de CORS | ✅ | Todas las llamadas del navegador van a `https://mooc-frontend-…/api/v1/...` (login, `/me`, cursos, quiz, progreso). Ninguna solicitud al dominio `mooc-backend`; sin errores de CORS |
| C04 | Cookies de sesión | F12 → Aplicación → Cookies, después del login | `mooc_session` con `Secure` y `HttpOnly`; `mooc_csrf` con `Secure` | ✅ | Después del login, `document.cookie` solo muestra `mooc_csrf`: `mooc_session` es `HttpOnly`. El sitio solo se sirve por HTTPS. Al cerrar sesión desaparecen las dos |
| C05 | Procesamiento por Pub/Sub | El profesor sube un video y un PDF (F36) | Cada archivo pasa a «Disponible» sin intervención; el trabajo aparece en los registros del worker | ✅ | El PDF quedó «Disponible» apenas terminó la carga y el video en menos de 30 s, sin intervención. Captura `fe-04-disponible.png` |
| C06 | Worker privado | Abrir la URL del worker en el navegador | 403: el worker no se puede usar desde Internet (detalle en [`evidencias.md`](evidencias.md) §5) | ✅ | El navegador recibe «Error: Forbidden» (HTTP 403) |

## 2. Autenticación y navegación por rol

| Id | Rol | Flujo | Ruta | Resultado esperado | Estado | Observación |
|---|---|---|---|---|---|---|
| F01 | Estudiante | Inicio de sesión | `/login` | Inicia sesión y llega a `/mis-cursos` | ✅ | Inicia sesión como estudiante. Tras el login la aplicación volvió a `/sin-permiso`, la última ruta de la sesión anterior del profesor en la misma pestaña; «Volver a mi espacio» lleva a `/mis-cursos`. Ver H3 |
| F02 | Profesor | Inicio de sesión | `/login` | Inicia sesión y llega a `/profesor/cursos` | ✅ | Inicia sesión y llega a `/profesor/cursos` |
| F03 | Estudiante | Restricción por rol | `/profesor/cursos` | Redirige a «Esta sección pertenece a otro rol» | ✅ | El estudiante en `/profesor/cursos` ve «Esta sección pertenece a otro rol» |
| F04 | Profesor | Restricción por rol | `/mis-cursos` | Redirige a «Esta sección pertenece a otro rol» | ✅ | El profesor en `/mis-cursos` ve «Esta sección pertenece a otro rol» |
| F05 | Usuario autenticado | Cierre de sesión | Global | La sesión se elimina y vuelve al login | ✅ | Vuelve a `/login`, no quedan cookies y `/api/v1/auth/me` responde 401 |

## 3. Flujo del profesor

| Id | Flujo | Pantalla | Resultado esperado | Estado | Observación |
|---|---|---|---|---|---|
| F31 | Ver mis cursos | `/profesor/cursos` | Lista los cursos del profesor o el estado vacío | ✅ | Lista los cursos del profesor, incluidos los de las pruebas de carga |
| F32 | Crear curso | `/profesor/cursos/nuevo` | Crea el curso y abre el editor | ✅ | Crea «Matriz E4 Cloud Run» y abre el editor (versión 1) |
| F33 | Crear módulos y unidades | Editor de curso | Se agregan módulo y unidad | ✅ | «Módulo 1» con «Unidad 1» |
| F34 | Crear recurso de texto | Editor de curso | Se guarda y publica el texto | ✅ | «Introducción»: «Texto guardado y publicado en el borrador» |
| F35 | Crear quiz | Editor de quiz | Quiz con pregunta, opciones y respuesta correcta | ✅ | Una pregunta, dos opciones, «Cloud Run» como correcta; aprueba con 60 % |
| F36 | Cargar video y PDF | Carga de archivos | Los dos llegan a «Disponible» (ver C05) | ✅ | Video y PDF en «Disponible» (C05) |
| F37 | Publicar curso | Editor de curso | Versión 1 publicada; aparece en el catálogo | ✅ | «Curso publicado (versión 1). Ya aparece en el catálogo» |

## 4. Flujo del estudiante

| Id | Flujo | Ruta | Resultado esperado | Estado | Observación |
|---|---|---|---|---|---|
| F06 | Ver catálogo | `/catalogo` | Aparece el curso publicado en F37 | ✅ | Aparece «Matriz E4 Cloud Run» |
| F07 | Ver detalle | `/cursos/:id` | Información y temario del curso | ✅ | 1 módulo · 4 recursos, con el temario completo |
| F08 | Inscribirse | `/cursos/:id` | «Quedaste inscrito» y botón «Ir al curso» | ✅ | «Quedaste inscrito» y botón «Ir al curso» |
| F09 | Mis cursos | `/mis-cursos` | Aparece el curso inscrito | ✅ | Aparece el curso con «Continuar curso» |
| F10 | Entrar al curso | `/aprender/:cursoId` | Módulos, unidades y recursos | ✅ | Módulo, unidad y los cuatro recursos |
| F12 | Leer texto | `/aprender/:cursoId` | Se muestra el contenido publicado | ✅ | Se muestra el texto publicado |
| F14 | Completar recurso | `/aprender/:cursoId` | El recurso queda completado y sube el progreso | ✅ | El texto y el PDF quedan «✓ Completado» después del tiempo mínimo de lectura |
| F19 | Ver video | `/aprender/:cursoId` | Se reproduce el HLS generado por el worker y registra progreso | ❌ | Dos causas. H2 (frontend): Chrome recibía el `.m3u8` directo y no usaba `hls.js`; corregido y desplegado el 15 de noviembre, ahora carga el manifiesto con `hls.js` (duración 19,7 s). H4 (backend): los segmentos responden 403 del bucket privado. Pendiente |
| F20 | Ver PDF | `/aprender/:cursoId` | Se muestra el PDF en el visor | ✅ | El PDF se muestra en el visor (iframe con URL firmada del bucket) y tiene el enlace «Abrir el documento en otra pestaña» |
| F25 | Abrir quiz | `/quiz/:quizId` | Inicia o recupera el intento | ✅ | Abre el intento con la pregunta |
| F27 | Enviar quiz | `/quiz/:quizId` | Se califica una sola vez | ✅ | «Evaluación enviada correctamente»; una sola solicitud `POST /quizzes/:id/attempts` (201) |
| F28 | Ver resultado | `/quiz/:quizId` | Puntaje, porcentaje y aprobación | ✅ | Puntaje 1, 100 %, Aprobado. Captura `fe-06-quiz.png` |
| F29 | Recargar resultado | `/quiz/:quizId` | Recargar no crea otro intento | ✅ | Al salir y volver al quiz se muestra el mismo resultado; no se envió otro `POST` de intento |
| F11 | Progreso del curso | `/aprender/:cursoId` | El porcentaje refleja lo completado | ✅ | 0 % → 33 % con el PDF → 67 % con el quiz (tres recursos obligatorios; el texto no es obligatorio) |

Imagen, audio, presentación, archivo, enlace e iframe (F13, F15–F18, F21–F24) siguen el mismo código de la E3 y no cambiaron en esta entrega; quedan 🟡 salvo que se prueben en este recorrido.

## 5. Hallazgos

| Id | Ruta | Qué pasó | Resolución |
|---|---|---|---|
| H1 | `/api/v1/*` | Con Cloud SQL apagada (`STOPPED`, para ahorrar créditos), la primera llamada a la API devolvió 500 y las siguientes no respondieron en 20 s. El frontend seguía sirviendo la SPA. | No es un defecto del código: se encendió `mooc-e4-postgres` (`--activation-policy=ALWAYS`) y la API respondió de inmediato por el dominio del frontend (401 «no autenticado» sin sesión, que es lo esperado). Antes de cada verificación hay que encender Cloud SQL. |
| H2 | `/aprender/:cursoId` (video) | El video procesado por el worker no se reproduce: `VisorRecurso.tsx` usaba el HLS nativo cuando `canPlayType('application/vnd.apple.mpegurl')` respondía algo. Chrome reciente responde «maybe», así que la app le entregaba el `.m3u8` directo y no cargaba `hls.js`, pero el reproductor nativo de Chrome no abre los segmentos `.ts`. En la E3 funcionaba, probablemente porque el Chrome de entonces respondía una cadena vacía y se usaba `hls.js`. El backend, el worker y el bucket están bien: el manifiesto y los segmentos responden 200. | Ahora se usa `hls.js` siempre que `Hls.isSupported()`; el modo nativo queda solo para navegadores sin MediaSource (Safari en iPhone). Dos pruebas nuevas en `visor.test.tsx` cubren los dos casos. Desplegado el 15 de noviembre: Chrome ya usa `hls.js`. Falta H4 para que el video se reproduzca. |
| H3 | `/login` | Al iniciar sesión como estudiante en la misma pestaña donde el profesor había cerrado sesión desde `/sin-permiso`, la app volvió a `/sin-permiso` en vez de `/mis-cursos`. | Menor: el botón «Volver a mi espacio» lleva a `/mis-cursos`. No se cambió en esta entrega. |
| H4 | `/aprender/:cursoId` (video) | Con el frontend corregido y desplegado (15 de noviembre, 4:51 p. m.), `hls.js` descarga el manifiesto pero cada segmento responde 403 «Anonymous caller does not have storage.objects.get». `GET /resources/:id/playback` firma solo `playlist.m3u8`; dentro del manifiesto los segmentos son rutas relativas (`segment_000.ts`), así que el navegador los pide sin firma, y el bucket de la E4 es privado (`public_access_prevention = enforced`). No depende del navegador. | Sin corregir en esta entrega. Arreglo propuesto, solo en el backend: en `GetPlaybackURL` (`internal/media/http/handler.go`), para HLS, leer el manifiesto del bucket, cambiar cada segmento por su URL prefirmada y devolver el manifiesto reescrito. El bucket sigue privado. |

## 6. Evidencias

Capturas en `docs/entrega4/evidencias/`, con prefijo `fe-`:

| Archivo | Qué muestra |
|---|---|
| `fe-01-version.png` | `/version.json` con el commit (C01) · pendiente |
| `fe-02-mismo-origen.png` | Pestaña Red con las llamadas a `/api` en el dominio del frontend (C03) · pendiente |
| `fe-03-cookies.png` | Cookies `Secure` y `HttpOnly` (C04) · pendiente |
| `fe-04-disponible.png` | Video y PDF en «Disponible» en el editor (C05, F36) |
| `fe-05-video.png` | Video reproduciéndose como estudiante (F19) · no aplica hasta corregir H4 |
| `fe-06-quiz.png` | Resultado del quiz (F28) |
| `fe-07-sin-permiso.png` | Estudiante en una ruta de profesor (F03) · pendiente |

## 7. Resultado

De 32 filas verificadas, 31 ✅ y 1 ❌ (F19, video). Imagen, audio, presentación, archivo, enlace e iframe (F13, F15–F18, F21–F24) quedan 🟡: no cambiaron desde la E3.

El recorrido profesor → estudiante funciona de punta a punta en Cloud Run: el profesor crea el curso, el worker procesa el video y el PDF por Pub/Sub sin intervención, el curso se publica y el estudiante se inscribe, lee el texto y el PDF, presenta el quiz y ve su progreso subir a 67 %. Lo único que no funciona es reproducir el video: el frontend ya se corrigió (H2), pero los segmentos HLS no tienen firma y el bucket privado los rechaza (H4, arreglo propuesto en el backend). Todas las llamadas pasan por el dominio del frontend, la cookie de sesión es `HttpOnly` y el worker no se puede usar desde Internet.
