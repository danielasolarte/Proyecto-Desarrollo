import { useCallback, useEffect, useId, useState, type FormEvent, type ReactNode } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import {
  api, esMultimedia, mensaje, nombreTipo, problemasDePublicacion, siguientePosicion,
  type Arbol, type Curso, type Recurso, type TipoRecurso,
} from '../../api/cursos'
import { useSesion } from '../../auth/Sesion'
import { Cargando, ErrorEstado, Exito, Vacio } from '../../componentes/estados/Estados'
import { RanuraMultimedia, RanuraQuiz } from './ranuras'
import '../catalogo/cursos.css'

const TIPOS: TipoRecurso[] = ['rich_text', 'video', 'pdf', 'image', 'audio', 'presentation', 'file', 'link', 'iframe', 'quiz']

// Editor del curso del profesor: metadatos, módulos, unidades y recursos de
// la versión en borrador, y publicación con la lista de problemas que
// devuelve la API. Toda acción recarga el árbol desde el servidor para que
// la pantalla nunca muestre algo distinto a lo guardado.
export function EditorCurso() {
  const { id = '' } = useParams()
  const location = useLocation()
  const { usuario } = useSesion()
  const [curso, setCurso] = useState<Curso | null>(null)
  const [arbol, setArbol] = useState<Arbol | null>(null)
  const [problemas, setProblemas] = useState<string[]>([])
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState('')
  const [aviso, setAviso] = useState(location.state?.creado ? 'Curso creado. Ahora agrega módulos, unidades y recursos.' : '')
  const [errorAccion, setErrorAccion] = useState('')
  const [ocupado, setOcupado] = useState(false)

  const cargar = useCallback(async (mostrarCarga = true) => {
    if (mostrarCarga) setCargando(true)
    setError('')
    try {
      const [c, p] = await Promise.all([api.curso(id), api.previsualizar(id)])
      setCurso(c)
      setArbol(p.tree)
      setProblemas(p.problems || [])
    } catch (e) { setError(mensaje(e, 'No pudimos cargar el curso.')) } finally { setCargando(false) }
  }, [id])

  useEffect(() => { void cargar() }, [cargar])

  // Ejecuta una acción, muestra su error si falla y recarga el árbol.
  async function hacer(accion: () => Promise<unknown>, exito?: string) {
    setOcupado(true)
    setErrorAccion('')
    setAviso('')
    try {
      await accion()
      if (exito) setAviso(exito)
      await cargar(false)
      return true
    } catch (e) {
      setErrorAccion(mensaje(e))
      return false
    } finally { setOcupado(false) }
  }

  if (cargando) return <Cargando mensaje="Cargando el editor…" />
  if (error || !curso || !arbol) return <ErrorEstado mensaje={error || 'No pudimos cargar el curso.'} reintentar={() => void cargar()} />
  if (usuario && usuario.role !== 'admin' && curso.teacher_id !== usuario.id) {
    return <section className="pagina-mensaje"><span className="etiqueta texto-verde">ACCESO RESTRINGIDO</span>
      <h1>Este curso pertenece a otro profesor.</h1><p>Solo puedes editar tus propios cursos.</p>
      <Link className="boton-principal" to="/profesor/cursos">Volver a mis cursos</Link></section>
  }

  const editable = arbol.version.status === 'draft'
  const version = arbol.version

  return <>
    <div className="ruta-seccion"><Link to="/profesor/cursos">MIS CURSOS</Link> <span>/</span> {(version.title || 'CURSO').toLocaleUpperCase('es')}</div>
    <section className="bienvenida encabezado-con-accion">
      <div><span className="etiqueta texto-verde">EDITOR · VERSIÓN {version.version_number}</span><h1>{version.title || 'Curso sin título'}</h1>
        <p>{editable ? 'Borrador: los cambios no son visibles para los estudiantes hasta que publiques.' : 'Esta versión está publicada y no se puede modificar.'}</p></div>
      {curso.published_version_id && <Link className="boton-secundario" to={`/cursos/${id}`}>Ver curso publicado</Link>}
    </section>

    <div aria-live="polite">
      {aviso && <Exito>{aviso}</Exito>}
      {errorAccion && <ErrorEstado mensaje={errorAccion} />}
    </div>

    {!editable && <BorradorDeActualizacion ocupado={ocupado} crear={tipo => void hacer(() => api.crearBorrador(id, tipo), 'Borrador creado: ya puedes editar.')} />}

    {editable && <Metadatos version={version} ocupado={ocupado}
      guardar={datos => hacer(() => api.actualizarVersion(version.id, datos), 'Datos del curso guardados.')} />}

    <section className="tarjeta bloque">
      <div className="tarjeta-cabecera"><h2>Estructura</h2><span className="insignia">{arbol.modules.length} {arbol.modules.length === 1 ? 'módulo' : 'módulos'}</span></div>
      {arbol.modules.length === 0 && <Vacio titulo="El curso no tiene módulos">{editable && <p>Agrega el primero abajo.</p>}</Vacio>}
      <ol className="editor-modulos">
        {arbol.modules.map(m => <li key={m.module.id} className="editor-modulo">
          <FilaEditable etiqueta="Módulo" titulo={m.module.title} editable={editable} ocupado={ocupado}
            renombrar={t => hacer(() => api.editarModulo(m.module.id, t, m.module.position), 'Módulo actualizado.')}
            eliminar={() => hacer(() => api.borrarModulo(m.module.id), 'Módulo eliminado.')} />
          <ol className="editor-unidades">
            {m.units.map(u => <li key={u.unit.id} className="editor-unidad">
              <FilaEditable etiqueta="Unidad" titulo={u.unit.title} editable={editable} ocupado={ocupado}
                renombrar={t => hacer(() => api.editarUnidad(u.unit.id, t, u.unit.position), 'Unidad actualizada.')}
                eliminar={() => hacer(() => api.borrarUnidad(u.unit.id), 'Unidad eliminada.')} />
              {u.resources.length === 0 && <p className="ranura-nota">Esta unidad todavía no tiene recursos.</p>}
              <ul className="editor-recursos">
                {u.resources.map(r => <li key={r.id}><RecursoEditor recurso={r} editable={editable} ocupado={ocupado} hacer={hacer} alCambiar={() => void cargar(false)} /></li>)}
              </ul>
              {editable && <NuevoRecurso ocupado={ocupado}
                crear={datos => hacer(() => api.crearRecurso(u.unit.id, { ...datos, position: siguientePosicion(u.resources), downloadable: false }), 'Recurso agregado.')} />}
            </li>)}
          </ol>
          {editable && <AgregarTitulo etiqueta="Nueva unidad" ocupado={ocupado}
            agregar={t => hacer(() => api.crearUnidad(m.module.id, t, siguientePosicion(m.units.map(x => x.unit))), 'Unidad agregada.')} />}
        </li>)}
      </ol>
      {editable && <div className="bloque-pie"><AgregarTitulo etiqueta="Nuevo módulo" ocupado={ocupado}
        agregar={t => hacer(() => api.crearModulo(version.id, t, siguientePosicion(arbol.modules.map(x => x.module))), 'Módulo agregado.')} /></div>}
    </section>

    {editable && <Publicacion problemas={problemas} ocupado={ocupado} publicar={async () => {
      setOcupado(true)
      setErrorAccion('')
      setAviso('')
      try {
        const v = await api.publicar(id)
        setAviso(`Curso publicado (versión ${v.version_number}). Ya aparece en el catálogo.`)
        await cargar(false)
      } catch (e) {
        const lista = problemasDePublicacion(e)
        if (lista) setProblemas(lista)
        setErrorAccion(lista ? 'No se pudo publicar: corrige los problemas de la lista.' : mensaje(e, 'No se pudo publicar.'))
      } finally { setOcupado(false) }
    }} />}
  </>
}

function BorradorDeActualizacion({ ocupado, crear }: { ocupado: boolean; crear: (tipo: 'minor' | 'major') => void }) {
  return <section className="tarjeta bloque"><div className="tarjeta-cabecera"><h2>Editar un curso publicado</h2></div>
    <div className="bloque-cuerpo"><p>Para hacer cambios se crea un borrador a partir de la versión publicada. Los estudiantes siguen viendo la versión actual hasta que publiques el borrador, y conservan su progreso.</p>
      <div className="acciones">
        <button className="boton-principal" disabled={ocupado} onClick={() => crear('minor')}>Crear borrador (cambio menor)</button>
        <button className="boton-secundario" disabled={ocupado} onClick={() => crear('major')}>Crear borrador (cambio mayor)</button>
      </div></div></section>
}

function Metadatos({ version, ocupado, guardar }: { version: Arbol['version']; ocupado: boolean; guardar: (d: { title: string; summary: string; category: string }) => Promise<boolean> }) {
  const [titulo, setTitulo] = useState(version.title)
  const [resumen, setResumen] = useState(version.summary)
  const [categoria, setCategoria] = useState(version.category)
  useEffect(() => { setTitulo(version.title); setResumen(version.summary); setCategoria(version.category) }, [version])
  const cambio = titulo !== version.title || resumen !== version.summary || categoria !== version.category
  function enviar(e: FormEvent<HTMLFormElement>) { e.preventDefault(); void guardar({ title: titulo.trim(), summary: resumen.trim(), category: categoria.trim() }) }
  return <section className="tarjeta bloque"><div className="tarjeta-cabecera"><h2>Datos del curso</h2></div>
    <form className="bloque-cuerpo" onSubmit={enviar}>
      <label htmlFor="m-titulo">Título</label><input id="m-titulo" required value={titulo} onChange={e => setTitulo(e.target.value)} />
      <label htmlFor="m-resumen">Resumen</label><textarea id="m-resumen" rows={3} value={resumen} onChange={e => setResumen(e.target.value)} />
      <label htmlFor="m-categoria">Categoría</label><input id="m-categoria" value={categoria} onChange={e => setCategoria(e.target.value)} />
      <div className="acciones"><button className="boton-principal" type="submit" disabled={ocupado || !cambio || !titulo.trim()}>Guardar datos</button></div>
    </form></section>
}

function FilaEditable({ etiqueta, titulo, editable, ocupado, renombrar, eliminar }: {
  etiqueta: string; titulo: string; editable: boolean; ocupado: boolean
  renombrar: (t: string) => Promise<boolean>; eliminar: () => Promise<boolean>
}) {
  const [editando, setEditando] = useState(false)
  const [valor, setValor] = useState(titulo)
  const idCampo = useId()
  if (editando) return <form className="fila-editable" onSubmit={async e => { e.preventDefault(); if (await renombrar(valor.trim())) setEditando(false) }}>
    <label className="oculto" htmlFor={idCampo}>Nuevo título de {etiqueta.toLowerCase()}</label>
    <input id={idCampo} value={valor} onChange={e => setValor(e.target.value)} autoFocus required />
    <button className="boton-secundario" type="submit" disabled={ocupado || !valor.trim()}>Guardar</button>
    <button className="boton-texto" type="button" onClick={() => { setValor(titulo); setEditando(false) }}>Cancelar</button>
  </form>
  return <div className="fila-editable"><span className="tipo-recurso">{etiqueta}</span><strong>{titulo || 'Sin título'}</strong>
    {editable && <span className="fila-acciones">
      <button className="boton-texto" onClick={() => setEditando(true)} disabled={ocupado} aria-label={`Renombrar ${etiqueta.toLowerCase()} ${titulo}`}>Renombrar</button>
      <BotonEliminar ocupado={ocupado} descripcion={`${etiqueta.toLowerCase()} ${titulo}`} eliminar={eliminar} />
    </span>}</div>
}

// Eliminar pide confirmación en la misma pantalla (sin ventanas del navegador).
function BotonEliminar({ ocupado, descripcion, eliminar }: { ocupado: boolean; descripcion: string; eliminar: () => Promise<boolean> }) {
  const [confirmando, setConfirmando] = useState(false)
  if (!confirmando) return <button className="boton-texto peligro" onClick={() => setConfirmando(true)} disabled={ocupado} aria-label={`Eliminar ${descripcion}`}>Eliminar</button>
  return <span className="confirmar" role="group" aria-label={`Confirmar eliminar ${descripcion}`}>¿Eliminar?
    <button className="boton-texto peligro" onClick={() => void eliminar().then(() => setConfirmando(false))} disabled={ocupado}>Sí, eliminar</button>
    <button className="boton-texto" onClick={() => setConfirmando(false)}>No</button></span>
}

function AgregarTitulo({ etiqueta, ocupado, agregar }: { etiqueta: string; ocupado: boolean; agregar: (t: string) => Promise<boolean> }) {
  const [valor, setValor] = useState('')
  const idCampo = useId()
  return <form className="agregar" onSubmit={async e => { e.preventDefault(); if (await agregar(valor.trim())) setValor('') }}>
    <label className="oculto" htmlFor={idCampo}>{etiqueta}</label>
    <input id={idCampo} placeholder={etiqueta} value={valor} onChange={e => setValor(e.target.value)} />
    <button className="boton-secundario" type="submit" disabled={ocupado || !valor.trim()}>Agregar</button>
  </form>
}

function NuevoRecurso({ ocupado, crear }: { ocupado: boolean; crear: (d: { type: TipoRecurso; title: string; visible: boolean; required: boolean; external_url?: string }) => Promise<boolean> }) {
  const [abierto, setAbierto] = useState(false)
  const [tipo, setTipo] = useState<TipoRecurso>('rich_text')
  const [titulo, setTitulo] = useState('')
  const [url, setUrl] = useState('')
  const [obligatorio, setObligatorio] = useState(true)
  const necesitaUrl = tipo === 'link' || tipo === 'iframe'
  const base = useId()
  if (!abierto) return <button className="boton-texto" onClick={() => setAbierto(true)} disabled={ocupado}>＋ Agregar recurso</button>
  return <form className="nuevo-recurso" onSubmit={async e => {
    e.preventDefault()
    const ok = await crear({ type: tipo, title: titulo.trim(), visible: true, required: obligatorio, external_url: necesitaUrl ? url.trim() : undefined })
    if (ok) { setTitulo(''); setUrl(''); setAbierto(false) }
  }}>
    <div className="campo"><label htmlFor={`${base}-tipo`}>Tipo</label>
      <select id={`${base}-tipo`} value={tipo} onChange={e => setTipo(e.target.value as TipoRecurso)}>{TIPOS.map(t => <option key={t} value={t}>{nombreTipo[t]}</option>)}</select></div>
    <div className="campo"><label htmlFor={`${base}-titulo`}>Título</label><input id={`${base}-titulo`} required value={titulo} onChange={e => setTitulo(e.target.value)} /></div>
    {necesitaUrl && <div className="campo"><label htmlFor={`${base}-url`}>Dirección (https)</label><input id={`${base}-url`} type="url" required pattern="https://.*" value={url} onChange={e => setUrl(e.target.value)} /></div>}
    <label className="casilla"><input type="checkbox" checked={obligatorio} onChange={e => setObligatorio(e.target.checked)} /> Obligatorio para completar el curso</label>
    <div className="acciones"><button className="boton-secundario" type="submit" disabled={ocupado || !titulo.trim()}>Agregar recurso</button>
      <button className="boton-texto" type="button" onClick={() => setAbierto(false)}>Cancelar</button></div>
  </form>
}

function RecursoEditor({ recurso: r, editable, ocupado, hacer, alCambiar }: {
  recurso: Recurso; editable: boolean; ocupado: boolean
  hacer: (a: () => Promise<unknown>, exito?: string) => Promise<boolean>; alCambiar: () => void
}) {
  let detalle: ReactNode = null
  if (r.type === 'rich_text') detalle = <EditorTexto recurso={r} editable={editable} ocupado={ocupado} hacer={hacer} />
  else if (esMultimedia(r.type)) detalle = <RanuraMultimedia recurso={r} alCambiar={alCambiar} />
  else if (r.type === 'quiz') detalle = <RanuraQuiz recurso={r} />
  else if (r.external_url) detalle = <a className="ranura-nota" href={r.external_url} target="_blank" rel="noreferrer">{r.external_url}</a>
  return <div className="recurso">
    <div className="fila-editable"><span className="tipo-recurso">{nombreTipo[r.type] || r.type}</span><strong>{r.title}</strong>
      {r.required && <span className="obligatorio">obligatorio</span>}
      {!r.visible && <span className="obligatorio">oculto</span>}
      {editable && <span className="fila-acciones"><BotonEliminar ocupado={ocupado} descripcion={`recurso ${r.title}`} eliminar={() => hacer(() => api.borrarRecurso(r.id), 'Recurso eliminado.')} /></span>}
    </div>
    {detalle}
  </div>
}

function EditorTexto({ recurso: r, editable, ocupado, hacer }: {
  recurso: Recurso; editable: boolean; ocupado: boolean; hacer: (a: () => Promise<unknown>, exito?: string) => Promise<boolean>
}) {
  const [texto, setTexto] = useState(r.content_draft_markdown ?? r.content_markdown ?? '')
  const publicado = (r.content_markdown ?? '') === texto && !!r.content_markdown
  if (!editable) return <p className="ranura-nota">{r.content_markdown ? 'Texto publicado.' : 'Sin texto publicado.'}</p>
  return <div className="ranura">
    <label htmlFor={`txt-${r.id}`}>Contenido (Markdown)</label>
    <textarea id={`txt-${r.id}`} rows={5} value={texto} onChange={e => setTexto(e.target.value)} />
    <div className="acciones">
      <button className="boton-secundario" disabled={ocupado || !texto.trim() || publicado}
        onClick={() => void hacer(async () => { await api.guardarTexto(r.id, texto); await api.publicarTexto(r.id) }, 'Texto guardado y publicado en el borrador.')}>Guardar texto</button>
      <span className="ranura-nota">{publicado ? 'Guardado.' : r.content_markdown ? 'Hay cambios sin guardar.' : 'Aún no tiene texto guardado.'}</span>
    </div>
  </div>
}

function Publicacion({ problemas, ocupado, publicar }: { problemas: string[]; ocupado: boolean; publicar: () => Promise<void> }) {
  return <section className="tarjeta bloque" aria-labelledby="titulo-publicar">
    <div className="tarjeta-cabecera"><h2 id="titulo-publicar">Publicación</h2>
      <span className={`estado-curso ${problemas.length ? 'estado-draft' : 'estado-published'}`}>{problemas.length ? `${problemas.length} pendiente${problemas.length === 1 ? '' : 's'}` : 'Listo para publicar'}</span></div>
    <div className="bloque-cuerpo">
      {problemas.length > 0
        ? <><p>Para publicar, la API exige que se cumplan estas reglas:</p><ul className="problemas">{problemas.map(p => <li key={p}>{p}</li>)}</ul></>
        : <p>El curso cumple las reglas de publicación.</p>}
      <button className="boton-principal" disabled={ocupado} onClick={() => void publicar()}>{ocupado ? 'Procesando…' : 'Publicar curso'}<span aria-hidden="true">→</span></button>
    </div>
  </section>
}
