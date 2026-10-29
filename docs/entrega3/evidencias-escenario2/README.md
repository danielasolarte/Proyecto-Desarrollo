# Escenario 2 - Entrega 3

## Entorno
- Base URL: https://136.68.98.101.sslip.io/api/v1
- k6: v2.3.0
- Perfiles:
  - corto: 640x360 / 15 s
  - medio: 1280x720 / 30 s
  - largo: 1920x1080 / 30 s

## Resultados

| Nivel | Checks OK | Checks fallidos | HTTP fail | Ready avg | Ready p95 | HLS p95 |
|---|---:|---:|---:|---:|---:|---:|
| Baseline | 106 | 0 | 0% | 18.02 s | 33.60 s | 2.63 s |
| L1 | 312 | 1 | 0.18% | 26.89 s | 68.13 s | 1.51 s |
| L2 | 544 | 0 | 0% | 47.93 s | 143.94 s | 2.68 s |
| L3 | 1024 | 0 | 0% | 45.04 s | 100.00 s | 3.24 s |

## Observaciones

- Todos los niveles cumplieron los thresholds funcionales definidos por el escenario.
- En L1 ocurrió un único fallo de transferencia a Storage.
- upload_functional_failures en L1 fue 1.49%, por debajo del límite de 5%.
- No se observaron errores HLS relevantes.
- Durante la carga el MIG de workers escaló de 1 a 2 instancias.
- La segunda instancia fue creada automáticamente por la cuenta de servicio de Google Cloud.
- La reducción del p95 de time-to-ready entre L2 y L3 es consistente con la incorporación de capacidad adicional de procesamiento.
