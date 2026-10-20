import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, mensaje } from '../../api/cursos'
import { ErrorEstado } from '../../componentes/estados/Estados'
import '../catalogo/cursos.css'

export function NuevoCurso() {
  const navigate = useNavigate()
  const [titulo, setTitulo] = useState('')
  const [resumen, setResumen] = useState('')
  const [categoria, setCategoria] = useState('')
  const [enviando, setEnviando] = useState(false)
  const [error, setError] = useState('')

  async function crear(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setEnviando(true)
    setError('')
    try {
      const { course } = await api.crearCurso({ title: titulo.trim(), summary: resumen.trim(), category: categoria.trim() })
      navigate(`/profesor/cursos/${course.id}/editar`, { replace: true, state: { creado: true } })
    } catch (e) { setError(mensaje(e, 'No pudimos crear el curso.')) } finally { setEnviando(false) }
  }

  return <>
    <div className="ruta-seccion"><Link to="/profesor/cursos">MIS CURSOS</Link> <span>/</span> NUEVO CURSO</div>
    <section className="bienvenida"><span className="etiqueta texto-verde">PROFESOR</span><h1>Crear un curso</h1>
      <p>Empieza con lo básico. Después agregas módulos, unidades y recursos, y lo publicas cuando esté listo.</p></section>
    <section className="tarjeta formulario-tarjeta">
      <form onSubmit={crear} aria-busy={enviando}>
        <label htmlFor="titulo">Título</label>
        <input id="titulo" required maxLength={200} value={titulo} onChange={e => setTitulo(e.target.value)} disabled={enviando} />
        <label htmlFor="resumen">Resumen</label>
        <textarea id="resumen" rows={4} value={resumen} onChange={e => setResumen(e.target.value)} disabled={enviando} />
        <label htmlFor="categoria">Categoría</label>
        <input id="categoria" placeholder="Por ejemplo: programación" value={categoria} onChange={e => setCategoria(e.target.value)} disabled={enviando} />
        {error && <ErrorEstado mensaje={error} />}
        <div className="acciones">
          <button className="boton-principal" type="submit" disabled={enviando || !titulo.trim()}>{enviando ? 'Creando…' : 'Crear y continuar'}<span aria-hidden="true">→</span></button>
          <Link className="boton-texto" to="/profesor/cursos">Cancelar</Link>
        </div>
      </form>
    </section>
  </>
}
