# Evidencias de integridad entre réplicas - Entrega 3

## I1 - Sesión y balanceo entre réplicas
Se verificó que las solicitudes fueran atendidas por múltiples instancias del MIG de API.

## I2 - Consistencia de escritura y lectura
Una respuesta escrita en un intento permaneció disponible en lecturas posteriores atendidas por otra réplica.

## I3 - Idempotencia de submit
Cuatro submits concurrentes retornaron el mismo submitted_at y score, sin doble calificación ni divergencia de estado.

## I4 - Recurso consistente entre réplicas
El recurso 7acf229f-245f-40bd-ba96-0a49a0c2c52c resolvió consistentemente al quiz c923aada-da26-4707-9d36-12ee4e956018 desde múltiples réplicas.

## I5 - Persistencia después del retiro de una API
El attempt 4b90a974-50d5-49eb-be78-5a68b0b9b98d fue creado antes de retirar mooc-api-mn09 y continuó disponible desde mooc-api-w28n después del retiro.

Resultado general:
- Integridad preservada entre réplicas.
- Estado persistente independiente de las VMs de API.
- Sin pérdida ni duplicación observada en las pruebas.
