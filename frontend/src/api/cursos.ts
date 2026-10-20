// Contrato de los módulos de cursos y catálogo (Daniela). Todas las llamadas
// pasan por el cliente compartido de Samara (cookies + CSRF).
import { cliente, ErrorAPI } from './cliente'

export type TipoRecurso =
  | 'rich_text' | 'video' | 'pdf' | 'image' | 'audio' | 'presentation' | 'file' | 'link' | 'iframe' | 'quiz'
export type EstadoProcesamiento = 'pending' | 'processing' | 'ready' | 'failed'
export type EstadoCurso = 'draft' | 'published' | 'published_with_draft'

export interface CursoCatalogo {
  course_id: string; slug: string; title: string; summary: string; category: string; published_at: string
}
export interface PaginaCatalogo { courses: CursoCatalogo[]; next_cursor: string }

export interface Temario {
  course_id: string; slug: string; teacher_id: string; title: string; summary: string; category: string
  published_at?: string
  modules: { id: string; title: string; units: { id: string; title: string; resources: { id: string; title: string; type: TipoRecurso; required: boolean }[] }[] }[]
}

export interface Inscripcion { id: string; course_id: string; status: 'active' | 'withdrawn' }

export interface CursoProfesor {
  id: string; slug: string; title: string; summary: string; category: string; status: EstadoCurso
  published_version_id?: string; latest_draft_version_id?: string; published_at?: string
  created_at: string; updated_at: string
}
export interface PaginaCursosProfesor { courses: CursoProfesor[]; next_cursor: string }

export interface Curso {
  id: string; teacher_id: string; slug: string
  published_version_id?: string; latest_draft_version_id?: string
}
export interface Version {
  id: string; course_id: string; version_number: number; status: 'draft' | 'published' | 'superseded'
  title: string; summary: string; category: string
}
export interface Modulo { id: string; title: string; position: number }
export interface Unidad { id: string; title: string; position: number }
export interface Recurso {
  id: string; title: string; type: TipoRecurso; position: number
  visible: boolean; required: boolean; downloadable: boolean
  processing_status?: EstadoProcesamiento
  content_markdown?: string; content_draft_markdown?: string; external_url?: string
}
export interface Arbol {
  version: Version
  modules: { module: Modulo; units: { unit: Unidad; resources: Recurso[] }[] }[]
}
export interface Previsualizacion { tree: Arbol; problems: string[] }

const q = (params: Record<string, string | number | undefined>) => {
  const s = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') s.set(k, String(v))
  const t = s.toString()
  return t ? `?${t}` : ''
}

export const api = {
  catalogo: (p: { q?: string; category?: string; cursor?: string; limit?: number }) =>
    cliente<PaginaCatalogo>(`/catalog${q(p)}`),
  temario: (id: string) => cliente<Temario>(`/courses/${encodeURIComponent(id)}/outline`),
  misInscripciones: () => cliente<Inscripcion[] | null>('/me/enrollments'),
  inscribirme: (id: string) => cliente<Inscripcion>(`/courses/${encodeURIComponent(id)}/enrollments`, { method: 'POST' }),

  misCursos: (cursor?: string) => cliente<PaginaCursosProfesor>(`/me/courses${q({ cursor, limit: 50 })}`),
  crearCurso: (datos: { title: string; summary: string; category: string }) =>
    cliente<{ course: Curso; version: Version }>('/courses', { method: 'POST', body: datos }),
  curso: (id: string) => cliente<Curso>(`/courses/${encodeURIComponent(id)}`),
  previsualizar: (id: string) => cliente<Previsualizacion>(`/courses/${encodeURIComponent(id)}/preview`),
  crearBorrador: (id: string, change_type: 'minor' | 'major') =>
    cliente<Version>(`/courses/${encodeURIComponent(id)}/draft`, { method: 'POST', body: { change_type } }),
  publicar: (id: string) => cliente<Version>(`/courses/${encodeURIComponent(id)}/publish`, { method: 'POST' }),
  actualizarVersion: (vid: string, datos: { title: string; summary: string; category: string }) =>
    cliente<Version>(`/versions/${vid}`, { method: 'PATCH', body: datos }),

  crearModulo: (vid: string, title: string, position: number) =>
    cliente<Modulo>(`/versions/${vid}/modules`, { method: 'POST', body: { title, position } }),
  editarModulo: (id: string, title: string, position: number) =>
    cliente<Modulo>(`/modules/${id}`, { method: 'PATCH', body: { title, position } }),
  borrarModulo: (id: string) => cliente<void>(`/modules/${id}`, { method: 'DELETE' }),

  crearUnidad: (mid: string, title: string, position: number) =>
    cliente<Unidad>(`/modules/${mid}/units`, { method: 'POST', body: { title, position } }),
  editarUnidad: (id: string, title: string, position: number) =>
    cliente<Unidad>(`/units/${id}`, { method: 'PATCH', body: { title, position } }),
  borrarUnidad: (id: string) => cliente<void>(`/units/${id}`, { method: 'DELETE' }),

  crearRecurso: (uid: string, datos: { type: TipoRecurso; title: string; position: number; visible: boolean; required: boolean; downloadable: boolean; external_url?: string }) =>
    cliente<Recurso>(`/units/${uid}/resources`, { method: 'POST', body: datos }),
  borrarRecurso: (id: string) => cliente<void>(`/resources/${id}`, { method: 'DELETE' }),
  guardarTexto: (id: string, markdown: string) =>
    cliente<Recurso>(`/resources/${id}/content/draft`, { method: 'PUT', body: { markdown } }),
  publicarTexto: (id: string) => cliente<Recurso>(`/resources/${id}/content/publish`, { method: 'POST' }),
}

/** Siguiente posición libre (la API exige posiciones únicas por nivel). */
export function siguientePosicion(items: { position: number }[]): number {
  return items.reduce((max, i) => Math.max(max, i.position), 0) + 1
}

/**
 * La API rechaza la publicación con un 400 cuyo mensaje es
 * "error de validacion\n<problema>; <problema>; ...". Devuelve la lista.
 */
export function problemasDePublicacion(error: unknown): string[] | null {
  if (!(error instanceof ErrorAPI) || error.status !== 400) return null
  const lineas = error.message.split('\n').filter(l => l.trim() && l.trim() !== 'error de validacion')
  const problemas = lineas.join('; ').split(';').map(p => p.trim()).filter(Boolean)
  return problemas.length ? problemas : null
}

export function mensaje(error: unknown, porDefecto = 'No se pudo completar la acción.'): string {
  return error instanceof Error && error.message ? error.message : porDefecto
}

export const nombreTipo: Record<TipoRecurso, string> = {
  rich_text: 'Texto', video: 'Video', pdf: 'PDF', image: 'Imagen', audio: 'Audio',
  presentation: 'Presentación', file: 'Archivo', link: 'Enlace', iframe: 'Contenido embebido', quiz: 'Quiz',
}

export const nombreEstado: Record<EstadoCurso, string> = {
  draft: 'Borrador', published: 'Publicado', published_with_draft: 'Publicado con cambios sin publicar',
}

export function esMultimedia(t: TipoRecurso): boolean {
  return ['video', 'pdf', 'image', 'audio', 'presentation', 'file'].includes(t)
}
