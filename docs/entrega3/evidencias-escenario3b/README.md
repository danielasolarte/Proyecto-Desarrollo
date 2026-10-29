# Escenario 3B - Elasticidad de Workers

## Objetivo

Validar que el MIG de workers responda correctamente ante carga multimedia, incluyendo escalamiento automático, retiro seguro de una instancia con trabajo activo, recuperación del procesamiento y reducción automática de capacidad cuando desaparece la demanda.

## Evidencia observada

1. Durante la carga peak, la cola multimedia presentó trabajos pendientes y activos.
2. El MIG incrementó automáticamente la cantidad de workers.
3. Se observaron múltiples trabajos activos simultáneamente, confirmando procesamiento concurrente.
4. Durante la carga se solicitó el retiro controlado de mooc-worker-zp1h.
5. La instancia pasó a estado DELETING mientras todavía existía trabajo activo.
6. El MIG creó automáticamente una instancia de reemplazo (mooc-worker-pdxt).
7. La cola continuó procesándose hasta alcanzar:
   - pendientes = 0
   - activos = 0
8. No fue necesario intervenir manualmente para recuperar el procesamiento.
9. Una vez eliminada la demanda, el autoscaler redujo automáticamente la cantidad de workers hasta volver a la capacidad mínima.

## Resultado

El escenario fue satisfactorio.

Se comprobó:

- Scale-up automático ante aumento de demanda.
- Procesamiento concurrente en múltiples workers.
- Retiro controlado de una instancia durante trabajo activo.
- Conservación de capacidad mediante reemplazo automático.
- Drenaje completo de la cola.
- Scale-down automático cuando la demanda regresó a cero.

Esto demuestra que el procesamiento multimedia puede adaptarse dinámicamente a la carga sin perder disponibilidad del servicio.
