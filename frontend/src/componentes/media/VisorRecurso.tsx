// Visor de recursos multimedia del estudiante: video y audio por HLS, PDF y
// presentaciones (convertidas a PDF), imágenes y archivos para descargar.
//
// Primero consulta el estado del archivo (/media): si el derivado todavía no
// existe (HLS o PDF convertido), muestra "Procesando" y no ofrece el archivo.
// Luego pide la URL de lectura (/playback), prefirmada y directa al bucket:
// el archivo no pasa por la API.
import { useEffect, useRef, useState } from 'react'
import { apiMedia, type EstadoMedia } from '../../api/media'
import { ErrorAPI } from '../../api/cliente'
import { mensaje, type TipoRecurso } from '../../api/cursos'
import { Cargando, ErrorEstado, Procesando, Vacio } from '../estados/Estados'
import './media.css'

export type RecursoVisible = { id: string; title: string; type: TipoRecurso }

type Estado =
  | { fase: 'cargando' }
  | { fase: 'sin-archivo' }
  | { fase: 'procesando' }
  | { fase: 'error'; mensaje: string }
  | { fase: 'listo'; url: string }

/** El derivado que se muestra al estudiante ya existe. */
export function derivadoListo(tipo: TipoRecurso, media: EstadoMedia): boolean {
  if (tipo === 'video' || tipo === 'audio') return Boolean(media.hls_manifest_key)
  if (tipo === 'presentation') return Boolean(media.converted_pdf_key)
  return Boolean(media.original_storage_key)
}

export function VisorRecurso({ recurso }: { recurso: RecursoVisible }) {
  const [estado, setEstado] = useState<Estado>({ fase: 'cargando' })
  const [intento, setIntento] = useState(0)

  useEffect(() => {
    let vigente = true
    setEstado({ fase: 'cargando' })
    ;(async () => {
      try {
        const media = await apiMedia.estado(recurso.id)
        if (!derivadoListo(recurso.type, media)) {
          if (vigente) setEstado({ fase: 'procesando' })
          return
        }
        const { url } = await apiMedia.reproduccion(recurso.id)
        if (vigente) setEstado({ fase: 'listo', url })
      } catch (e) {
        if (!vigente) return
        if (e instanceof ErrorAPI && e.status === 404) setEstado({ fase: 'sin-archivo' })
        else setEstado({ fase: 'error', mensaje: mensaje(e, 'No pudimos cargar el archivo de este recurso.') })
      }
    })()
    return () => { vigente = false }
  }, [recurso.id, recurso.type, intento])

  switch (estado.fase) {
    case 'cargando':
      return <Cargando mensaje="Cargando el archivo…" />
    case 'sin-archivo':
      return <Vacio titulo="Este recurso todavía no tiene archivo." />
    case 'procesando':
      return (
        <div className="visor-procesando">
          <Procesando mensaje="El archivo se está procesando. Aún no está disponible." />
          <button type="button" className="boton-secundario" onClick={() => setIntento(n => n + 1)}>Volver a consultar</button>
        </div>
      )
    case 'error':
      return <ErrorEstado mensaje={estado.mensaje} reintentar={() => setIntento(n => n + 1)} />
    case 'listo':
      return <Contenido recurso={recurso} url={estado.url} />
  }
}

function Contenido({ recurso, url }: { recurso: RecursoVisible; url: string }) {
  switch (recurso.type) {
    case 'video':
      return <ReproductorHLS url={url} titulo={recurso.title} audio={false} />
    case 'audio':
      return <ReproductorHLS url={url} titulo={recurso.title} audio />
    case 'pdf':
    case 'presentation':
      return (
        <div className="visor-documento">
          <iframe src={url} title={recurso.title} className="visor-pdf" />
          <a href={url} target="_blank" rel="noopener noreferrer" className="boton-secundario">Abrir el documento en otra pestaña ↗</a>
        </div>
      )
    case 'image':
      return <img src={url} alt={recurso.title} className="visor-imagen" />
    default:
      return (
        <a href={url} target="_blank" rel="noopener noreferrer" className="boton-secundario">Descargar el archivo ↓</a>
      )
  }
}

// HLS con hls.js (Chrome, Firefox, Edge). Safari reproduce HLS de forma nativa.
// hls.js se carga solo cuando hace falta, para no engordar el resto de la app.
// Los segmentos se piden al bucket con rutas relativas al manifiesto.
function ReproductorHLS({ url, titulo, audio }: { url: string; titulo: string; audio: boolean }) {
  const medio = useRef<HTMLVideoElement & HTMLAudioElement>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const elemento = medio.current
    if (!elemento) return
    let destruir: (() => void) | undefined
    let vigente = true
    setError('')

    if (elemento.canPlayType('application/vnd.apple.mpegurl')) {
      elemento.src = url
    } else {
      import('hls.js').then(({ default: Hls }) => {
        if (!vigente) return
        if (!Hls.isSupported()) {
          setError('Este navegador no puede reproducir el contenido. Prueba con Chrome, Firefox, Edge o Safari.')
          return
        }
        const hls = new Hls()
        hls.on(Hls.Events.ERROR, (_evento, datos) => {
          if (datos.fatal) setError('No pudimos reproducir el contenido. Recarga la página para intentarlo de nuevo.')
        })
        hls.loadSource(url)
        hls.attachMedia(elemento)
        destruir = () => hls.destroy()
      }).catch(() => { if (vigente) setError('No pudimos cargar el reproductor.') })
    }
    return () => {
      vigente = false
      destruir?.()
      elemento.removeAttribute('src')
    }
  }, [url])

  return (
    <div className="visor-medio">
      {audio
        ? <audio ref={medio} controls preload="metadata" aria-label={titulo} />
        : <video ref={medio} controls preload="metadata" playsInline aria-label={titulo} className="visor-video" />}
      {error && <ErrorEstado mensaje={error} />}
    </div>
  )
}
