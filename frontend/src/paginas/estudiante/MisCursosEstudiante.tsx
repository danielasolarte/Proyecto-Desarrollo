import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  api,
  mensaje,
  type Inscripcion,
  type Temario,
} from '../../api/cursos'
import {
  Cargando,
  ErrorEstado,
  Vacio,
} from '../../componentes/estados/Estados'
import '../catalogo/cursos.css'

type CursoInscrito = {
  inscripcion: Inscripcion
  temario: Temario
}

export function MisCursosEstudiante() {
  const [cursos, setCursos] = useState<CursoInscrito[]>([])
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState('')

  const cargar = useCallback(async () => {
    setCargando(true)
    setError('')

    try {
      const inscripciones = await api.misInscripciones()

      const activas = (inscripciones ?? []).filter(
        inscripcion => inscripcion.status === 'active',
      )

      const resultados = await Promise.all(
        activas.map(async inscripcion => {
          const temario = await api.temario(inscripcion.course_id)

          return {
            inscripcion,
            temario,
          }
        }),
      )

      setCursos(resultados)
    } catch (e) {
      setError(
        mensaje(e, 'No pudimos cargar tus cursos inscritos.'),
      )
    } finally {
      setCargando(false)
    }
  }, [])

  useEffect(() => {
    void cargar()
  }, [cargar])

  if (cargando) {
    return <Cargando mensaje="Cargando tus cursos…" />
  }

  if (error) {
    return (
      <ErrorEstado
        mensaje={error}
        reintentar={() => void cargar()}
      />
    )
  }

  return (
    <>
      <div className="ruta-seccion">
        MI ESPACIO <span>/</span> MIS CURSOS
      </div>

      <section className="bienvenida encabezado-con-accion">
        <div>
          <span className="etiqueta texto-verde">
            MI APRENDIZAJE
          </span>

          <h1>Mis cursos</h1>

          <p>
            Continúa aprendiendo desde los cursos en los que estás
            inscrito.
          </p>
        </div>

        <Link className="boton-secundario" to="/catalogo">
          Explorar cursos
        </Link>
      </section>

      {cursos.length === 0 ? (
        <section className="tarjeta">
          <Vacio titulo="Todavía no tienes cursos inscritos">
            <p>
              Explora el catálogo y elige un curso para comenzar.
            </p>

            <Link className="boton-principal" to="/catalogo">
              Explorar cursos
            </Link>
          </Vacio>
        </section>
      ) : (
        <ul className="rejilla-cursos">
          {cursos.map(({ inscripcion, temario }) => {
            const totalRecursos = temario.modules.reduce(
              (total, modulo) =>
                total +
                modulo.units.reduce(
                  (subtotal, unidad) =>
                    subtotal + unidad.resources.length,
                  0,
                ),
              0,
            )

            return (
              <li
                key={inscripcion.id}
                className="tarjeta tarjeta-curso"
              >
                {temario.category && (
                  <span className="insignia">
                    {temario.category}
                  </span>
                )}

                <h2>{temario.title}</h2>

                <p>{temario.summary}</p>

                <p className="detalle-datos">
                  {temario.modules.length}{' '}
                  {temario.modules.length === 1
                    ? 'módulo'
                    : 'módulos'}
                  {' · '}
                  {totalRecursos}{' '}
                  {totalRecursos === 1
                    ? 'recurso'
                    : 'recursos'}
                </p>

                <Link
                  className="enlace-accion"
                  to={`/aprender/${temario.course_id}`}
                >
                  Continuar curso →
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </>
  )
}