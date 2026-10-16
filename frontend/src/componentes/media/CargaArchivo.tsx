// Carga de archivos de un recurso multimedia (video, audio, imagen, PDF,
// presentación o archivo). Sube directo al bucket en partes, puede pausarse y
// reanudarse (también tras recargar la página, si se elige el mismo archivo)
// y, al terminar, espera a que el worker procese el archivo.
import { useEffect, useId, useRef, useState } from 'react'
import { ErrorAPI } from '../../api/cliente'
import type { EstadoProcesamiento } from '../../api/cursos'
import {
  PARTE_POR_DEFECTO, aceptaPorTipo, apiMedia, formatoTamano, guardarCarga, huella, leerCarga,
  olvidarCarga, partesPendientes, sha256Hex, subirParte,
} from '../../api/media'
import './media.css'

type Fase = 'inactivo' | 'preparando' | 'subiendo' | 'confirmando' | 'pausado' | 'error' | 'enviado'

const PARALELO = 3
const ESPERA_MS = 10_000
const MAX_CONSULTAS = 36 // unos 6 minutos

export function CargaArchivo({ resourceId, tipo = 'file', estado, editable = true, alCambiar }: {
  resourceId: string
  tipo?: string
  estado?: EstadoProcesamiento
  editable?: boolean
  alCambiar: () => void
}) {
  const id = useId()
  const [archivo, setArchivo] = useState<File | null>(null)
  const [fase, setFase] = useState<Fase>('inactivo')
  const [subidos, setSubidos] = useState(0)
  const [error, setError] = useState('')
  const [esperando, setEsperando] = useState(false)
  const control = useRef<AbortController | null>(null)
  const montado = useRef(true)
  // alCambiar puede cambiar en cada render del editor; se usa por referencia
  // para que la espera no se reinicie con cada recarga.
  const recargar = useRef(alCambiar)
  recargar.current = alCambiar
  const consultas = useRef(0)

  // En modo estricto React monta, desmonta y vuelve a montar: por eso se
  // marca montado en el cuerpo del efecto y no solo en el valor inicial.
  useEffect(() => {
    montado.current = true
    return () => { montado.current = false; control.current?.abort() }
  }, [])

  // Tras confirmar la carga, el worker cambia el estado del recurso
  // (pending -> processing -> ready/failed). Se recarga el editor cada 10 s
  // hasta que termine o se agote la espera.
  useEffect(() => {
    if (!esperando) return
    if (estado === 'ready' || estado === 'failed') { setEsperando(false); return }
    const t = setInterval(() => {
      consultas.current += 1
      if (consultas.current > MAX_CONSULTAS) { clearInterval(t); setEsperando(false); return }
      recargar.current()
    }, ESPERA_MS)
    return () => clearInterval(t)
  }, [esperando, estado])

  const guardada = leerCarga(resourceId)
  const reanudable = !!(archivo && guardada && guardada.huella === huella(archivo))
  const ocupado = fase === 'preparando' || fase === 'subiendo' || fase === 'confirmando'

  async function subir() {
    if (!archivo) return
    const ctrl = new AbortController()
    control.current = ctrl
    setError('')
    const avance = new Map<number, number>()
    let base = 0
    const pintar = () => { if (montado.current) setSubidos(base + [...avance.values()].reduce((a, b) => a + b, 0)) }

    try {
      // 1. Reusar la sesión anterior si es el mismo archivo y sigue activa.
      let sesionId = ''
      let tamanoParte = PARTE_POR_DEFECTO
      let listas: number[] = []
      const previa = leerCarga(resourceId)
      if (previa && previa.huella === huella(archivo)) {
        try {
          const r = await apiMedia.partes(previa.sesionId)
          sesionId = previa.sesionId
          tamanoParte = previa.tamanoParte
          listas = (r.parts || []).map(p => p.part_number)
          base = (r.parts || []).reduce((a, p) => a + p.size_bytes, 0)
        } catch (e) {
          if (!(e instanceof ErrorAPI) || e.status === 0) throw e
          olvidarCarga(resourceId) // vencida o ya cerrada: se empieza de nuevo
        }
      }

      // 2. Si no, abrir una sesión nueva.
      if (!sesionId) {
        setFase('preparando')
        const checksum = await sha256Hex(archivo)
        if (ctrl.signal.aborted) throw new DOMException('Carga pausada', 'AbortError')
        const r = await apiMedia.iniciar(resourceId, {
          mime_type: archivo.type || 'application/octet-stream',
          size_bytes: archivo.size,
          ...(checksum ? { checksum_sha256: checksum } : {}),
        })
        sesionId = r.upload_session.id
        tamanoParte = r.recommended_part_bytes || PARTE_POR_DEFECTO
        guardarCarga(resourceId, { sesionId, huella: huella(archivo), tamanoParte })
      }

      // 3. Subir las partes que faltan, de a PARALELO a la vez.
      setFase('subiendo')
      pintar()
      const cola = partesPendientes(archivo.size, tamanoParte, listas)
      const trabajador = async () => {
        for (let n = cola.shift(); n !== undefined; n = cola.shift()) {
          const parte = n
          const { url } = await apiMedia.urlParte(sesionId, parte)
          const trozo = archivo.slice((parte - 1) * tamanoParte, Math.min(parte * tamanoParte, archivo.size))
          await subirParte(url, trozo, b => { avance.set(parte, b); pintar() }, ctrl.signal)
          avance.set(parte, trozo.size)
          pintar()
        }
      }
      await Promise.all(Array.from({ length: PARALELO }, trabajador))

      // 4. Confirmar: la API cierra el multipart y encola el procesamiento.
      setFase('confirmando')
      await apiMedia.completar(sesionId)
      olvidarCarga(resourceId)
      if (!montado.current) return
      setFase('enviado')
      setArchivo(null)
      consultas.current = 0
      setEsperando(true)
      alCambiar()
    } catch (e) {
      if (!montado.current) return
      if (e instanceof DOMException && e.name === 'AbortError') { setFase('pausado'); return }
      setFase('error')
      setError(e instanceof Error && e.message ? e.message : 'No se pudo subir el archivo.')
    } finally {
      if (control.current === ctrl) control.current = null
    }
  }

  const porcentaje = archivo && archivo.size > 0 ? Math.min(100, Math.round((subidos / archivo.size) * 100)) : 0
  const yaTieneArchivo = estado === 'ready' || estado === 'failed' || estado === 'processing'

  return <div className="carga-archivo">
    {esperando && estado !== 'processing' && estado !== 'ready' && estado !== 'failed' &&
      <p className="ranura-nota" role="status">Archivo recibido. Está en cola para procesarse; el estado se actualiza solo.</p>}

    {editable && !ocupado && <div className="carga-elegir">
      <label htmlFor={`${id}-archivo`}>{yaTieneArchivo ? 'Reemplazar archivo' : 'Archivo'}</label>
      <input id={`${id}-archivo`} type="file" accept={aceptaPorTipo[tipo] ?? ''}
        onChange={e => { setArchivo(e.target.files?.[0] || null); setFase('inactivo'); setSubidos(0); setError('') }} />
    </div>}

    {archivo && <p className="ranura-nota">{archivo.name} · {formatoTamano(archivo.size)}
      {reanudable && fase !== 'subiendo' && ' · se continuará donde quedó la carga anterior'}</p>}

    {(fase === 'subiendo' || fase === 'confirmando' || fase === 'pausado' || (fase === 'error' && subidos > 0)) && archivo &&
      <div className="carga-avance">
        <progress max={100} value={porcentaje} aria-label="Avance de la carga">{porcentaje}%</progress>
        <span>{porcentaje}% · {formatoTamano(subidos)} de {formatoTamano(archivo.size)}</span>
      </div>}

    {fase === 'preparando' && <p className="ranura-nota" role="status">Preparando la carga…</p>}
    {fase === 'confirmando' && <p className="ranura-nota" role="status">Confirmando la carga…</p>}
    {fase === 'pausado' && <p className="ranura-nota" role="status">Carga pausada. Puedes continuarla ahora o más tarde eligiendo el mismo archivo.</p>}
    {fase === 'error' && <div className="aviso aviso-error" role="alert"><p>{error}</p></div>}

    {editable && <div className="acciones">
      {archivo && !ocupado && <button className="boton-secundario" type="button" onClick={subir}>
        {fase === 'pausado' || fase === 'error' || reanudable ? 'Continuar carga' : 'Subir archivo'}
      </button>}
      {fase === 'subiendo' && <button className="boton-texto" type="button" onClick={() => control.current?.abort()}>Pausar</button>}
    </div>}
  </div>
}
