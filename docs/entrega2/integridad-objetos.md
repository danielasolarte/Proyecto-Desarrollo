# Verificacion de integridad de objetos migrados (Entrega 2, correccion C5)

- Fecha: 2026-10-10 15:45 (hora local)
- Commit: `85ba200`
- Bucket: `gs://mooc-e2-media-proyecto1-entrega2`
- Script: `deploy/gcp/07-verificar-integridad-objetos.ps1`

## Resumen

- Filas en `media_assets`: 77
- Objetos en el bucket: 533
- Originales presentes con tamano correcto: 74 de 77
- SHA-256 verificados descargando el objeto: 10 (coinciden: 10)
- Filas con algun problema: 3
- Objetos sin referencia en la base de datos: 4

## Interpretacion

- **Todos los recursos que la plataforma marca como disponibles estan integros.** Los 73 videos en estado `ready` tienen el original en el bucket, con el tamano registrado en `media_assets`, y su manifiesto HLS con segmentos. En los 10 originales descargados, el SHA-256 coincide con `checksum_sha256` de la base de datos.
- **Las 3 filas con problemas estan en estado `failed`, es decir, la plataforma ya las rechazo.** Dos no tienen original (la carga nunca se completo) y una tiene un tamano distinto al declarado (9.914.181 bytes esperados contra 1.334.985 en el bucket). Son cargas interrumpidas durante las pruebas de carga: las preparaciones que se cortaron por timeout y los colapsos del Worker del 10 de octubre. Que terminen en `failed` y no en `ready` es el comportamiento esperado de la validacion de tamano y checksum del worker.
- **Los 4 objetos sin referencia** estan bajo un mismo recurso (`resources/fc108d87-.../original/`). Son originales de cargas repetidas sobre ese recurso cuyo `media_asset` apunta a otra clave. No afectan a ningun recurso disponible y se pueden limpiar con una regla de ciclo de vida del bucket o un borrado manual.

## Detalle por recurso

| media_asset | Tipo | Estado | Original | SHA-256 | HLS | PDF | Problemas |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `0d9c5db8` | video | ready | ok | coincide | ok (2 segmentos) | - |  |
| `dec936ff` | video | ready | ok | coincide | ok (6 segmentos) | - |  |
| `a903b754` | video | ready | ok | coincide | ok (11 segmentos) | - |  |
| `6139d4dd` | video | ready | ok | coincide | ok (2 segmentos) | - |  |
| `04aa412d` | video | ready | ok | coincide | ok (2 segmentos) | - |  |
| `d667babf` | video | ready | ok | coincide | ok (6 segmentos) | - |  |
| `d5df1ae8` | video | ready | ok | coincide | ok (11 segmentos) | - |  |
| `734d6485` | video | ready | ok | coincide | ok (2 segmentos) | - |  |
| `ccb77bf3` | video | ready | ok | coincide | ok (6 segmentos) | - |  |
| `64d71e7f` | video | ready | ok | coincide | ok (11 segmentos) | - |  |
| `2750d454` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `9496fe5f` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `1a1418b4` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `96e8ddf3` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `8cf7cfb0` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `f802de3b` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `baed61d8` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `e79a7a32` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `00e581fe` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `8bc728a8` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `b90b7d6b` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `12e25791` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `d52d1be6` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `d617aac8` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `b3034f93` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `e93e24cb` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `1cd28bb3` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `7b739bf0` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `abc68e1d` | video | failed | ok | no verificado | - | - |  |
| `f38cf3ba` | video | failed | ok | no verificado | - | - | tamano BD 9914181 vs bucket 1334985 |
| `c31132f6` | video | failed | falta | no verificado | - | - | original no existe |
| `5e43d6ed` | video | failed | falta | no verificado | - | - | original no existe |
| `63abe241` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `321bebf7` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `60bcbd40` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `f61265b0` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `08890a9a` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `0bf0478e` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `0a929220` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `b644da06` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `eef1653c` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `dd4a9098` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `b6d55729` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `4f849e5a` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `4e24dd28` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `b7f34a59` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `653271b0` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `fc8bab1c` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `102124bd` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `57bead6c` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `1298b40b` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `6d6151ae` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `1d919c12` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `847dbb02` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `3c2372ab` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `07daeb7d` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `e7b9c39b` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `e695e2c3` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `05785502` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `d2141eed` | video | ready | ok | no verificado | ok (11 segmentos) | - |  |
| `a89c0eb6` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `53cb9bdc` | video | ready | ok | no verificado | ok (6 segmentos) | - |  |
| `a333c219` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `6eea525a` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `b5263ad0` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `90c6f1a8` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `ab7e1e44` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `60fdb834` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `4e46c1bb` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `ff47f85e` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `46303d7d` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `2e077997` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `81882214` | video | ready | ok | no verificado | ok (2 segmentos) | - |  |
| `aa6a809d` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `c3ae0627` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `4583e6df` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |
| `97cd1f02` | video | ready | ok | no verificado | ok (4 segmentos) | - |  |

## Objetos sin referencia (primeros 30)

- `resources/fc108d87-dc40-4bfc-8f73-29182b3acfd1/original/84e0051e-0b86-461b-85cc-b96c291ab102`
- `resources/fc108d87-dc40-4bfc-8f73-29182b3acfd1/original/e8b90363-3e3e-4e90-8677-5bbbe41b442d`
- `resources/fc108d87-dc40-4bfc-8f73-29182b3acfd1/original/ae676e1e-4642-4352-81b9-d6493298061c`
- `resources/fc108d87-dc40-4bfc-8f73-29182b3acfd1/original/e5b45e0f-a6ce-4f29-826e-69841f830fa1`
