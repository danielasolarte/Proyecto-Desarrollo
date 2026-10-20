import { useState } from 'react'
import { Link, NavLink, Outlet } from 'react-router-dom'
import { inicioPorRol, nombreRol, useSesion } from '../auth/Sesion'
import { ErrorEstado } from './estados/Estados'

export function Marco() {
  const { usuario, salir } = useSesion()
  const [saliendo, setSaliendo] = useState(false)
  const [error, setError] = useState('')
  if (!usuario) return null
  async function cerrar() {
    setSaliendo(true)
    setError('')
    try { await salir() } catch (e) {
      setError(e instanceof Error ? e.message : 'No pudimos cerrar la sesión.')
    } finally { setSaliendo(false) }
  }
  return <>
    <header className="cabecera"><Link className="marca" to={inicioPorRol[usuario.role]}><span className="marca-icono" aria-hidden="true">m</span>mooc<span className="marca-punto">.</span></Link>
      <nav aria-label="Navegación principal">
        {usuario.role === 'student' && <><NavLink to="/mis-cursos">Mi aprendizaje</NavLink><NavLink to="/catalogo">Explorar cursos</NavLink></>}
        {usuario.role === 'teacher' && <><NavLink to="/profesor/cursos">Mis cursos</NavLink><NavLink to="/catalogo">Explorar cursos</NavLink></>}
        {usuario.role === 'admin' && <NavLink to="/admin">Mi espacio</NavLink>}
      </nav>
      <div className="cuenta"><span className="rol">{nombreRol[usuario.role]}</span><button className="boton-texto" onClick={() => void cerrar()} disabled={saliendo}>{saliendo ? 'Saliendo…' : 'Cerrar sesión'}</button></div>
    </header>
    <main id="contenido" className="contenido">{error && <ErrorEstado mensaje={error} />}<Outlet /></main>
  </>
}
