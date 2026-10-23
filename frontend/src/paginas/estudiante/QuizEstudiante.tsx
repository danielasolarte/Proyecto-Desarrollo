import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  enviarIntento,
  guardarRespuesta,
  iniciarIntento,
  obtenerIntento,
  type IntentoQuiz,
} from '../../api/quizzes'
import { Cargando, ErrorEstado, Exito } from '../../componentes/estados/Estados'

function mensajeError(error: unknown) {
  return error instanceof Error
    ? error.message
    : 'No pudimos completar la operación.'
}

export function QuizEstudiante() {
  const { quizId } = useParams()
  const navigate = useNavigate()

  const [intento, setIntento] = useState<IntentoQuiz | null>(null)
  const [respuestas, setRespuestas] = useState<Record<string, string>>({})
  const [cargando, setCargando] = useState(true)
  const [guardando, setGuardando] = useState<string | null>(null)
  const [enviando, setEnviando] = useState(false)
  const [error, setError] = useState('')
  const [enviado, setEnviado] = useState(false)

  async function cargarIntento() {
    if (!quizId) {
        setError('No se encontró el quiz.')
        setCargando(false)
        return
    }

    setCargando(true)
    setError('')

    const storageKey = `quiz-attempt-${quizId}`

    try {
        const attemptGuardado = localStorage.getItem(storageKey)

        if (attemptGuardado) {
            try {
                const detalle = await obtenerIntento(attemptGuardado)

                setIntento(detalle.attempt)
                setEnviado(detalle.attempt.status === 'submitted')

                const respuestasGuardadas: Record<string, string> = {}

                for (const respuesta of detalle.answers ?? []) {
                    if (respuesta.selected_option_id) {
                        respuestasGuardadas[respuesta.question_id] =
                        respuesta.selected_option_id
                    }
                }

                setRespuestas(respuestasGuardadas)
                return
            } catch {
                /*
                Si el intento guardado ya no existe o no es válido,
                eliminamos la referencia y creamos uno nuevo.
                */
                localStorage.removeItem(storageKey)
            }
        }

        const nuevoIntento = await iniciarIntento(quizId)

        localStorage.setItem(storageKey, nuevoIntento.id)

        setIntento(nuevoIntento)
        setEnviado(nuevoIntento.status === 'submitted')
        setRespuestas({})
    } catch (e) {
            setError(mensajeError(e))
    } finally {
            setCargando(false)
    }
    }

  useEffect(() => {
    void cargarIntento()
  }, [quizId])

  const preguntas = useMemo(
    () =>
      [...(intento?.snapshot.questions ?? [])].sort(
        (a, b) => a.position - b.position,
      ),
    [intento],
  )

  const respondidas = Object.keys(respuestas).length
  const total = preguntas.length
  const completo = total > 0 && respondidas === total

  async function seleccionarRespuesta(
    questionId: string,
    optionId: string,
  ) {
    if (!intento || enviado || enviando) return

    const anterior = respuestas[questionId]

    setRespuestas(actuales => ({
      ...actuales,
      [questionId]: optionId,
    }))

    setGuardando(questionId)
    setError('')

    try {
      await guardarRespuesta(intento.id, questionId, optionId)
    } catch (e) {
      setRespuestas(actuales => {
        const nuevas = { ...actuales }

        if (anterior) nuevas[questionId] = anterior
        else delete nuevas[questionId]

        return nuevas
      })

      setError(mensajeError(e))
    } finally {
      setGuardando(null)
    }
  }

  async function enviar() {
    if (!intento || enviando || enviado || !completo) return

    setEnviando(true)
    setError('')

    /*
      Una misma key se reutiliza durante este envío. Además el botón queda
      deshabilitado mientras la solicitud está en curso, evitando el doble
      submit desde la interfaz.
    */
    const idempotencyKey = crypto.randomUUID()

    try {
      const resultado = await enviarIntento(
        intento.id,
        idempotencyKey,
      )

      setIntento(resultado)
      setEnviado(true)
    } catch (e) {
      setError(mensajeError(e))
    } finally {
      setEnviando(false)
    }
  }

  if (cargando) {
    return <Cargando mensaje="Preparando tu evaluación…" />
  }

  if (error && !intento) {
    return (
      <ErrorEstado
        mensaje={error}
        reintentar={() => void cargarIntento()}
      />
    )
  }

  if (!intento) return null

  return (
    <>
      <div className="ruta-seccion">
        MI APRENDIZAJE <span>/</span> EVALUACIÓN
      </div>

      <section className="bienvenida">
        <span className="etiqueta texto-verde">QUIZ</span>
        <h1>Presenta tu evaluación</h1>

        {!enviado ? (
          <p>
            Responde todas las preguntas antes de enviar. Tus respuestas
            se guardan mientras avanzas.
          </p>
        ) : (
          <p>La evaluación ya fue enviada.</p>
        )}
      </section>

      {error && (
        <ErrorEstado mensaje={error} />
      )}

      {enviado ? (
        <section className="tarjeta quiz-resultado">
          <Exito>
            <strong>Evaluación enviada correctamente.</strong>
          </Exito>

          <div className="quiz-resumen">
            <div>
              <span className="quiz-resumen-etiqueta">Puntaje</span>
              <strong>{intento.score ?? 0}</strong>
            </div>

            <div>
              <span className="quiz-resumen-etiqueta">Porcentaje</span>
              <strong>
                {typeof intento.percentage === 'number'
                  ? `${intento.percentage.toFixed(1)} %`
                  : '—'}
              </strong>
            </div>

            <div>
              <span className="quiz-resumen-etiqueta">Resultado</span>
              <strong>
                {intento.passed === true
                  ? 'Aprobado'
                  : intento.passed === false
                    ? 'No aprobado'
                    : 'Pendiente'}
              </strong>
            </div>
          </div>

          <button
            className="boton-secundario"
            type="button"
            onClick={() => navigate(-1)}
          >
            Volver al curso
          </button>
        </section>
      ) : (
        <>
          <div className="quiz-progreso">
            <span>
              {respondidas} de {total} preguntas respondidas
            </span>

            <div
              className="quiz-progreso-barra"
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={total}
              aria-valuenow={respondidas}
            >
              <span
                style={{
                  width: total
                    ? `${(respondidas / total) * 100}%`
                    : '0%',
                }}
              />
            </div>
          </div>

          <ol className="quiz-preguntas">
            {preguntas.map((pregunta, indice) => (
              <li key={pregunta.id} className="tarjeta quiz-pregunta">
                <div className="quiz-pregunta-cabecera">
                  <span className="insignia">
                    Pregunta {indice + 1}
                  </span>

                  <span className="quiz-puntos">
                    {pregunta.points}{' '}
                    {pregunta.points === 1 ? 'punto' : 'puntos'}
                  </span>
                </div>

                <h2>{pregunta.text}</h2>

                <fieldset
                  className="quiz-opciones"
                  disabled={enviando}
                >
                  <legend className="sr-only">
                    Opciones para la pregunta {indice + 1}
                  </legend>

                  {[...pregunta.options]
                    .sort((a, b) => a.position - b.position)
                    .map(opcion => {
                      const seleccionada =
                        respuestas[pregunta.id] === opcion.id

                      return (
                        <label
                          key={opcion.id}
                          className={`quiz-opcion ${
                            seleccionada
                              ? 'quiz-opcion-seleccionada'
                              : ''
                          }`}
                        >
                          <input
                            type="radio"
                            name={`pregunta-${pregunta.id}`}
                            value={opcion.id}
                            checked={seleccionada}
                            onChange={() =>
                              void seleccionarRespuesta(
                                pregunta.id,
                                opcion.id,
                              )
                            }
                          />

                          <span>{opcion.text}</span>
                        </label>
                      )
                    })}
                </fieldset>

                {guardando === pregunta.id && (
                  <p className="quiz-guardando">
                    Guardando respuesta…
                  </p>
                )}
              </li>
            ))}
          </ol>

          <div className="quiz-enviar">
            {!completo && (
              <p>
                Debes responder todas las preguntas antes de enviar.
              </p>
            )}

            <button
              className="boton-principal"
              type="button"
              disabled={!completo || enviando || guardando !== null}
              onClick={() => void enviar()}
            >
              {enviando
                ? 'Enviando evaluación…'
                : 'Enviar evaluación'}
            </button>
          </div>
        </>
      )}
    </>
  )
}