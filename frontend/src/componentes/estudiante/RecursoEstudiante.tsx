import { useEffect, useRef, useState } from 'react'
import {
  api,
  mensaje,
  nombreTipo,
  type TipoRecurso,
} from '../../api/cursos'
import { VisorRecurso } from '../media/VisorRecurso'

type RecursoEstudianteResumen = {
  id: string
  title: string
  type: TipoRecurso
  required: boolean
}

type Props = {
  recurso: RecursoEstudianteResumen
  onCompletado?: () => void
}

export function RecursoEstudiante({
  recurso,
  onCompletado,
}: Props) {
  const [abierto, setAbierto] = useState(false)
  const [completado, setCompletado] = useState(false)
  const [segundosActivos, setSegundosActivos] = useState(0)
  const [cargando, setCargando] = useState(false)
  const [error, setError] = useState('')
  const [contenido, setContenido] = useState<string | null>(null)
  const posicionRef = useRef<number | undefined>(undefined)

  const heartbeatRef = useRef<number | null>(null)
  const contadorRef = useRef<number | null>(null)

  async function abrir() {
    if (abierto || cargando) return

    setCargando(true)
    setError('')

    try {
      const progreso = await api.heartbeatRecurso(
        recurso.id,
        posicionRef.current,
      )
      recurso.id,
      posicionRef.current,
    )

      setAbierto(true)
      setCompletado(progreso.Status === 'completed')
      setSegundosActivos(progreso.ActiveSeconds ?? 0)

      if (
        recurso.type === 'rich_text' ||
        recurso.type === 'link' ||
        recurso.type === 'iframe'
      ) {
        const datosContenido = await api.contenidoRecurso(recurso.id)

        if (recurso.type === 'rich_text') {
          setContenido(datosContenido.content_markdown ?? '')
        } else {
          setContenido(datosContenido.external_url ?? '')
        }
      }
    } catch (e) {
      setError(
        mensaje(e, 'No pudimos registrar la apertura del recurso.'),
      )
    } finally {
      setCargando(false)
    }
  }

  async function heartbeat() {
    try {
      const progreso = await api.heartbeatRecurso(
        recurso.id,
        posicionRef.current,
      )

      setSegundosActivos(progreso.ActiveSeconds ?? 0)

      if (progreso.Status === 'completed') {
        setCompletado(true)
      }
    } catch (e) {
      console.error('Error enviando heartbeat', e)
    }
  }

  async function completar() {
    if (cargando || completado || segundosActivos < 30) return

    setCargando(true)
    setError('')

    try {
      // Sincroniza primero el tiempo activo con el backend.
      const progresoActualizado =
        await api.heartbeatRecurso(
          recurso.id,
          posicionRef.current,
        )

      setSegundosActivos(
        progresoActualizado.ActiveSeconds ?? 0,
      )

      if ((progresoActualizado.ActiveSeconds ?? 0) < 30) {
        const faltan =
          30 - (progresoActualizado.ActiveSeconds ?? 0)

        setError(
          `Espera ${faltan} segundos más antes de completar el recurso.`,
        )
        return
      }

      await api.completarRecurso(recurso.id)

      setCompletado(true)
      onCompletado?.()
    } catch (e) {
      setError(
        mensaje(e, 'No pudimos completar el recurso.'),
      )
    } finally {
      setCargando(false)
    }
  }

  useEffect(() => {
    if (!abierto || completado) return

    contadorRef.current = window.setInterval(() => {
      setSegundosActivos(actual => actual + 1)
    }, 1000)

    heartbeatRef.current = window.setInterval(() => {
      void heartbeat()
    }, 15000)

    return () => {
      if (contadorRef.current) {
        window.clearInterval(contadorRef.current)
      }

      if (heartbeatRef.current) {
        window.clearInterval(heartbeatRef.current)
      }
    }
  }, [abierto, completado])

  return (
    <div className="recurso-estudiante">
      <div className="recurso-estudiante-cabecera">
        <div>
          <span className="tipo-recurso">
            {nombreTipo[recurso.type]}
          </span>

          <strong>{recurso.title}</strong>

          {recurso.required && (
            <span className="obligatorio">
              Obligatorio
            </span>
          )}

          {completado && (
            <span className="recurso-completado">
              ✓ Completado
            </span>
          )}
        </div>

        {!abierto ? (
          <button
            type="button"
            className="boton-secundario"
            disabled={cargando}
            onClick={() => void abrir()}
          >
            {cargando ? 'Abriendo…' : 'Abrir recurso'}
          </button>
        ) : (
          <button
            type="button"
            className="boton-principal"
            disabled={
              cargando ||
              completado ||
              segundosActivos < 31
            }
            onClick={() => void completar()}
          >
            {completado
              ? 'Completado'
              : segundosActivos < 31
                ? `Disponible en ${31 - segundosActivos}s`
                : 'Marcar como completado'}
          </button>
        )}
      </div>

      {error && (
        <p className="recurso-estudiante-error">
          {error}
        </p>
      )}
      {abierto && (
        <div className="recurso-estudiante-contenido">
          {recurso.type === 'rich_text' ? (
            contenido ? (
              <div className="recurso-texto-publicado">
                {contenido}
              </div>
            ) : (
              <p>Este recurso no tiene contenido publicado.</p>
            )
          ) : recurso.type === 'link' ? (
            contenido ? (
              <div className="recurso-enlace">
                <p>
                  Este recurso contiene un enlace externo.
                </p>

                <a
                  href={contenido}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="boton-secundario"
                >
                  Abrir enlace ↗
                </a>
              </div>
            ) : (
              <p>Este recurso no tiene un enlace configurado.</p>
            )
          ) : recurso.type === 'iframe' ? (
            contenido ? (
              <div className="recurso-iframe-contenedor">
                <iframe
                  src={contenido}
                  title={recurso.title}
                  className="recurso-iframe"
                  loading="lazy"
                  allowFullScreen
                />
              </div>
            ) : (
              <p>Este recurso no tiene contenido embebido configurado.</p>
            )
          ) : (
            <VisorRecurso
              recurso={recurso}
              onPositionChange={seconds => {
                posicionRef.current = seconds
              }}
            />
          )}

          {!completado && (
            <p className="recurso-estudiante-tiempo">
              Tiempo activo: {segundosActivos}s
            </p>
          )}
        </div>
      )}
    </div>
  )
}