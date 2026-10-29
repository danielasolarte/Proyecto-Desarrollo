// Carga directa de archivos al bucket (multipart reanudable).
//
// El archivo NO pasa por la API: la API abre una sesión de carga, entrega una
// URL prefirmada por cada parte y, al final, confirma la carga y encola el
// procesamiento (antivirus y, según el tipo, HLS o conversión a PDF).
import { cliente } from './cliente'

export interface SesionCarga {
  id: string
  resource_id: string
  status: 'initiated' | 'uploading' | 'completed' | 'aborted' | 'expired'
  expires_at: string
}
export interface ParteSubida { part_number: number; etag: string; size_bytes: number }
export interface Asset { id: string; resource_id: string; kind: string; original_size_bytes?: number }
/** Respuesta de GET /resources/:id/media. Los derivados aparecen cuando el worker termina. */
export interface EstadoMedia extends Asset {
  original_storage_key?: string
  hls_manifest_key?: string
  converted_pdf_key?: string
  duration_seconds?: number
}

export const apiMedia = {
  iniciar: (resourceId: string, datos: { mime_type: string; size_bytes: number; checksum_sha256?: string }) =>
    cliente<{ upload_session: SesionCarga; recommended_part_bytes: number }>(
      `/resources/${encodeURIComponent(resourceId)}/uploads/initiate`, { method: 'POST', body: datos }),
  partes: (sesionId: string) =>
    cliente<{ parts: ParteSubida[] | null }>(`/uploads/${encodeURIComponent(sesionId)}/parts`),
  urlParte: (sesionId: string, n: number) =>
    cliente<{ url: string }>(`/uploads/${encodeURIComponent(sesionId)}/parts/${n}/url`),
  completar: (sesionId: string) =>
    cliente<Asset>(`/uploads/${encodeURIComponent(sesionId)}/complete`, { method: 'POST' }),
  estado: (resourceId: string) =>
    cliente<EstadoMedia>(`/resources/${encodeURIComponent(resourceId)}/media`),
  /** URL de lectura prefirmada: manifiesto HLS (video/audio), PDF convertido (presentación) u original. */
  reproduccion: (resourceId: string) =>
    cliente<{ url: string }>(`/resources/${encodeURIComponent(resourceId)}/playback`),
}

export const PARTE_POR_DEFECTO = 8 * 1024 * 1024

/** Tipos de archivo aceptados por tipo de recurso (atributo accept del input). */
export const aceptaPorTipo: Record<string, string> = {
  video: 'video/*',
  audio: 'audio/*',
  image: 'image/*',
  pdf: 'application/pdf',
  presentation: '.ppt,.pptx,.odp,application/vnd.ms-powerpoint,application/vnd.openxmlformats-officedocument.presentationml.presentation,application/vnd.oasis.opendocument.presentation',
  file: '',
}

/** Números de parte (1..n) que faltan por subir. */
export function partesPendientes(tamano: number, tamanoParte: number, subidas: number[]): number[] {
  const total = Math.max(1, Math.ceil(tamano / tamanoParte))
  const listas = new Set(subidas)
  const faltan: number[] = []
  for (let n = 1; n <= total; n++) if (!listas.has(n)) faltan.push(n)
  return faltan
}

export function formatoTamano(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

// Para reanudar: se recuerda en este navegador la sesión de carga de cada
// recurso junto con la "huella" del archivo (nombre, tamaño, fecha). Si el
// profesor vuelve a elegir el mismo archivo, se suben solo las partes que
// faltan. localStorage puede fallar (modo privado): en ese caso no se reanuda.
export interface CargaGuardada { sesionId: string; huella: string; tamanoParte: number }
const clave = (resourceId: string) => `mooc:carga:${resourceId}`

export function huella(f: File): string {
  return `${f.name}|${f.size}|${f.lastModified}`
}
export function leerCarga(resourceId: string): CargaGuardada | null {
  try {
    const v = localStorage.getItem(clave(resourceId))
    return v ? JSON.parse(v) as CargaGuardada : null
  } catch { return null }
}
export function guardarCarga(resourceId: string, c: CargaGuardada) {
  try { localStorage.setItem(clave(resourceId), JSON.stringify(c)) } catch { /* sin reanudación */ }
}
export function olvidarCarga(resourceId: string) {
  try { localStorage.removeItem(clave(resourceId)) } catch { /* nada */ }
}

/** SHA-256 en hexadecimal, solo para archivos medianos (en uno grande tarda y consume memoria). */
export const LIMITE_CHECKSUM = 200 * 1024 * 1024
export async function sha256Hex(f: File): Promise<string | undefined> {
  if (f.size > LIMITE_CHECKSUM || !globalThis.crypto?.subtle) return undefined
  try {
    const digest = await crypto.subtle.digest('SHA-256', await f.arrayBuffer())
    return Array.from(new Uint8Array(digest)).map(b => b.toString(16).padStart(2, '0')).join('')
  } catch { return undefined }
}

/**
 * Sube una parte con PUT a la URL prefirmada. Va directo al bucket, sin
 * cookies ni CSRF (la autorización está en la firma de la URL). Usa XHR para
 * poder informar el avance byte a byte.
 */
export function subirParte(url: string, cuerpo: Blob, alAvanzar: (bytes: number) => void, senal: AbortSignal): Promise<void> {
  return new Promise((resolver, rechazar) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', url)
    xhr.upload.onprogress = e => alAvanzar(e.loaded)
    xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolver() : rechazar(new Error(`El almacenamiento rechazó la parte (HTTP ${xhr.status}).`)))
    xhr.onerror = () => rechazar(new Error('Se perdió la conexión con el almacenamiento.'))
    xhr.onabort = () => rechazar(new DOMException('Carga pausada', 'AbortError'))
    senal.addEventListener('abort', () => xhr.abort(), { once: true })
    xhr.send(cuerpo)
  })
}
