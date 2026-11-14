# Escalamiento y pruebas de carga – Entrega 4

## Objetivo

Validar el comportamiento de los servicios desplegados en Cloud Run bajo carga controlada, comprobar que los límites de escalamiento definidos en Terraform se respetan y medir los objetivos de desempeño propuestos para la Entrega 4.

La arquitectura establece un máximo de 3 instancias por servicio. El backend y el frontend escalan según solicitudes HTTP, mientras que el worker procesa una tarea por instancia y escala según la carga proveniente de Pub/Sub.

## Configuración del backend

Servicio: `mooc-backend`

- Región: `us-central1`
- Mínimo de instancias: 0
- Máximo de instancias: 3
- CPU: 1 vCPU
- Memoria: 1 GiB
- Concurrencia: 40 solicitudes por instancia
- Timeout: 60 s
- Acceso: público
- Base de datos: Cloud SQL PostgreSQL mediante VPC privada
- Máximo de conexiones a PostgreSQL por instancia: `DB_MAX_CONNS=5`

## Objetivos medibles

Los objetivos iniciales definidos para la E4 son:

- Creación de una solicitud de carga: p95 menor a 2 segundos, sin incluir la transferencia del archivo.
- PDF procesado: 95 % listo en máximo 2 minutos desde que el original queda almacenado.
- Video corto de hasta 1 minuto: 95 % con HLS listo en máximo 5 minutos.
- Una ráfaga de trabajos debe ser absorbida sin superar 3 workers y sin dejar mensajes en el tópico de fallidos.

## Baseline del backend

### Prueba 1 – Catálogo, carga baja

Se ejecutó una prueba controlada contra el endpoint público del catálogo.

Configuración:

- Tasa base: 1 solicitud/s.
- Tasa pico: 2 solicitudes/s.
- Operación: `GET /api/v1/catalog`.
- Duración aproximada: 7 minutos.

Resultados:

| Métrica | Resultado |
|---|---:|
| Solicitudes HTTP | 570 |
| Errores HTTP | 0 % |
| Checks exitosos | 100 % |
| Promedio | 58.56 ms |
| Mediana | 44.70 ms |
| p95 | 50.73 ms |
| p99 | 137.28 ms |
| Máximo | 5.82 s |

La prueba no presentó errores. La observación máxima de 5.82 s fue aislada y no afectó el comportamiento del 95 % de las solicitudes.

### Prueba 2 – Catálogo, mayor tasa

Se aumentó la tasa máxima manteniendo únicamente solicitudes al catálogo.

Configuración:

- Tasa base: 1 solicitud/s.
- Tasa pico: 5 solicitudes/s.
- Operación: `GET /api/v1/catalog`.
- Duración aproximada: 7 minutos.

Resultados:

| Métrica | Resultado |
|---|---:|
| Solicitudes HTTP | 1110 |
| Errores HTTP | 0 % |
| Checks exitosos | 100 % |
| Promedio | 45.78 ms |
| Mediana | 44.61 ms |
| p95 | 49.75 ms |
| p99 | 58.41 ms |
| Máximo | 228.73 ms |

El aumento de la tasa no degradó significativamente el p95. El backend mantuvo 0 % de errores.

### Prueba 3 – Catálogo + autenticación

Se mezclaron consultas públicas de catálogo con operaciones de inicio de sesión. El 25 % de las operaciones seleccionadas por el escenario correspondieron a login, con el objetivo de incluir una operación más intensiva en CPU debido a la verificación de contraseña.

Configuración:

- Tasa base: 1 solicitud/s.
- Tasa pico: 5 solicitudes/s.
- Proporción configurada de login: 25 %.
- Operaciones: catálogo y `POST /api/v1/auth/login`.
- Duración aproximada: 7 minutos.

Resultados:

| Métrica | Resultado |
|---|---:|
| Solicitudes HTTP | 1111 |
| Errores HTTP | 0 % |
| Checks exitosos | 100 % |
| Promedio | 64.60 ms |
| Mediana | 45.70 ms |
| p95 | 131.44 ms |
| p99 | 141.10 ms |
| Máximo | 488.33 ms |

La incorporación de operaciones de autenticación elevó el p95 desde aproximadamente 50 ms hasta 131 ms, pero no produjo errores HTTP. El backend mantuvo un comportamiento estable durante toda la corrida.

## Comparación inicial

| Prueba | Carga | Operaciones | Requests | Errores | p95 |
|---|---|---|---:|---:|---:|
| Baseline 1 | 1 → 2 req/s | Catálogo | 570 | 0 % | 50.73 ms |
| Baseline 2 | 1 → 5 req/s | Catálogo | 1110 | 0 % | 49.75 ms |
| Catálogo + login | 1 → 5 req/s | Catálogo + autenticación | 1111 | 0 % | 131.44 ms |

Estos resultados constituyen la línea base del servicio backend antes de las pruebas de escalamiento.

## Evidencias del baseline

Los resultados crudos de k6 se almacenan en:

`k6/results/e4/`

Archivos disponibles:

- `backend-catalog-1to5-20261109-161136.log`
- `backend-login-20261109-162109.log`
- `backend-login-20261109-162109.json`

## Prueba fuerte de escalamiento del backend

Se ejecutó una prueba de carga controlada contra `mooc-backend`, incrementando la tasa desde 20 hasta 120 iteraciones por segundo e incluyendo operaciones de catálogo y autenticación.

### Resultados

- Solicitudes HTTP: 31.923
- Checks exitosos: 100 %
- Errores HTTP: 0 %
- Latencia promedio: 1,65 s
- p95: 4,31 s
- p99: 5,46 s
- Latencia máxima: 8,45 s
- VUs máximos utilizados por k6: 400
- Iteraciones descartadas: 1.077

Durante la prueba, Cloud Run escaló hasta **3 instancias activas**, alcanzando el máximo configurado para el servicio sin superarlo. Al finalizar la carga, las instancias dejaron de estar activas y quedaron en estado `idle`.

La prueba demuestra que el límite `maxScale = 3` se respeta bajo carga y que el backend puede escalar horizontalmente sin producir errores HTTP durante el escenario probado.

Sin embargo, la prueba de mayor carga también evidencia que el backend alcanzó su límite de capacidad con la configuración actual. El p95 aumentó hasta 4,31 s y k6 descartó 1.077 iteraciones, lo que indica que la tasa de llegada superó la capacidad que podían sostener las 3 instancias disponibles. Una causa probable es la combinación de hasta 40 solicitudes concurrentes por instancia con un pool de solo 5 conexiones a PostgreSQL por instancia (`DB_MAX_CONNS=5`), lo que puede generar espera por una conexión cuando aumenta la concurrencia. Además, las operaciones de autenticación utilizan bcrypt para verificar contraseñas, una tarea intensiva en CPU, mientras cada instancia dispone de 1 vCPU. Por lo tanto, bajo esta carga, tanto la contención del pool de conexiones como el costo computacional del login pueden contribuir al aumento de latencia. Un eventual aumento de capacidad debería evaluarse junto con el tamaño del pool de conexiones y los recursos de CPU, y no únicamente incrementando el número de instancias.

### Evidencias

- `k6/results/e4/backend-scaling-20261109.log`
- `k6/results/e4/backend-scaling-20261109.json`
- `k6/results/e4/backend-cloudrun-config.txt`
- `k6/results/e4/backend-iam.txt`
- `docs/entrega4/evidencias/backend-instance-count-20261109.png`

## Prueba de escalamiento del frontend

Se ejecutó una prueba de carga controlada contra `mooc-frontend` con una tasa creciente de solicitudes sobre la SPA.

### Resultados

- Solicitudes HTTP: 92.999
- Checks exitosos: 100 %
- Errores HTTP: 0 %
- Latencia promedio: 46,19 ms
- p90: 51,88 ms
- p95: 56,99 ms
- Latencia máxima: 1,09 s
- VUs máximos utilizados: 33

Durante la prueba, Cloud Run escaló de 1 a **2 instancias activas**. El servicio mantuvo el 100 % de respuestas exitosas y no superó el límite configurado de 3 instancias.

Al finalizar la carga, las instancias activas descendieron a 0 y quedaron instancias en estado `idle`.

### Evidencias

- `k6/results/e4/frontend-scaling-20261109.log`
- `k6/results/e4/frontend-cloudrun-config.txt`
- `k6/results/e4/frontend-iam.txt`
- `docs/entrega4/evidencias/frontend-instance-count-20261109.png`