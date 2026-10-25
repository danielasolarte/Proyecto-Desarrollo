# Matriz de verificación del frontend

## Objetivo

Esta matriz registra los flujos funcionales del frontend de la Entrega 3,
las rutas utilizadas, los endpoints involucrados, el resultado esperado,
el estado actual y la evidencia asociada.

La validación final se realizará sobre el entorno desplegado en GCP.
Durante el desarrollo también se registran pruebas locales para verificar
la integración funcional antes del despliegue.

---

## Convenciones

| Estado | Significado |
|---|---|
| ✅ | Validado correctamente |
| 🟡 | Implementado, pendiente validación final en cloud |
| ⏳ | Pendiente por dependencia |
| ❌ | No implementado o con error |

---

## 1. Autenticación y navegación por rol

| ID | Rol | Flujo | Ruta UI | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|---|
| F01 | Estudiante | Inicio de sesión | `/login` | El estudiante inicia sesión y accede a su espacio | ✅ Local | Pendiente captura |
| F02 | Profesor | Inicio de sesión | `/login` | El profesor inicia sesión y accede a su espacio | ✅ Local | Pendiente captura |
| F03 | Estudiante | Restricción por rol | `/profesor/*` | El estudiante no puede acceder a rutas de profesor | ✅ Local | Pendiente captura |
| F04 | Profesor | Restricción por rol | `/aprender/:cursoId` | El profesor no puede consumir un curso como estudiante | ✅ Local | Pendiente captura |
| F05 | Usuario autenticado | Cierre de sesión | Global | La sesión se elimina y se redirige al login | 🟡 | Pendiente validar |

---

## 2. Catálogo e inscripción

| ID | Rol | Flujo | Ruta UI | Endpoints principales | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|---|
| F06 | Estudiante | Ver catálogo | `/catalogo` | `GET /catalog` | Se muestran los cursos publicados | ✅ Local | Pendiente captura |
| F07 | Estudiante | Ver detalle de curso | `/cursos/:id` | `GET /courses/:id` | Se muestra la información del curso | ✅ Local | Pendiente captura |
| F08 | Estudiante | Inscripción | `/cursos/:id` | `POST /courses/:id/enrollments` | Se crea una inscripción activa | ✅ Local | Validado con curso de prueba |

---

## 3. Mis cursos y aprendizaje

| ID | Rol | Flujo | Ruta UI | Endpoints principales | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|---|
| F09 | Estudiante | Ver cursos inscritos | `/mis-cursos` | `GET /me/enrollments` | Se muestran los cursos donde el estudiante está inscrito | ✅ Local | Pendiente captura |
| F10 | Estudiante | Entrar al curso | `/aprender/:cursoId` | `GET /courses/:id/outline` | Se muestran módulos, unidades y recursos | ✅ Local | Pendiente captura |
| F11 | Estudiante | Consultar progreso | `/aprender/:cursoId` | `GET /courses/:id/progress` | Se muestra porcentaje y estado del curso | ✅ Local | Evidencia: curso con 100 % y estado Aprobado |

---

## 4. Recursos de aprendizaje

### 4.1 Texto

| ID | Flujo | Endpoints | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|
| F12 | Abrir recurso de texto | `POST /resources/:id/progress/open` + `GET /resources/:id/content` | Se muestra el contenido publicado | ✅ Local | Validado con “Lectura de Prueba” |
| F13 | Registrar actividad | `POST /resources/:id/progress/heartbeat` | Se acumula tiempo activo | ✅ Local | Validado |
| F14 | Completar recurso | `POST /resources/:id/progress/complete` | El recurso aparece como completado y se actualiza el progreso del curso | ✅ Local | Validado |

### 4.2 Enlace

| ID | Flujo | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|
| F15 | Abrir enlace externo | El estudiante puede abrir el enlace en una nueva pestaña | ✅ Local | Validado |
| F16 | Completar enlace | El recurso registra progreso y puede marcarse como completado | ✅ Local | Validado |

### 4.3 Contenido embebido

| ID | Flujo | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|
| F17 | Abrir contenido embebido | El contenido externo se muestra dentro de un `iframe` | ✅ Local | Validado con OpenStreetMap |
| F18 | Completar contenido embebido | Se registra actividad y el recurso puede completarse | ✅ Local | Validado |

### 4.4 Multimedia

| ID | Tipo | Resultado esperado | Estado | Dependencia |
|---|---|---|---|---|
| F19 | Video | Reproducir HLS desde el navegador | ⏳ | `<VisorRecurso>` de Mateo |
| F20 | PDF | Visualizar PDF desde el navegador | ⏳ | `<VisorRecurso>` de Mateo |
| F21 | Imagen | Visualizar imagen procesada | ⏳ | `<VisorRecurso>` de Mateo |
| F22 | Audio | Reproducir audio | ⏳ | `<VisorRecurso>` de Mateo |
| F23 | Presentación | Visualizar presentación procesada | ⏳ | `<VisorRecurso>` de Mateo |
| F24 | Archivo | Acceder al archivo disponible | ⏳ | `<VisorRecurso>` de Mateo |

---

## 5. Quiz del estudiante

| ID | Flujo | Ruta UI | Endpoints principales | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|---|
| F25 | Abrir quiz | `/quiz/:quizId` | `GET /resources/:resourceID/quiz`, `POST /quizzes/:id/attempts` | Se inicia o recupera el intento del estudiante | ✅ Local | Validado |
| F26 | Responder pregunta | `/quiz/:quizId` | `PUT /attempts/:id/answers/:q` | La respuesta se guarda correctamente | ✅ Local | Validado |
| F27 | Enviar quiz | `/quiz/:quizId` | `POST /attempts/:id/submit` | Se calcula el resultado una sola vez | ✅ Local | Validado |
| F28 | Mostrar resultado | `/quiz/:quizId` | `GET /attempts/:id` | Se muestran puntaje, porcentaje y aprobación | ✅ Local | 1 punto, 100 %, Aprobado |
| F29 | Recargar resultado | `/quiz/:quizId` | `GET /attempts/:id` | Recargar o cerrar/reabrir no crea un intento adicional | ✅ Local | Validado |
| F30 | Quiz completado | `/aprender/:cursoId` | Progreso del curso | El recurso aparece como “Completado” y el botón cambia a “Ver resultado” | ✅ Local | Validado |

---

## 6. Flujo del profesor

| ID | Rol | Flujo | Ruta UI | Estado | Responsable |
|---|---|---|---|---|---|
| F31 | Profesor | Ver mis cursos | `/profesor/cursos` | 🟡 | Daniela |
| F32 | Profesor | Crear curso | `/profesor/cursos/nuevo` | 🟡 | Daniela |
| F33 | Profesor | Crear módulos y unidades | Editor de curso | 🟡 | Daniela |
| F34 | Profesor | Crear recurso | Editor de curso | 🟡 | Daniela |
| F35 | Profesor | Crear quiz | `<EditorQuiz>` | 🟡 | Camila / ya implementado |
| F36 | Profesor | Cargar archivos | `<CargaArchivo>` | ⏳ | Mateo |
| F37 | Profesor | Publicar curso | Editor de curso | 🟡 | Daniela |

---

## 7. Recorrido conectado profesor → estudiante

| Paso | Acción | Estado |
|---|---|---|
| 1 | Profesor crea curso | 🟡 |
| 2 | Profesor crea módulo y unidad | 🟡 |
| 3 | Profesor agrega recursos | 🟡 |
| 4 | Profesor crea quiz | ✅ Implementado |
| 5 | Profesor publica curso | 🟡 |
| 6 | Estudiante encuentra el curso | ✅ |
| 7 | Estudiante se inscribe | ✅ |
| 8 | Estudiante entra a “Mis cursos” | ✅ |
| 9 | Estudiante navega módulos, unidades y recursos | ✅ |
| 10 | Estudiante consume recurso de texto/enlace/iframe | ✅ |
| 11 | Estudiante registra progreso | ✅ |
| 12 | Estudiante presenta quiz | ✅ |
| 13 | Estudiante consulta resultado | ✅ |
| 14 | Curso actualiza el porcentaje de progreso | ✅ |
| 15 | Estudiante consume multimedia | ⏳ Mateo |

---

## 8. Validación pendiente en cloud

Las pruebas anteriores fueron realizadas inicialmente en el entorno local.
Antes de cerrar la Entrega 3 deben repetirse sobre el entorno desplegado en
GCP para verificar:

- navegación completa mediante HTTPS;
- sesión válida detrás del balanceador;
- acceso a cursos desde múltiples réplicas de API;
- persistencia del progreso independientemente de la réplica;
- integración del visor multimedia;
- carga de archivos desde navegador;
- recorrido conectado completo profesor-estudiante;
- evidencia final con fecha y commit.

---

## 9. Evidencias

Para cada flujo validado en cloud se debe registrar:

- captura de pantalla;
- fecha y hora;
- commit utilizado;
- URL o ruta probada;
- resultado esperado;
- resultado observado;
- observaciones si hubo diferencias.