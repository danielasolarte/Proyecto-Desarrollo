# Pruebas de integridad entre réplicas — Entrega 3

## Objetivo

Verificar que el sistema conserva consistencia funcional cuando las solicitudes
de un mismo usuario son atendidas por diferentes instancias del MIG de API.

Las pruebas se ejecutarán contra el balanceador HTTPS de la Entrega 3, con al
menos dos instancias sanas de API registradas simultáneamente.

---

## Precondiciones

Antes de ejecutar las pruebas se debe verificar:

- MIG de API con mínimo 2 instancias sanas.
- Balanceador HTTPS operativo.
- `GET /health/ready` devuelve HTTP 200.
- Las respuestas incluyen `X-Instance-Id`.
- Cloud SQL está disponible mediante IP privada.
- Redis compartido está disponible para todas las réplicas.
- Existe una cuenta de estudiante activa.
- Existe un curso publicado.
- El estudiante está inscrito en el curso.
- Existe al menos un quiz publicado accesible al estudiante.
- Commit y versión utilizados registrados antes de iniciar.

---

## Evidencia común

Para cada prueba se debe registrar:

- fecha y hora UTC;
- commit probado;
- URL del balanceador;
- `X-Instance-Id` de cada solicitud;
- código HTTP;
- identificadores relevantes;
- estado final observado;
- resultado esperado;
- PASS / FAIL.

---

# I1. Sesión válida entre réplicas

## Objetivo

Comprobar que una sesión creada por una réplica puede ser utilizada por otra.

## Procedimiento

1. Iniciar sesión como estudiante.
2. Conservar la cookie de sesión.
3. Ejecutar múltiples solicitudes autenticadas a `/api/v1/me`.
4. Registrar `X-Instance-Id` de cada respuesta.
5. Continuar hasta observar al menos dos instancias diferentes.

## Resultado esperado

- Todas las solicitudes autenticadas responden HTTP 200.
- Se observa al menos dos valores diferentes de `X-Instance-Id`.
- El usuario obtenido es el mismo en todas las respuestas.
- No aparecen respuestas `401 Unauthorized`.

## Criterio

PASS si la misma sesión permanece válida al cambiar de réplica.

---

# I2. Escritura en una réplica y lectura desde otra

## Objetivo

Comprobar que los datos de negocio no dependen del almacenamiento local de una
VM de API.

## Procedimiento

1. Con una sesión autenticada, realizar una escritura persistente.
2. Registrar `X-Instance-Id` de la respuesta.
3. Consultar inmediatamente el dato mediante múltiples solicitudes.
4. Continuar hasta que una lectura sea atendida por otra instancia.

Ejemplo de dato:

- progreso de un recurso;
- respuesta de quiz;
- inscripción;
- otra operación persistida en Cloud SQL.

## Resultado esperado

- La escritura responde correctamente.
- Una réplica diferente puede leer inmediatamente el nuevo estado.
- El valor leído coincide con el valor persistido.

## Criterio

PASS si una escritura realizada mediante una instancia es observable desde otra.

---

# I3. Doble envío de quiz sin doble calificación

## Objetivo

Verificar la idempotencia del envío cuando solicitudes duplicadas pueden ser
atendidas por réplicas diferentes.

## Procedimiento

1. Crear un intento de quiz.
2. Guardar todas las respuestas.
3. Generar una única `Idempotency-Key`.
4. Lanzar dos solicitudes concurrentes de submit para el mismo `attempt_id`.
5. Registrar:
   - HTTP status;
   - `X-Instance-Id`;
   - score;
   - submitted_at.
6. Consultar el intento después de ambas respuestas.
7. Verificar en PostgreSQL la cantidad de intentos/calificaciones persistidas.

## Resultado esperado

- No se crea una segunda calificación.
- El intento termina una sola vez en `submitted`.
- El score persistido es único.
- Los reintentos devuelven un resultado coherente.
- No aparecen eventos de progreso duplicados asociados al mismo submit.

## Criterio

PASS si dos submits concurrentes producen un único efecto persistente.

---

# I4. Acceso a recursos desde diferentes réplicas

## Objetivo

Comprobar que una API concreta no es propietaria del acceso a los recursos.

## Procedimiento

1. Consultar un recurso publicado como estudiante inscrito.
2. Repetir la solicitud hasta obtener respuestas de al menos dos
   `X-Instance-Id`.
3. Comparar metadatos y estado devuelto.

Cuando el visor multimedia esté integrado, repetir la validación sobre el
flujo correspondiente.

## Resultado esperado

- Todas las réplicas autorizan correctamente al estudiante.
- Los metadatos del recurso son consistentes.
- Ninguna réplica depende de archivos locales para responder.

## Criterio

PASS si el mismo recurso es accesible independientemente de la réplica de API.

---

# I5. Persistencia después del retiro de una API

## Objetivo

Comprobar que retirar una réplica no elimina ni altera datos de negocio.

## Procedimiento

1. Crear o modificar un dato persistente.
2. Registrar el `X-Instance-Id` que atendió la operación.
3. Confirmar el valor mediante una lectura.
4. Provocar o esperar el retiro controlado de una instancia del MIG.
5. Volver a consultar el dato mediante el balanceador.
6. Registrar la nueva instancia que responde.

## Resultado esperado

- El dato sigue existiendo después del retiro.
- La sesión continúa siendo válida.
- Una réplica restante devuelve el mismo estado.
- No se pierde progreso, intento, inscripción ni otro dato persistido.

## Criterio

PASS si los datos sobreviven al retiro de la instancia que intervino en la
operación original.

---

# Resumen

| ID | Propiedad | Validación local | Validación GCP |
|---|---|---|---|
| I1 | Sesión entre réplicas | ✅ PASS | ⏳ Pendiente |
| I2 | Escritura / lectura consistente | ✅ PASS | ⏳ Pendiente |
| I3 | Submit idempotente | ✅ PASS | ⏳ Pendiente |
| I4 | Recursos entre réplicas | ✅ PASS | ⏳ Pendiente |
| I5 | Persistencia tras retiro | ✅ PASS | ⏳ Pendiente |
---

### Resultado de preparación local

Los scripts de integridad fueron ejecutados previamente contra el entorno
local para verificar su funcionamiento antes de utilizarlos sobre el
despliegue de la Entrega 3.

Las pruebas locales verificaron:

- autenticación y conservación de identidad;
- persistencia de respuestas de quiz;
- idempotencia del submit concurrente;
- consistencia del acceso a recursos;
- persistencia de datos entre las fases pre y post retiro.

La validación definitiva requiere múltiples instancias de API detrás del
balanceador y se ejecutará con `REQUIRE_TWO_INSTANCES=true`.

## Dependencias para ejecución final

La preparación de estas pruebas puede realizarse localmente, pero la validación
final requiere:

- balanceador HTTPS de la Entrega 3;
- al menos dos instancias simultáneas en `mooc-api-mig`;
- acceso a `X-Instance-Id`;
- entorno Cloud SQL compartido;
- Redis compartido;
- frontend/recursos necesarios según la prueba.

La ejecución definitiva debe realizarse sobre el commit congelado de la
Entrega 3.