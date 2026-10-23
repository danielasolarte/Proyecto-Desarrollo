import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { obtenerIntento } from '../../api/quizzes'
import { RecursoEstudiante } from '../../componentes/estudiante/RecursoEstudiante'
import {
  api,
  mensaje,
  nombreTipo,
  type ProgresoCurso,
  type Temario,
} from '../../api/cursos'
import {
  Cargando,
  ErrorEstado,
} from '../../componentes/estados/Estados'
import '../catalogo/cursos.css'

export function AprenderCurso() {
  const { cursoId } = useParams()
  const navigate = useNavigate()

  const [temario, setTemario] = useState<Temario | null>(null)
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState('')
  const [abriendoRecurso, setAbriendoRecurso] = useState<string | null>(null)
  const [progreso, setProgreso] = useState<ProgresoCurso | null>(null)
  const [quizzesCompletados, setQuizzesCompletados] = useState<Record<string, boolean>>({})

  const cargar = useCallback(async () => {
    if (!cursoId) {
      setError('No se encontró el curso.')
      setCargando(false)
      return
    }

    setCargando(true)
    setError('')

    try {
      const [datosTemario, datosProgreso] = await Promise.all([
        api.temario(cursoId),
        api.progresoCurso(cursoId),
        ])

        setTemario(datosTemario)
        setProgreso(datosProgreso)
        const recursosQuiz = datosTemario.modules.flatMap(modulo =>
            modulo.units.flatMap(unidad =>
                unidad.resources.filter(
                    recurso => recurso.type === 'quiz',
                ),
            ),
        )

        const estadosQuiz = await Promise.all(
            recursosQuiz.map(async recurso => [
                recurso.id,
                await verificarQuizCompletado(recurso.id),
            ] as const),
        )

        setQuizzesCompletados(
            Object.fromEntries(estadosQuiz),
        )
    } catch (e) {
      setError(
        mensaje(e, 'No pudimos cargar el contenido del curso.'),
      )
    } finally {
      setCargando(false)
    }
  }, [cursoId])

  useEffect(() => {
    void cargar()
  }, [cargar])

  async function verificarQuizCompletado(resourceId: string) {
    try {
        const quiz = await api.quizDeRecurso(resourceId)

        const attemptId = localStorage.getItem(
            `quiz-attempt-${quiz.id}`,
     )

        if (!attemptId) {
            return false
        }

        const detalle = await obtenerIntento(attemptId)

        return detalle.attempt.status === 'submitted'
    } catch {
        return false
    }
}

  async function abrirQuiz(resourceId: string) {
    if (abriendoRecurso) return

    setAbriendoRecurso(resourceId)
    setError('')

    try {
      const quiz = await api.quizDeRecurso(resourceId)
      navigate(`/quiz/${quiz.id}`)
    } catch (e) {
      setError(
        mensaje(e, 'No pudimos abrir la evaluación.'),
      )
    } finally {
      setAbriendoRecurso(null)
    }
  }

  if (cargando) {
    return <Cargando mensaje="Cargando contenido del curso…" />
  }

  if (error && !temario) {
    return (
      <ErrorEstado
        mensaje={error}
        reintentar={() => void cargar()}
      />
    )
  }

  if (!temario) return null

  return (
    <>
      <div className="ruta-seccion">
        MI APRENDIZAJE <span>/</span> {temario.title}
      </div>

      <section className="bienvenida detalle-cabecera">
        <span className="insignia">
          {temario.category}
        </span>

        <h1>{temario.title}</h1>

        <p>{temario.summary}</p>
      </section>

      <section className="tarjeta aprender-progreso">
        <div className="aprender-progreso-cabecera">
            <div>
            <span className="etiqueta texto-verde">
                PROGRESO DEL CURSO
            </span>

            <strong>
                {progreso?.ProgressPercentage?.toFixed(0) ?? '0'} %
            </strong>
            </div>

            <span className="aprender-progreso-estado">
            {progreso?.Status === 'approved'
                ? 'Aprobado'
                : progreso?.Status === 'completed'
                ? 'Completado'
                : 'En progreso'}
            </span>
        </div>

        <div
            className="aprender-progreso-barra"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={progreso?.ProgressPercentage ?? 0}
        >
            <span
            style={{
                width: `${Math.min(
                100,
                progreso?.ProgressPercentage ?? 0,
                )}%`,
            }}
            />
        </div>
        </section>

      {error && (
        <ErrorEstado mensaje={error} />
      )}

      {temario.modules.length === 0 ? (
        <section className="tarjeta">
          <p>Este curso todavía no tiene contenido disponible.</p>
        </section>
      ) : (
        <ol className="temario aprender-temario">
          {temario.modules.map((modulo, indiceModulo) => (
            <li key={modulo.id}>
              <section className="tarjeta aprender-modulo">
                <div className="aprender-modulo-cabecera">
                  <span className="etiqueta texto-verde">
                    MÓDULO {indiceModulo + 1}
                  </span>

                  <h2>{modulo.title}</h2>
                </div>

                {modulo.units.length === 0 ? (
                  <p className="aprender-vacio">
                    Este módulo no tiene unidades.
                  </p>
                ) : (
                  <ol className="aprender-unidades">
                    {modulo.units.map((unidad, indiceUnidad) => (
                      <li
                        key={unidad.id}
                        className="aprender-unidad"
                      >
                        <h3>
                          {indiceUnidad + 1}. {unidad.title}
                        </h3>

                        {unidad.resources.length === 0 ? (
                          <p className="aprender-vacio">
                            Esta unidad no tiene recursos.
                          </p>
                        ) : (
                          <ul className="aprender-recursos">
                            {unidad.resources.map(recurso => (
                              <li
                                key={recurso.id}
                                className="aprender-recurso"
                                >
                                {recurso.type === 'quiz' ? (
                                    <>
                                    <div>
                                        <div className="aprender-recurso-titulo">
                                        <span className="tipo-recurso">
                                            {nombreTipo[recurso.type]}
                                        </span>

                                        <strong>{recurso.title}</strong>

                                        {recurso.required && (
                                            <span className="obligatorio">
                                            Obligatorio
                                            </span>
                                        )}

                                        {quizzesCompletados[recurso.id] && (
                                            <span className="recurso-completado">
                                            ✓ Completado
                                            </span>
                                        )}
                                        </div>
                                    </div>

                                    <button
                                        type="button"
                                        className="boton-secundario"
                                        disabled={abriendoRecurso === recurso.id}
                                        onClick={() => void abrirQuiz(recurso.id)}
                                    >
                                        {abriendoRecurso === recurso.id
                                        ? 'Abriendo…'
                                        : quizzesCompletados[recurso.id]
                                            ? 'Ver resultado'
                                            : 'Presentar quiz'}
                                    </button>
                                    </>
                                ) : (
                                    <RecursoEstudiante
                                    recurso={recurso}
                                    onCompletado={async () => {
                                        if (!cursoId) return

                                        try {
                                        const nuevoProgreso =
                                            await api.progresoCurso(cursoId)

                                        setProgreso(nuevoProgreso)
                                        } catch (e) {
                                        setError(
                                            mensaje(
                                            e,
                                            'El recurso se completó, pero no pudimos actualizar el progreso del curso.',
                                            ),
                                        )
                                        }
                                    }}
                                    />
                                )}
                                </li>
                            ))}
                          </ul>
                        )}
                      </li>
                    ))}
                  </ol>
                )}
              </section>
            </li>
          ))}
        </ol>
      )}
    </>
  )
}