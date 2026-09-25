# Estimación de costos

## Contexto

Proveedor: Google Cloud Platform  
Proyecto: proyecto1-entrega2-desarrollo  
Región: us-central1  
Fecha de estimación: 2026-09-23

La estimación corresponde al despliegue básico de la Entrega 2 y se actualizará
con la configuración efectiva una vez estén creados todos los recursos.

## Recursos considerados

| Recurso | Servicio GCP | Configuración | Horas estimadas | Costo estimado |
|---|---|---|---:|---:|
| Web Server | Compute Engine | Pendiente | Pendiente | Pendiente |
| Worker Server | Compute Engine | Pendiente | Pendiente | Pendiente |
| Base de datos | Cloud SQL PostgreSQL | Pendiente | Pendiente | Pendiente |
| Disco Web | Persistent Disk | 30 GiB | Pendiente | Pendiente |
| Disco Worker | Persistent Disk | 30 GiB | Pendiente | Pendiente |
| Multimedia | Cloud Storage | Pendiente | Pendiente | Pendiente |
| IP externa | Static External IP | Pendiente | Pendiente | Pendiente |
| Transferencia | Network Egress | Según pruebas | Pendiente | Pendiente |
| Web Server | Compute Engine | e2-small, 30 GiB | Pendiente | Pendiente |
| Worker Server | Compute Engine | e2-small inicialmente, 30 GiB | Pendiente | Pendiente |

## Supuestos

- Región: us-central1.
- La infraestructura se mantiene encendida únicamente durante desarrollo,
  pruebas y sustentación.
- No se utiliza alta disponibilidad.
- No se utilizan balanceadores ni autoscaling.
- Cloud SQL utiliza una sola zona.
- Los costos observados se contrastarán posteriormente con el panel de Billing.

## Presupuesto y alertas

Estado: pendiente de confirmar permisos de Billing.

Umbrales previstos:

- 50 %
- 75 %
- 90 %

## Costo observado

Pendiente después de las pruebas de capacidad.