import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, mensaje, type CursoCatalogo } from '../../api/cursos'
import { Cargando, ErrorEstado, Vacio } from '../../componentes/estados/Estados'
import './cursos.css'

// Catálogo de cursos publicados: búsqueda por texto y categoría, con
// paginación por cursor ("Ver más"). Los filtros viven en la URL para que
// se puedan compartir y sobrevivan a recargar la página.
export function Catalogo() {
  const [params, setParams] = useSearchParams()
  const texto = params.get('q') || ''
  const categoria = params.get('categoria') || ''
  const [busqueda, setBusqueda] = useState(texto)
  const [filtroCategoria, setFiltroCategoria] = useState(categoria)
  const [cursos, setCursos] = useState<CursoCatalogo[]>([])
  const [siguiente, setSiguiente] = useState('')
  const [cargando, setCargando] = useState(true)
  const [cargandoMas, setCargandoMas] = useState(false)
  const [error, setError] = useState('')

  const cargar = useCallback(async () => {
    setCargando(true)
    setError('')
    try {
      const pagina = await api.catalogo({ q: texto, category: categoria, limit: 12 })
      setCursos(pagina.courses)
      setSiguiente(pagina.next_cursor)
    } catch (e) { setError(mensaje(e, 'No pudimos cargar el catálogo.')) } finally { setCargando(false) }
  }, [texto, categoria])

  useEffect(() => { void cargar() }, [cargar])

  async function verMas() {
    setCargandoMas(true)
    try {
      const pagina = await api.catalogo({ q: texto, category: categoria, cursor: siguiente, limit: 12 })
      setCursos(actuales => [...actuales, ...pagina.courses])
      setSiguiente(pagina.next_cursor)
    } catch (e) { setError(mensaje(e, 'No pudimos cargar más cursos.')) } finally { setCargandoMas(false) }
  }

  function buscar(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const nuevos = new URLSearchParams()
    if (busqueda.trim()) nuevos.set('q', busqueda.trim())
    if (filtroCategoria.trim()) nuevos.set('categoria', filtroCategoria.trim())
    setParams(nuevos)
  }

  return <>
    <div className="ruta-seccion">MI ESPACIO <span>/</span> EXPLORAR CURSOS</div>
    <section className="bienvenida"><span className="etiqueta texto-verde">CATÁLOGO</span><h1>Explora cursos</h1>
      <p>Cursos publicados por los profesores de la plataforma.</p></section>

    <form className="filtros" role="search" onSubmit={buscar}>
      <div className="campo"><label htmlFor="q">Buscar</label>
        <input id="q" type="search" placeholder="Título o descripción" value={busqueda} onChange={e => setBusqueda(e.target.value)} /></div>
      <div className="campo"><label htmlFor="categoria">Categoría</label>
        <input id="categoria" placeholder="Por ejemplo: programación" value={filtroCategoria} onChange={e => setFiltroCategoria(e.target.value)} /></div>
      <button className="boton-principal" type="submit">Buscar</button>
    </form>

    {cargando ? <Cargando mensaje="Buscando cursos…" />
      : error ? <ErrorEstado mensaje={error} reintentar={() => void cargar()} />
      : cursos.length === 0 ? <Vacio titulo={texto || categoria ? 'No hay cursos con esos filtros' : 'Todavía no hay cursos publicados'}>
          {(texto || categoria) && <p><button className="boton-secundario" onClick={() => { setBusqueda(''); setFiltroCategoria(''); setParams(new URLSearchParams()) }}>Ver todos los cursos</button></p>}
        </Vacio>
      : <>
        <ul className="rejilla-cursos" aria-label="Cursos publicados">
          {cursos.map(c => <li key={c.course_id} className="tarjeta tarjeta-curso">
            {c.category && <span className="insignia">{c.category}</span>}
            <h2><Link to={`/cursos/${c.course_id}`}>{c.title}</Link></h2>
            <p>{c.summary || 'Sin descripción.'}</p>
            <Link className="enlace-accion" to={`/cursos/${c.course_id}`} aria-label={`Ver el curso ${c.title}`}>Ver curso →</Link>
          </li>)}
        </ul>
        {siguiente && <div className="centrado"><button className="boton-secundario" onClick={() => void verMas()} disabled={cargandoMas}>{cargandoMas ? 'Cargando…' : 'Ver más cursos'}</button></div>}
      </>}
  </>
}
