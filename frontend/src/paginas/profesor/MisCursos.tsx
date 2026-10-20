import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, mensaje, nombreEstado, type CursoProfesor } from '../../api/cursos'
import { Cargando, ErrorEstado, Vacio } from '../../componentes/estados/Estados'
import '../catalogo/cursos.css'

// "Mis cursos" del profesor: sus cursos en borrador y publicados
// (GET /me/courses). El catálogo solo muestra los publicados.
export function MisCursos() {
  const [cursos, setCursos] = useState<CursoProfesor[]>([])
  const [siguiente, setSiguiente] = useState('')
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState('')

  const cargar = useCallback(async () => {
    setCargando(true)
    setError('')
    try {
      const pagina = await api.misCursos()
      setCursos(pagina.courses)
      setSiguiente(pagina.next_cursor)
    } catch (e) { setError(mensaje(e, 'No pudimos cargar tus cursos.')) } finally { setCargando(false) }
  }, [])

  useEffect(() => { void cargar() }, [cargar])

  async function verMas() {
    try {
      const pagina = await api.misCursos(siguiente)
      setCursos(actuales => [...actuales, ...pagina.courses])
      setSiguiente(pagina.next_cursor)
    } catch (e) { setError(mensaje(e)) }
  }

  return <>
    <div className="ruta-seccion">MI ESPACIO <span>/</span> MIS CURSOS</div>
    <section className="bienvenida encabezado-con-accion">
      <div><span className="etiqueta texto-verde">PROFESOR</span><h1>Mis cursos</h1><p>Crea, edita y publica tus cursos.</p></div>
      <Link className="boton-principal" to="/profesor/cursos/nuevo">Crear curso<span aria-hidden="true">＋</span></Link>
    </section>

    {cargando ? <Cargando mensaje="Cargando tus cursos…" />
      : error ? <ErrorEstado mensaje={error} reintentar={() => void cargar()} />
      : cursos.length === 0 ? <Vacio titulo="Todavía no has creado cursos"><p>Empieza con el botón Crear curso.</p></Vacio>
      : <section className="tarjeta">
        <ul className="lista-cursos" aria-label="Mis cursos">
          {cursos.map(c => <li key={c.id}>
            <div>
              <h2><Link to={`/profesor/cursos/${c.id}/editar`}>{c.title || 'Curso sin título'}</Link></h2>
              <p>{c.category || 'Sin categoría'} · actualizado el {new Date(c.updated_at).toLocaleDateString('es-CO')}</p>
            </div>
            <span className={`estado-curso estado-${c.status}`}>{nombreEstado[c.status]}</span>
            <div className="acciones">
              <Link className="boton-secundario" to={`/profesor/cursos/${c.id}/editar`}>Editar</Link>
              {c.status !== 'draft' && <Link className="boton-texto" to={`/cursos/${c.id}`}>Ver publicado</Link>}
            </div>
          </li>)}
        </ul>
        {siguiente && <div className="centrado"><button className="boton-secundario" onClick={() => void verMas()}>Ver más</button></div>}
      </section>}
  </>
}
