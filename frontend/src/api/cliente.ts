export class ErrorAPI extends Error {
  constructor(public status: number, message: string, public code?: string) {
    super(message)
    this.name = 'ErrorAPI'
  }
}

type Opciones = Omit<RequestInit, 'body' | 'credentials'> & { body?: unknown }
export const SESION_EXPIRADA = 'mooc:sesion-expirada'

function cookie(nombre: string): string | undefined {
  const valor = document.cookie.split('; ').find(item => item.startsWith(`${nombre}=`))
  return valor ? decodeURIComponent(valor.substring(nombre.length + 1)) : undefined
}

// Todas las pantallas usan este cliente. La sesión vive en una cookie HttpOnly.
export async function cliente<T>(ruta: string, opciones: Opciones = {}): Promise<T> {
  if (!ruta.startsWith('/') || ruta.startsWith('//') || ruta.includes('..')) {
    throw new Error('Usa una ruta relativa al API, por ejemplo /me.')
  }
  const { body, ...init } = opciones
  const method = (init.method || 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (body !== undefined) headers.set('Content-Type', 'application/json')
  if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(method)) {
    const csrf = cookie('mooc_csrf')
    if (csrf) headers.set('X-CSRF-Token', csrf)
  }
  let respuesta: Response
  try {
    respuesta = await fetch(`/api/v1${ruta}`, {
      ...init, method, headers, credentials: 'include',
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new ErrorAPI(0, 'No pudimos conectar con el servidor. Intenta de nuevo.')
  }
  const texto = await respuesta.text()
  let datos: unknown
  try { datos = texto ? JSON.parse(texto) : undefined } catch {
    throw new ErrorAPI(respuesta.status, 'El servidor devolvió una respuesta inesperada.')
  }
  if (!respuesta.ok) {
    const contenido = datos as { error?: { message?: string; code?: string }; message?: string; code?: string } | undefined
    const detalle = contenido?.error || contenido
    if (respuesta.status === 401 && !ruta.startsWith('/auth/')) {
      window.dispatchEvent(new Event(SESION_EXPIRADA))
    }
    throw new ErrorAPI(respuesta.status, detalle?.message || 'No se pudo completar la solicitud.', detalle?.code)
  }
  return datos as T
}
