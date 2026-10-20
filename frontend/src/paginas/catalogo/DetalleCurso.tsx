import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ErrorAPI } from '../../api/cliente'
import { api, mensaje, nombreTipo, type Temario } from '../../api/cursos'
import { useSesion } from '../../auth/Sesion'
import { Cargando, ErrorEstado, Exito, Vacio } from '../../componentes/estados/Estados'
import './cursos.css'

// Detalle de un curso publicado con su temario (sin contenido) y la
// inscripción del estudiante. El profesor dueño ve un acceso a su editor.
export function DetalleCurso() {
  const { id = '' } = useParams()
  const { usuario } = useSesion()
  const navigate = useNavigate()
  const [temario, setTemario] = useState<Temario | null>(null)
  const [inscrito, setInscrito] = useState(false)
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState('')
  const [noExiste, setNoExiste] = useState(false)
  const [inscribiendo, setInscribiendo] = useState(false)
  const [errorInscripcion, setErrorInscripcion] = useState('')
  const [recienInscrito, setRecienInscrito] = useState(false)
  const esEstudiante = usuario?.role === 'student'

  const cargar = useCallback(async () => {
    setCargando(true)
    setError('')
    setNoExiste(false)
    try {
      const [t, inscripciones] = await Promise.all([
        api.temario(id),
        esEstudiante ? api.misInscripciones() : Promise.resolve(null),
      ])
      setTemario(t)
      setInscrito(!!inscripciones?.some(i => i.course_id === id && i.status === 'active'))
    } catch (e) {
      if (e instanceof ErrorAPI && e.status === 404) setNoExiste(true)
      else setError(mensaje(e, 'No pudimos cargar el curso.'))
    } finally { setCargando(false) }
  }, [id, esEstudiante])

  useEffect(() => { void cargar() }, [cargar])

  async function inscribirme() {
    setInscribiendo(true)
    setErrorInscripcion('')
    try {
      await api.inscribirme(id)
      setInscrito(true)
      setRecienInscrito(true)
    } catch (e) {
      // 409: ya estaba inscrito (por ejemplo, en otra pestaña).
      if (e instanceof ErrorAPI && e.status === 409) setInscrito(true)
      else setErrorInscripcion(mensaje(e, 'No pudimos inscribirte.'))
    } finally { setInscribiendo(false) }
  }

  if (cargando) return <Cargando mensaje="Cargando el curso…" />
  if (noExiste) return <section className="pagina-mensaje"><span className="etiqueta texto-verde">CURSO NO DISPONIBLE</span>
    <h1>Este curso no está publicado.</h1><p>Puede que el enlace esté mal o que el curso aún no se haya publicado.</p>
    <Link className="boton-principal" to="/catalogo">Volver al catálogo</Link></section>
  if (error || !temario) return <ErrorEstado mensaje={error || 'No pudimos cargar el curso.'} reintentar={() => void cargar()} />

  const totalRecursos = temario.modules.reduce((n, m) => n + m.units.reduce((k, u) => k + u.resources.length, 0), 0)
  const esDueno = usuario?.id === temario.teacher_id

  return <>
    <div className="ruta-seccion"><Link to="/catalogo">EXPLORAR CURSOS</Link> <span>/</span> {temario.title.toLocaleUpperCase('es')}</div>
    <section className="bienvenida detalle-cabecera">
      {temario.category && <span className="insignia">{temario.category}</span>}
      <h1>{temario.title}</h1>
      <p>{temario.summary}</p>
      <p className="detalle-datos">{temario.modules.length} {temario.modules.length === 1 ? 'módulo' : 'módulos'} · {totalRecursos} {totalRecursos === 1 ? 'recurso' : 'recursos'}</p>
      <div className="acciones">
        {esEstudiante && (inscrito
          ? <button className="boton-principal" onClick={() => navigate(`/aprender/${id}`)}>Ir al curso<span aria-hidden="true">→</span></button>
          : <button className="boton-principal" onClick={() => void inscribirme()} disabled={inscribiendo}>{inscribiendo ? 'Inscribiendo…' : 'Inscribirme'}<span aria-hidden="true">→</span></button>)}
        {esDueno && <Link className="boton-secundario" to={`/profesor/cursos/${id}/editar`}>Editar este curso</Link>}
      </div>
      {recienInscrito && <Exito>Quedaste inscrito. Ya puedes empezar el curso.</Exito>}
      {errorInscripcion && <ErrorEstado mensaje={errorInscripcion} />}
    </section>

    <section className="tarjeta">
      <div className="tarjeta-cabecera"><h2>Temario</h2></div>
      {temario.modules.length === 0 ? <Vacio titulo="Este curso todavía no tiene módulos" /> :
        <ol className="temario">
          {temario.modules.map(m => <li key={m.id}><h3>{m.title}</h3>
            <ol>{m.units.map(u => <li key={u.id}><span className="temario-unidad">{u.title}</span>
              <ul>{u.resources.map(r => <li key={r.id}><span className="tipo-recurso">{nombreTipo[r.type] || r.type}</span> {r.title}{r.required && <span className="obligatorio"> · obligatorio</span>}</li>)}</ul>
            </li>)}</ol>
          </li>)}
        </ol>}
    </section>
  </>
}
