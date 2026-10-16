import { Link, Navigate, Route, Routes } from 'react-router-dom'
import { inicioPorRol, useSesion } from './auth/Sesion'
import { RutaProtegida } from './auth/RutaProtegida'
import { Marco } from './componentes/Marco'
import { Cargando, ErrorEstado } from './componentes/estados/Estados'
import { Login } from './paginas/Login'
import { Inicio, NoEncontrado, SinPermiso } from './paginas/Inicio'
import { Catalogo } from './paginas/catalogo/Catalogo'
import { DetalleCurso } from './paginas/catalogo/DetalleCurso'
import { MisCursos } from './paginas/profesor/MisCursos'
import { NuevoCurso } from './paginas/profesor/NuevoCurso'
import { EditorCurso } from './paginas/profesor/EditorCurso'

declare const __APP_COMMIT__: string

function RedireccionInicio() {
  const { usuario, cargando, error, recargar } = useSesion()
  if (cargando) return <Cargando />
  if (error) return <ErrorEstado mensaje={error} reintentar={() => void recargar()} />
  return <Navigate to={usuario ? inicioPorRol[usuario.role] : '/login'} replace />
}

export function App() {
  return <div className="app">
    <a className="saltar" href="#contenido">Saltar al contenido</a>
    <Routes>
      <Route path="/login" element={<><header className="cabecera cabecera-login"><Link className="marca" to="/"><span className="marca-icono" aria-hidden="true">m</span>mooc<span className="marca-punto">.</span></Link><span className="cabecera-leyenda">Enseña. Aprende. Avanza.</span></header><Login /></>} />
      <Route path="/" element={<RedireccionInicio />} />
      <Route element={<RutaProtegida />}>
        <Route element={<Marco />}>
          <Route path="/sin-permiso" element={<SinPermiso />} />
          <Route path="/catalogo" element={<Catalogo />} />
          <Route path="/cursos/:id" element={<DetalleCurso />} />
          <Route element={<RutaProtegida roles={['teacher']} />}>
            <Route path="/profesor" element={<Navigate to="/profesor/cursos" replace />} />
            <Route path="/profesor/cursos" element={<MisCursos />} />
            <Route path="/profesor/cursos/nuevo" element={<NuevoCurso />} />
            <Route path="/profesor/cursos/:id/editar" element={<EditorCurso />} />
            <Route path="/profesor/*" element={<Inicio modulo="Mis cursos" />} />
          </Route>
          <Route element={<RutaProtegida roles={['student']} />}>
            <Route path="/mis-cursos" element={<Inicio modulo="Mi aprendizaje" />} />
            <Route path="/aprender/:cursoId" element={<Inicio modulo="Mi aprendizaje" />} />
            <Route path="/quiz/:quizId" element={<Inicio modulo="Evaluación" />} />
          </Route>
          <Route element={<RutaProtegida roles={['admin']} />}>
            <Route path="/admin/*" element={<Inicio modulo="Administración" />} />
          </Route>
        </Route>
      </Route>
      <Route path="*" element={<NoEncontrado />} />
    </Routes>
    <footer className="pie"><span>MOOC · Un espacio para aprender y compartir.</span><span>Versión {__APP_COMMIT__}</span></footer>
  </div>
}
