# Matriz de verificación del frontend

## Objetivo

Esta matriz registra los principales flujos funcionales del frontend de la Entrega 3, las rutas utilizadas, los endpoints involucrados, el resultado esperado, el estado de validación y la evidencia obtenida.

La validación final se realizó sobre el entorno desplegado en GCP, accediendo al frontend mediante HTTPS a través del balanceador:

`https://136.68.98.101.sslip.io/`

El recorrido principal se ejecutó completamente desde el navegador, incluyendo el flujo conectado profesor → estudiante.

---

## Convenciones

| Estado | Significado |
|---|---|
| ✅ Cloud | Validado correctamente desde el navegador sobre el entorno desplegado en GCP |
| ✅ Local | Validado correctamente en entorno local |
| 🟡 | Implementado, pero pendiente de validación final en cloud |
| ⏳ | Pendiente por dependencia |
| ❌ | No implementado o con error |

---

## 1. Autenticación y navegación por rol

| ID | Rol | Flujo | Ruta UI | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|---|
| F01 | Estudiante | Inicio de sesión | `/login` | El estudiante inicia sesión y accede a su espacio | ✅ Cloud | Login exitoso con cuenta de validación y redirección automática a `/mis-cursos` |
| F02 | Profesor | Inicio de sesión | `/login` | El profesor inicia sesión y accede a su espacio | ✅ Cloud | Login exitoso con cuenta de profesor y acceso a `/profesor/cursos` |
| F03 | Estudiante | Restricción por rol | `/profesor/*` | El estudiante no puede acceder a rutas de profesor | ✅ Local | Validado mediante rutas protegidas; pendiente captura específica cloud |
| F04 | Profesor | Restricción por rol | Rutas de otro rol | El profesor no puede acceder a secciones pertenecientes a otro rol | ✅ Cloud | El acceso a una sección de otro rol redirigió a `/sin-permiso` mostrando “Esta sección pertenece a otro rol” |
| F05 | Usuario autenticado | Cierre de sesión | Global | La sesión se elimina y se redirige al login | ✅ Cloud | Se cerró la sesión del profesor correctamente para iniciar posteriormente la sesión del estudiante |

---

## 2. Catálogo e inscripción

| ID | Rol | Flujo | Ruta UI | Endpoints principales | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|---|---|
| F06 | Estudiante | Ver catálogo | `/catalogo` | `GET /catalog` | Se muestran los cursos publicados | ✅ Cloud | “Curso Validación Frontend E3” apareció correctamente en el catálogo después de ser publicado |
| F07 | Estudiante | Ver detalle de curso | `/cursos/:id` | `GET /courses/:id` | Se muestra la información y el temario del curso | ✅ Cloud | Se mostró el curso con un módulo, una unidad y cuatro recursos publicados: texto, quiz, video y PDF |
| F08 | Estudiante | Inscripción | `/cursos/:id` | `POST /courses/:id/enrollments` | Se crea una inscripción activa | ✅ Cloud | La interfaz mostró “Quedaste inscrito. Ya puedes empezar el curso” y el botón cambió a “Ir al curso” |

---

## 3. Mis cursos y aprendizaje

| ID | Rol | Flujo | Ruta UI | Endpoints principales | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|---|---|
| F09 | Estudiante | Ver cursos inscritos | `/mis-cursos` | `GET /me/enrollments` | Se muestran los cursos donde el estudiante está inscrito | ✅ Cloud | La ruta cargó correctamente al iniciar sesión y mostró inicialmente el estado vacío antes de la inscripción |
| F10 | Estudiante | Entrar al curso | `/aprender/:cursoId` | `GET /courses/:id/outline` | Se muestran módulos, unidades y recursos | ✅ Cloud | Se mostró correctamente “Modulo Validación”, “Unidad 1” y los cuatro recursos publicados |
| F11 | Estudiante | Consultar progreso | `/aprender/:cursoId` | `GET /courses/:id/progress` | Se muestra porcentaje y estado del curso | ✅ Cloud | El curso inició en 0 %, aumentó a 25 % al completar el primer recurso y terminó en 100 % con estado “Aprobado” |

---

## 4. Recursos de aprendizaje

### 4.1 Texto

| ID | Flujo | Endpoints | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|
| F12 | Abrir recurso de texto | `POST /resources/:id/progress/open` + `GET /resources/:id/content` | Se muestra el contenido publicado | ✅ Cloud | “Lectura Validación E3” se abrió correctamente y mostró el contenido publicado |
| F13 | Registrar actividad | `POST /resources/:id/progress/heartbeat` | Se acumula tiempo activo | ✅ Cloud | La interfaz mostró el contador de tiempo activo mientras el recurso permanecía abierto |
| F14 | Completar recurso | `POST /resources/:id/progress/complete` | El recurso aparece como completado y se actualiza el progreso del curso | ✅ Cloud | El recurso pasó a “Completado” y el progreso general aumentó de 0 % a 25 % |

### 4.2 Enlace

| ID | Flujo | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|
| F15 | Abrir enlace externo | El estudiante puede abrir el enlace en una nueva pestaña | ✅ Local | Validado durante pruebas funcionales locales |
| F16 | Completar enlace | El recurso registra progreso y puede marcarse como completado | ✅ Local | Validado durante pruebas funcionales locales |

### 4.3 Contenido embebido

| ID | Flujo | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|
| F17 | Abrir contenido embebido | El contenido externo se muestra dentro de un `iframe` | ✅ Local | Validado con OpenStreetMap |
| F18 | Completar contenido embebido | Se registra actividad y el recurso puede completarse | ✅ Local | Validado durante pruebas funcionales locales |

### 4.4 Multimedia

| ID | Tipo | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|
| F19 | Video | Reproducir HLS desde el navegador y registrar progreso | ✅ Cloud | “Video Validación E3” se reprodujo mediante el visor HLS; el frontend registró la posición reproducida y el recurso pudo marcarse como completado |
| F20 | PDF | Visualizar PDF desde el navegador | ✅ Cloud | “PDF Validación E3” se mostró correctamente dentro del visor embebido y registró tiempo activo |
| F21 | Imagen | Visualizar imagen procesada | 🟡 | Implementado mediante `VisorRecurso`, pero no incluido en el recorrido cloud validado |
| F22 | Audio | Reproducir audio | 🟡 | Implementado mediante `VisorRecurso`, pero no incluido en el recorrido cloud validado |
| F23 | Presentación | Visualizar presentación procesada | 🟡 | Implementado mediante `VisorRecurso`, pero no incluido en el recorrido cloud validado |
| F24 | Archivo | Acceder al archivo disponible | 🟡 | Implementado mediante `VisorRecurso`, pero no incluido en el recorrido cloud validado |

---

## 5. Quiz del estudiante

| ID | Flujo | Ruta UI | Endpoints principales | Resultado esperado | Estado | Evidencia |
|---|---|---|---|---|---|---|
| F25 | Abrir quiz | `/quiz/:quizId` | `GET /resources/:resourceID/quiz`, `POST /quizzes/:id/attempts` | Se inicia o recupera el intento del estudiante | ✅ Cloud | “Quiz Validación E3” abrió correctamente con una pregunta y las opciones configuradas |
| F26 | Responder pregunta | `/quiz/:quizId` | `PUT /attempts/:id/answers/:q` | La respuesta se guarda correctamente | ✅ Cloud | Se seleccionó “Validar el recorrido conectado” y la interfaz mostró 1 de 1 preguntas respondidas |
| F27 | Enviar quiz | `/quiz/:quizId` | `POST /attempts/:id/submit` | Se calcula el resultado una sola vez | ✅ Cloud | La evaluación fue enviada correctamente |
| F28 | Mostrar resultado | `/quiz/:quizId` | `GET /attempts/:id` | Se muestran puntaje, porcentaje y aprobación | ✅ Cloud | Resultado mostrado: 1 punto, 100.0 % y estado “Aprobado” |
| F29 | Recargar resultado | `/quiz/:quizId` | `GET /attempts/:id` | Recargar o cerrar/reabrir no crea un intento adicional | ✅ Cloud | Al recargar la página se mantuvo el resultado existente y no se creó un nuevo intento |
| F30 | Quiz completado | `/aprender/:cursoId` | Progreso del curso | El recurso aparece como “Completado” y el botón cambia a “Ver resultado” | ✅ Cloud | Al volver al curso el quiz apareció como “Completado” y el botón cambió a “Ver resultado” |

---

## 6. Flujo del profesor

| ID | Rol | Flujo | Ruta UI | Estado | Evidencia |
|---|---|---|---|---|---|
| F31 | Profesor | Ver mis cursos | `/profesor/cursos` | ✅ Cloud | La pantalla cargó correctamente y mostró inicialmente el estado vacío del profesor |
| F32 | Profesor | Crear curso | `/profesor/cursos/nuevo` | ✅ Cloud | Se creó “Curso Validación Frontend E3” y se abrió automáticamente el editor de la versión 1 |
| F33 | Profesor | Crear módulos y unidades | Editor de curso | ✅ Cloud | Se creó “Modulo Validación” y dentro de este “Unidad 1” |
| F34 | Profesor | Crear recurso | Editor de curso | ✅ Cloud | Se creó y guardó el recurso de texto “Lectura Validación E3” |
| F35 | Profesor | Crear quiz | `<EditorQuiz>` | ✅ Cloud | Se creó “Quiz Validación E3” con una pregunta, opciones, respuesta correcta y regla de aprobación |
| F36 | Profesor | Cargar archivos | `<CargaArchivo>` | ✅ Cloud | Se cargaron desde el navegador un video y un PDF; ambos alcanzaron el estado “Disponible” |
| F37 | Profesor | Publicar curso | Editor de curso | ✅ Cloud | El curso fue publicado como versión 1 y la interfaz confirmó que ya aparecía en el catálogo |

---

## 7. Recorrido conectado profesor → estudiante

| Paso | Acción | Estado | Evidencia |
|---|---|---|---|
| 1 | Profesor inicia sesión | ✅ Cloud | Acceso correcto al espacio de profesor |
| 2 | Profesor crea curso | ✅ Cloud | “Curso Validación Frontend E3” |
| 3 | Profesor crea módulo y unidad | ✅ Cloud | “Modulo Validación” y “Unidad 1” |
| 4 | Profesor agrega recurso de texto | ✅ Cloud | “Lectura Validación E3” |
| 5 | Profesor crea quiz | ✅ Cloud | “Quiz Validación E3” |
| 6 | Profesor carga video | ✅ Cloud | “Video Validación E3” procesado y disponible |
| 7 | Profesor carga PDF | ✅ Cloud | “PDF Validación E3” procesado y disponible |
| 8 | Profesor publica curso | ✅ Cloud | Versión 1 publicada |
| 9 | Estudiante inicia sesión | ✅ Cloud | Redirección a `/mis-cursos` |
| 10 | Estudiante encuentra el curso | ✅ Cloud | Curso visible en `/catalogo` |
| 11 | Estudiante consulta detalle | ✅ Cloud | Temario y recursos visibles |
| 12 | Estudiante se inscribe | ✅ Cloud | Inscripción confirmada |
| 13 | Estudiante entra al curso | ✅ Cloud | Módulo, unidad y recursos visibles |
| 14 | Estudiante consume recurso de texto | ✅ Cloud | Contenido mostrado correctamente |
| 15 | Estudiante registra progreso | ✅ Cloud | Tiempo activo y completado registrados |
| 16 | Estudiante reproduce video HLS | ✅ Cloud | Video reproducido mediante `VisorRecurso` |
| 17 | Estudiante visualiza PDF | ✅ Cloud | Documento mostrado en visor embebido |
| 18 | Estudiante presenta quiz | ✅ Cloud | Intento creado, respuesta guardada y evaluación enviada |
| 19 | Estudiante consulta resultado | ✅ Cloud | 1 punto, 100 %, Aprobado |
| 20 | Estudiante recarga el resultado | ✅ Cloud | No se crea un segundo intento |
| 21 | Curso actualiza el progreso | ✅ Cloud | Progreso final 100 % y estado “Aprobado” |

---

## 8. Hallazgos durante la validación cloud

### H01. Registro de posición en recursos de video

Durante la validación del recurso multimedia se detectó que el video se reproducía correctamente mediante HLS, pero al intentar marcarlo como completado la API respondía:

`invalid progress event`

Al inspeccionar las solicitudes realizadas desde el navegador se encontró que los heartbeats estaban enviando:

```json
{
  "position_seconds": null
}
```

Esto impedía que el backend verificara qué porcentaje del video había sido reproducido.

Se corrigió la integración entre `VisorRecurso` y `RecursoEstudiante` para propagar la posición actual del reproductor mediante `onTimeUpdate` y enviarla al endpoint de heartbeat.

Después de desplegar la corrección se verificó desde DevTools que las solicitudes enviaban valores reales:

```json
{
  "position_seconds": 3
}
```

Posteriormente el video pudo completarse correctamente y el progreso del curso se actualizó hasta alcanzar el 100 %.

**Estado:** ✅ Corregido y validado en cloud.

---

## 9. Resultado final de la validación cloud

El recorrido funcional principal de la Entrega 3 fue ejecutado completamente desde el navegador contra el entorno desplegado en GCP.

Se validaron satisfactoriamente:

- autenticación de profesor y estudiante;
- navegación y protección de rutas por rol;
- creación de cursos;
- creación de módulos y unidades;
- creación de recursos;
- creación y configuración de quizzes;
- carga directa de archivos multimedia desde el navegador;
- procesamiento de video y PDF;
- publicación del curso;
- catálogo de cursos publicados;
- inscripción del estudiante;
- navegación del contenido académico;
- registro y consulta del progreso;
- reproducción de video HLS;
- visualización de PDF;
- presentación y calificación de quizzes;
- persistencia del resultado del intento;
- actualización del progreso general del curso.

El curso utilizado durante la validación fue:

`Curso Validación Frontend E3`

El recorrido terminó con:

- todos los recursos obligatorios completados;
- quiz aprobado con 100 %;
- progreso general del curso en 100 %;
- estado final del curso: `Aprobado`.

---

## 10. Evidencia técnica del despliegue

El frontend fue accedido mediante HTTPS a través del balanceador:

`https://136.68.98.101.sslip.io/`

La aplicación publica un archivo `version.json` que permite relacionar la versión visible del frontend con el commit desplegado.

Durante la corrección del flujo multimedia se verificó explícitamente que el commit publicado en `version.json` coincidiera con el commit presente en el repositorio antes de repetir la prueba.

Las solicitudes de progreso y multimedia fueron inspeccionadas adicionalmente mediante las herramientas de desarrollo del navegador para verificar:

- envío de `position_seconds`;
- ejecución periódica de heartbeats;
- respuesta de `/progress/complete`;
- persistencia del resultado del quiz;
- comportamiento del visor multimedia.

---

## 11. Evidencias visuales recomendadas

Para el documento final o el video de entrega se recomienda conservar como mínimo las siguientes capturas:

| Evidencia | Contenido |
|---|---|
| E01 | Login y espacio del profesor |
| E02 | Curso creado en el editor |
| E03 | Módulo y unidad creados |
| E04 | Recurso de texto guardado |
| E05 | Quiz creado con pregunta y opciones |
| E06 | Video y PDF en estado “Disponible” |
| E07 | Curso publicado |
| E08 | Catálogo mostrando el curso |
| E09 | Detalle e inscripción |
| E10 | Vista de aprendizaje con progreso inicial 0 % |
| E11 | Recurso de texto abierto y tiempo activo |
| E12 | Progreso actualizado a 25 % |
| E13 | Video reproduciéndose mediante HLS |
| E14 | PDF abierto en el visor |
| E15 | Quiz antes de responder |
| E16 | Quiz respondido |
| E17 | Resultado del quiz: 100 %, Aprobado |
| E18 | Quiz mostrado como “Completado” y “Ver resultado” |
| E19 | Progreso final del curso: 100 %, Aprobado |
| E20 | DevTools mostrando `position_seconds` durante reproducción del video |

---

## 12. Conclusión

La validación sobre GCP demuestra que el frontend permite realizar el recorrido conectado profesor → estudiante requerido para la Entrega 3 sin sustituir los pasos principales por llamadas manuales a la API.

El profesor puede crear y estructurar un curso, agregar contenido académico y multimedia, crear un quiz y publicar el curso. Posteriormente, un estudiante puede encontrarlo en el catálogo, inscribirse, consumir el contenido, registrar progreso, reproducir contenido multimedia, presentar la evaluación y consultar su resultado.

La prueba finalizó con el curso en 100 % de progreso y estado `Aprobado`.

Durante la ejecución también se identificó y corrigió un problema en el registro de la posición de reproducción de los videos, validando posteriormente la corrección directamente sobre el entorno cloud.

## 12. Evidencias visuales

En el archivo Evidencias_Frontend_Entrega3.docx se encuentra una serie de capturas tomadas durante la validación final del frontend en cloud 