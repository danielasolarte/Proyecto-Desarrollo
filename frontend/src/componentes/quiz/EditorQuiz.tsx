// Editor del quiz de un recurso de tipo quiz: crea la configuración del quiz
// (nota mínima, intentos, tiempo, retroalimentación) y agrega preguntas de
// opción múltiple con una respuesta correcta.
//
// Límite del backend actual: las preguntas y opciones solo se pueden crear,
// no editar ni borrar. Por eso se valida todo antes de enviar.
import { useCallback, useEffect, useId, useState, type FormEvent } from 'react'
import { ErrorAPI } from '../../api/cliente'
import { siguientePosicion } from '../../api/cursos'
import { apiQuiz, nombreModo, problemaPregunta, type ModoRetroalimentacion, type QuizAutoria } from '../../api/quiz'
import { Cargando, ErrorEstado } from '../estados/Estados'
import './quiz.css'

const mensajeDe = (e: unknown, porDefecto: string) => (e instanceof Error && e.message ? e.message : porDefecto)

export function EditorQuiz({ resourceId, editable = true }: { resourceId: string; editable?: boolean }) {
  const [datos, setDatos] = useState<QuizAutoria | null>(null)
  const [sinQuiz, setSinQuiz] = useState(false)
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState('')

  const cargar = useCallback(async () => {
    setError('')
    try {
      setDatos(await apiQuiz.delRecurso(resourceId))
      setSinQuiz(false)
    } catch (e) {
      if (e instanceof ErrorAPI && e.status === 404) { setDatos(null); setSinQuiz(true) }
      else setError(mensajeDe(e, 'No se pudo cargar el quiz.'))
    } finally {
      setCargando(false)
    }
  }, [resourceId])

  useEffect(() => { void cargar() }, [cargar])

  if (cargando) return <Cargando mensaje="Cargando el quiz…" />
  if (error) return <ErrorEstado mensaje={error} reintentar={() => { setCargando(true); void cargar() }} />
  if (sinQuiz || !datos) {
    return editable
      ? <ConfigurarQuiz resourceId={resourceId} alCrear={cargar} />
      : <p className="ranura-nota">Este quiz todavía no tiene preguntas.</p>
  }

  const { quiz } = datos
  const preguntas = [...(datos.questions || [])].sort((a, b) => a.Position - b.Position)
  const total = preguntas.reduce((s, p) => s + p.Points, 0)
  return <div className="editor-quiz">
    <p className="quiz-resumen">
      Aprueba con {quiz.PassingScore}% · {quiz.MaxAttempts ? `${quiz.MaxAttempts} intento(s)` : 'intentos ilimitados'}
      {' · '}{quiz.TimeLimitSeconds ? `${Math.round(quiz.TimeLimitSeconds / 60)} min` : 'sin límite de tiempo'}
      {' · '}{nombreModo[quiz.FeedbackMode] || quiz.FeedbackMode}
    </p>
    {preguntas.length === 0
      ? <p className="ranura-nota">Aún no hay preguntas. Agrega la primera.</p>
      : <ol className="quiz-preguntas">
        {preguntas.map(p => <li key={p.ID}>
          <span className="quiz-enunciado">{p.Text}</span> <span className="quiz-puntos">{p.Points} pt</span>
          <ul>{[...(p.Options || [])].sort((a, b) => a.Position - b.Position).map(o =>
            <li key={o.ID} className={o.IsCorrect ? 'quiz-correcta' : undefined}>
              {o.Text}{o.IsCorrect && <span className="oculto"> (correcta)</span>}
            </li>)}</ul>
        </li>)}
      </ol>}
    {preguntas.length > 0 && <p className="ranura-nota">{preguntas.length} pregunta(s) · {total} punto(s) en total</p>}
    {editable && <NuevaPregunta quizId={quiz.ID} posicion={siguientePosicion(preguntas.map(p => ({ position: p.Position })))} alCrear={cargar} />}
  </div>
}

function ConfigurarQuiz({ resourceId, alCrear }: { resourceId: string; alCrear: () => Promise<void> }) {
  const id = useId()
  const [nota, setNota] = useState('60')
  const [intentos, setIntentos] = useState('')
  const [minutos, setMinutos] = useState('')
  const [modo, setModo] = useState<ModoRetroalimentacion>('after_submit')
  const [ocupado, setOcupado] = useState(false)
  const [error, setError] = useState('')

  async function enviar(e: FormEvent) {
    e.preventDefault()
    const n = Number(nota)
    if (!(n >= 0 && n <= 100)) { setError('La nota mínima debe estar entre 0 y 100.'); return }
    const i = intentos.trim() ? Number(intentos) : null
    const m = minutos.trim() ? Number(minutos) : null
    if (i !== null && !(Number.isInteger(i) && i > 0)) { setError('Los intentos deben ser un número entero mayor que cero.'); return }
    if (m !== null && !(m > 0)) { setError('El tiempo debe ser mayor que cero.'); return }
    setOcupado(true); setError('')
    try {
      await apiQuiz.crear(resourceId, { passing_score: n, max_attempts: i, time_limit_seconds: m === null ? null : Math.round(m * 60), feedback_mode: modo })
      await alCrear()
    } catch (err) {
      setError(mensajeDe(err, 'No se pudo crear el quiz.'))
    } finally {
      setOcupado(false)
    }
  }

  return <form className="quiz-config" onSubmit={enviar}>
    <p className="ranura-nota">Configura el quiz antes de agregar preguntas.</p>
    <div className="quiz-campos">
      <div className="campo"><label htmlFor={`${id}-nota`}>Nota mínima para aprobar (%)</label>
        <input id={`${id}-nota`} type="number" min={0} max={100} step="any" required value={nota} onChange={e => setNota(e.target.value)} /></div>
      <div className="campo"><label htmlFor={`${id}-intentos`}>Intentos permitidos</label>
        <input id={`${id}-intentos`} type="number" min={1} step={1} placeholder="Ilimitados" value={intentos} onChange={e => setIntentos(e.target.value)} /></div>
      <div className="campo"><label htmlFor={`${id}-min`}>Tiempo límite (minutos)</label>
        <input id={`${id}-min`} type="number" min={1} step="any" placeholder="Sin límite" value={minutos} onChange={e => setMinutos(e.target.value)} /></div>
      <div className="campo"><label htmlFor={`${id}-modo`}>Retroalimentación</label>
        <select id={`${id}-modo`} value={modo} onChange={e => setModo(e.target.value as ModoRetroalimentacion)}>
          {(Object.keys(nombreModo) as ModoRetroalimentacion[]).map(k => <option key={k} value={k}>{nombreModo[k]}</option>)}
        </select></div>
    </div>
    {error && <div className="aviso aviso-error" role="alert"><p>{error}</p></div>}
    <button className="boton-secundario" type="submit" disabled={ocupado}>{ocupado ? 'Creando…' : 'Crear quiz'}</button>
  </form>
}

function NuevaPregunta({ quizId, posicion, alCrear }: { quizId: string; posicion: number; alCrear: () => Promise<void> }) {
  const id = useId()
  const [abierta, setAbierta] = useState(false)
  const [texto, setTexto] = useState('')
  const [puntos, setPuntos] = useState('1')
  const [opciones, setOpciones] = useState([{ texto: '' }, { texto: '' }])
  const [correcta, setCorrecta] = useState(0)
  const [ocupado, setOcupado] = useState(false)
  const [error, setError] = useState('')

  function reiniciar() {
    setTexto(''); setPuntos('1'); setOpciones([{ texto: '' }, { texto: '' }]); setCorrecta(0); setError('')
  }

  async function enviar(e: FormEvent) {
    e.preventDefault()
    const p = Number(puntos)
    const problema = problemaPregunta(texto, p, opciones, correcta)
    if (problema) { setError(problema); return }
    setOcupado(true); setError('')
    // Se envían solo las opciones con texto, conservando cuál es la correcta.
    const llenas = opciones.map((o, i) => ({ texto: o.texto.trim(), correcta: i === correcta })).filter(o => o.texto)
    try {
      const pregunta = await apiQuiz.crearPregunta(quizId, { text: texto.trim(), position: posicion, points: p })
      try {
        for (const [i, o] of llenas.entries()) {
          await apiQuiz.crearOpcion(pregunta.ID, { text: o.texto, position: i + 1, is_correct: o.correcta })
        }
      } catch (err) {
        await alCrear()
        setError(`La pregunta se creó pero faltaron opciones: ${mensajeDe(err, 'error al guardar una opción')}.`)
        return
      }
      reiniciar(); setAbierta(false)
      await alCrear()
    } catch (err) {
      setError(mensajeDe(err, 'No se pudo crear la pregunta.'))
    } finally {
      setOcupado(false)
    }
  }

  if (!abierta) return <button className="boton-secundario" type="button" onClick={() => setAbierta(true)}>Agregar pregunta</button>

  return <form className="nueva-pregunta" onSubmit={enviar}>
    <div className="campo"><label htmlFor={`${id}-texto`}>Pregunta</label>
      <textarea id={`${id}-texto`} rows={2} required value={texto} onChange={e => setTexto(e.target.value)} /></div>
    <div className="campo quiz-puntos-campo"><label htmlFor={`${id}-puntos`}>Puntos</label>
      <input id={`${id}-puntos`} type="number" min={0.5} step="any" required value={puntos} onChange={e => setPuntos(e.target.value)} /></div>
    <fieldset className="quiz-opciones">
      <legend>Opciones (marca la correcta)</legend>
      {opciones.map((o, i) => <div className="quiz-opcion" key={i}>
        <input type="radio" name={`${id}-correcta`} aria-label={`Opción ${i + 1} es la correcta`} checked={correcta === i} onChange={() => setCorrecta(i)} />
        <input aria-label={`Texto de la opción ${i + 1}`} value={o.texto} onChange={e => setOpciones(opciones.map((x, j) => (j === i ? { texto: e.target.value } : x)))} />
        {opciones.length > 2 && <button className="boton-texto" type="button" aria-label={`Quitar opción ${i + 1}`}
          onClick={() => { setOpciones(opciones.filter((_, j) => j !== i)); setCorrecta(c => (c === i ? 0 : c > i ? c - 1 : c)) }}>Quitar</button>}
      </div>)}
      {opciones.length < 6 && <button className="boton-texto" type="button" onClick={() => setOpciones([...opciones, { texto: '' }])}>Agregar opción</button>}
    </fieldset>
    <p className="ranura-nota">Revisa antes de guardar: por ahora las preguntas no se pueden editar ni borrar.</p>
    {error && <div className="aviso aviso-error" role="alert"><p>{error}</p></div>}
    <div className="acciones">
      <button className="boton-secundario" type="submit" disabled={ocupado}>{ocupado ? 'Guardando…' : 'Guardar pregunta'}</button>
      <button className="boton-texto" type="button" disabled={ocupado} onClick={() => { reiniciar(); setAbierta(false) }}>Cancelar</button>
    </div>
  </form>
}
