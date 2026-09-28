# Pruebas de carga k6 — Plataforma MOOC completa

Este directorio contiene las pruebas de carga transversales de la API de la
plataforma MOOC.

El objetivo no es probar únicamente un módulo individual, sino generar carga
concurrente sobre varios flujos representativos de la plataforma y observar su
comportamiento funcional y de rendimiento.

## Estructura

```text
k6/
├── full-project.js        # Script principal de k6
├── run-smoke.ps1          # Prueba rápida de validación
├── run-capacity.ps1       # Pruebas escalonadas de capacidad
├── run-acceptance.ps1     # Perfil de aceptación / alta concurrencia
├── README.md              # Documentación de las pruebas
└── results/
    ├── .gitkeep
    ├── smoke-summary.json
    ├── capacity-100.json
    ├── capacity-250.json
    ├── capacity-500.json
    ├── capacity-1000.json
    └── capacity-2000.json
```

`full-project.js` contiene la lógica principal de las pruebas, los escenarios,
los datos sintéticos y los thresholds.

Los archivos `.ps1` son scripts de PowerShell que simplifican la ejecución de
k6 desde Docker.

Los archivos `.json` dentro de `results/` contienen la evidencia exportada por
k6 de las diferentes corridas.

---

## Qué cubre

El script `full-project.js` genera datos sintéticos en `setup()` y posteriormente
ejecuta carga concurrente sobre los siguientes módulos:

- identidad y login;
- administración y auditoría;
- cursos y autoría;
- editor Markdown;
- catálogo e inscripciones;
- quizzes;
- creación de intentos;
- guardado de respuestas;
- submit de quiz;
- verificación de idempotencia;
- progreso y aprobación;
- insignias;
- multimedia;
- inicio de carga multipart;
- URLs prefirmadas;
- listado de partes;
- playback multimedia opcional.

La prueba no intenta transcodificar miles de videos simultáneamente.

El procesamiento HLS, reintentos, DLQ y comportamiento interno de los workers
se verifican mediante pruebas E2E y operativas independientes.

k6 se utiliza principalmente para generar carga sobre la API HTTP y sus
dependencias compartidas, incluyendo PostgreSQL, Redis y MinIO.

---

# Ejecución con Docker

Las pruebas se ejecutan utilizando la imagen oficial de k6:

```text
grafana/k6
```

Por lo tanto, no es necesario instalar k6 directamente en Windows.

La API debe estar disponible localmente en:

```text
http://localhost:8080
```

Como k6 se ejecuta dentro de Docker, se utiliza:

```text
http://host.docker.internal:8080/api/v1
```

para acceder a la API que corre en el host.

---

# Scripts de ejecución

## `run-smoke.ps1`

Ejecuta una prueba rápida con una carga pequeña.

Su objetivo es comprobar que:

- Docker puede ejecutar k6;
- k6 puede comunicarse con la API;
- el `setup()` funciona;
- los escenarios principales pueden ejecutarse;
- las variables y credenciales son válidas;
- los endpoints responden antes de ejecutar pruebas más pesadas.

Se ejecuta desde la raíz del proyecto:

```powershell
.\k6\run-smoke.ps1
```

El resultado se guarda en:

```text
k6/results/smoke-summary.json
```

---

## `run-capacity.ps1`

Ejecuta automáticamente las pruebas escalonadas de capacidad utilizadas para la
evaluación del proyecto.

Las corridas se realizan de forma independiente con:

```text
100 VUs
250 VUs
500 VUs
1000 VUs
2000 VUs
```

Entre las corridas se deja un pequeño intervalo para reducir interferencias
entre pruebas consecutivas.

Se ejecuta con:

```powershell
.\k6\run-capacity.ps1
```

Los resultados se almacenan en:

```text
k6/results/capacity-100.json
k6/results/capacity-250.json
k6/results/capacity-500.json
k6/results/capacity-1000.json
k6/results/capacity-2000.json
```

Esto permite comparar para cada nivel de carga:

- porcentaje de checks correctos;
- tasa de errores HTTP;
- cantidad de requests;
- latencia global;
- p95;
- comportamiento por módulo.

Para evitar conflictos artificiales, los VUs asociados al flujo de estudiantes
utilizan identidades independientes durante las pruebas de capacidad.

---

## `run-acceptance.ps1`

Ejecuta el perfil de alta concurrencia definido para evaluar el sistema cerca
del objetivo máximo inicial de 2000 usuarios concurrentes.

Se ejecuta con:

```powershell
.\k6\run-acceptance.ps1
```

Este perfil permite realizar una corrida de aceptación independiente de las
pruebas escalonadas.

La prueba de capacidad es la principal evidencia utilizada actualmente porque
permite identificar progresivamente en qué nivel comienza la degradación.

---

# Perfiles disponibles

## `smoke`

Prueba rápida de validación.

Utiliza pocos usuarios virtuales y sirve para detectar errores de configuración
antes de ejecutar cargas mayores.

Ejemplo manual con Docker:

```powershell
docker run --rm `
  -v "${PWD}:/project" `
  -w /project `
  grafana/k6 run `
  -e PROFILE=smoke `
  -e BASE_URL=http://host.docker.internal:8080/api/v1 `
  k6/full-project.js
```

---

## `local`

Perfil de aproximadamente 100 VUs utilizado durante el desarrollo local.

Ejemplo:

```powershell
docker run --rm `
  -v "${PWD}:/project" `
  -w /project `
  grafana/k6 run `
  -e PROFILE=local `
  -e BASE_URL=http://host.docker.internal:8080/api/v1 `
  k6/full-project.js
```

---

## `capacity`

Perfil configurable utilizado por `run-capacity.ps1`.

El número total de usuarios virtuales se especifica mediante:

```text
TOTAL_VUS
```

Ejemplo para 500 VUs:

```powershell
docker run --rm `
  -v "${PWD}:/project" `
  -w /project `
  grafana/k6 run `
  -e PROFILE=capacity `
  -e TOTAL_VUS=500 `
  -e BASE_URL=http://host.docker.internal:8080/api/v1 `
  --summary-export /project/k6/results/capacity-500.json `
  k6/full-project.js
```

---

## `acceptance`

Perfil de máxima concurrencia definido para aproximarse al objetivo inicial del
proyecto de 2000 usuarios concurrentes.

La prueba escalonada `capacity` permite analizar mejor el punto de degradación y
por eso fue utilizada como evidencia principal.

---

# Base de datos

Para obtener una corrida completamente reproducible se recomienda iniciar desde
un entorno limpio:

```powershell
docker compose down -v
docker compose up -d
```

Aplicar las migraciones:

```powershell
migrate -database "postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable" -path migrations up
```

Iniciar la API:

```powershell
go run ./cmd/api
```

Después puede ejecutarse:

```powershell
.\k6\run-smoke.ps1
```

y posteriormente:

```powershell
.\k6\run-capacity.ps1
```

---

# Credenciales administrativas

El `setup()` necesita permisos administrativos para preparar los datos
sintéticos utilizados por la prueba.

Si existe un administrador diferente al configurado por defecto, pueden
proporcionarse las credenciales mediante variables de entorno.

Ejemplo:

```powershell
docker run --rm `
  -v "${PWD}:/project" `
  -w /project `
  grafana/k6 run `
  -e PROFILE=smoke `
  -e BASE_URL=http://host.docker.internal:8080/api/v1 `
  -e ADMIN_EMAIL=admin@example.com `
  -e ADMIN_PASSWORD=MiClave `
  k6/full-project.js
```

---

# Multimedia procesada opcional

El escenario multimedia siempre prueba operaciones de autoría como:

- iniciar un upload multipart;
- obtener una URL prefirmada;
- listar las partes del upload.

Para incluir también operaciones sobre un recurso multimedia ya procesado puede
proporcionarse:

```text
PLAYBACK_RESOURCE_ID
```

El recurso debe encontrarse en estado listo para reproducción.

---

# Verificación pública de insignias opcional

La verificación pública de insignias puede incluirse en la carga si se
proporciona un código válido.

Puede obtenerse desde PostgreSQL:

```sql
SELECT verification_code
FROM badge_issuances
WHERE revoked_at IS NULL
LIMIT 1;
```

y proporcionarse mediante:

```text
VERIFICATION_CODE
```

---

# Thresholds

Los thresholds iniciales definidos en `full-project.js` permiten detectar
degradación de rendimiento.

Los objetivos documentados inicialmente fueron:

| Área | p95 máximo |
|---|---:|
| Catálogo | 500 ms |
| Cursos / autoría | 700 ms |
| Administración | 800 ms |
| Quizzes | 1000 ms |
| Progreso | 1000 ms |
| API global | 1000 ms |
| Auth / login | 1500 ms |
| Multimedia | 1500 ms |

Adicionalmente:

```text
http_req_failed < 1%
checks > 99%
```

Estos thresholds son objetivos internos de la prueba y no deben modificarse
únicamente para hacer que una corrida aparezca como exitosa.

Los resultados reales se conservan incluso cuando un threshold es superado.

---

# Resultados obtenidos

Se realizaron pruebas independientes de capacidad con 100, 250, 500, 1000 y
2000 VUs.

| VUs | Requests | Checks correctos | Error HTTP | p95 global |
|---:|---:|---:|---:|---:|
| 100 | 11,990 | 100.00% | 0.000% | 1.29 s |
| 250 | 3,991 | 97.54% | 2.36% | 57.42 s |
| 500 | 6,406 | 99.95% | 0.047% | 10.70 s |
| 1000 | 8,156 | 99.93% | 0.061% | 21.48 s |
| 2000 | 5,211 | 74.41% | 26.60% | 44.40 s |

## Interpretación

### 100 VUs

El sistema permaneció estable funcionalmente:

- 100% de checks correctos;
- 0% de errores HTTP;
- p95 global aproximado de 1.29 segundos.

Fue la carga con mejor comportamiento observado.

### 250 VUs

Se presentó una degradación atípicamente alta:

- 97.54% de checks correctos;
- 2.36% de errores HTTP;
- p95 global aproximado de 57.42 segundos.

Este resultado no sigue una progresión monotónica respecto a las corridas de
500 y 1000 VUs, por lo que puede haber sido afectado por condiciones
transitorias del entorno local.

### 500 VUs

El sistema mantuvo una alta integridad funcional:

- aproximadamente 99.95% de checks correctos;
- aproximadamente 0.047% de errores HTTP.

Sin embargo, el p95 global aumentó a aproximadamente 10.70 segundos.

### 1000 VUs

La mayoría de las operaciones siguieron completándose correctamente:

- aproximadamente 99.93% de checks correctos;
- aproximadamente 0.061% de errores HTTP.

La latencia aumentó considerablemente:

```text
p95 global ≈ 21.48 s
```

### 2000 VUs

La configuración local presentó degradación crítica:

- aproximadamente 74.41% de checks correctos;
- aproximadamente 26.60% de errores HTTP;
- p95 global aproximado de 44.40 segundos.

Por lo tanto, la configuración local probada no mantiene de forma estable una
carga de 2000 usuarios concurrentes.

---

# Principal cuello de botella observado

El módulo de quizzes presentó de forma consistente las mayores latencias bajo
carga.

El flujo probado incluye:

```text
crear intento
→ guardar respuesta
→ submit
→ replay idempotente
→ actualización de progreso
→ posible emisión de insignia
```

Por tratarse de un flujo con varias operaciones de escritura y validación, su
degradación aparece antes que en algunos módulos predominantemente de lectura.

---

# Monitoreo con Docker Stats

Durante las pruebas se utilizó:

```powershell
docker stats
```

para observar el consumo de recursos de los servicios de infraestructura.

Valores aproximados observados:

| Servicio | CPU observada | Memoria aproximada |
|---|---:|---:|
| Redis | 0.5% – 10.6% | 11 – 13 MiB |
| PostgreSQL | 0% – 20% | 39 – 72 MiB |
| MinIO | 0% – 15.5% | 113 – 126 MiB |
| Mailpit | 0% – 13% | 26 – 33 MiB |
| ClamAV | 0% – 13.4% | 784 MiB – 1.04 GiB |

Redis, PostgreSQL y MinIO mantuvieron un consumo de memoria relativamente bajo.

ClamAV fue el principal consumidor de memoria del entorno, utilizando
aproximadamente entre 0.8 y 1.0 GiB.

Los snapshots observados no muestran agotamiento de memoria en PostgreSQL,
Redis o MinIO.

Por esta razón, la degradación de latencia bajo cargas altas no puede atribuirse
únicamente al consumo de memoria de estos contenedores.

También deben considerarse:

- la API Go;
- el pool de conexiones de PostgreSQL;
- consultas costosas;
- bloqueos o contención;
- cantidad de goroutines;
- límites de Docker Desktop;
- CPU y memoria disponibles en el host;
- ejecución simultánea de k6 y los servicios locales.

Los datos de `docker stats` se utilizaron como evidencia cualitativa, ya que las
muestras no fueron almacenadas con timestamps asociados a cada nivel exacto de
VUs.

---

# Evidencia

Los resultados exportados por k6 se encuentran en:

```text
k6/results/
```

Actualmente se conservan:

```text
smoke-summary.json
capacity-100.json
capacity-250.json
capacity-500.json
capacity-1000.json
capacity-2000.json
```

Estos archivos contienen las métricas completas de cada corrida y permiten
auditar los resultados reportados.

Para una entrega o demostración se recomienda conservar:

1. los archivos JSON generados por k6;
2. la salida final de consola;
3. evidencia de `docker stats`;
4. `docker compose ps`;
5. logs de la API;
6. logs de los workers;
7. evidencia del comportamiento bajo diferentes niveles de VUs.

---

# Escalamiento

k6 genera carga, pero no demuestra por sí mismo escalamiento horizontal.

La prueba de 2000 VUs permite observar cómo responde la configuración actual,
pero acreditar escalamiento requiere además mostrar que la infraestructura
puede aumentar su capacidad.

Los workers pueden escalar de forma independiente.

La API actualmente expone directamente:

```text
8080:8080
```

por lo que ejecutar varias réplicas de la API requiere introducir un reverse
proxy o load balancer, o modificar la estrategia de publicación de puertos.

Por tanto, la prueba de carga y la demostración de escalamiento son evidencias
complementarias pero diferentes.

---

# Conclusión

Las pruebas muestran que la plataforma mantiene estabilidad funcional con
cargas moderadas y conserva una tasa de errores muy baja incluso en algunas
corridas de 500 y 1000 VUs.

Sin embargo, la latencia aumenta considerablemente con la concurrencia y el
módulo de quizzes aparece como el principal candidato para optimización.

A 2000 VUs la configuración local deja de cumplir los objetivos de
disponibilidad y rendimiento, por lo que sería necesario optimizar la
aplicación y/o escalar la infraestructura para soportar de forma estable ese
nivel de concurrencia.