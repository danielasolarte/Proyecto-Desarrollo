// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cliente, ErrorAPI, SESION_EXPIRADA } from './cliente'

afterEach(() => {
  vi.unstubAllGlobals()
  document.cookie = 'mooc_csrf=; Max-Age=0; Path=/'
})

describe('contrato HTTP de sesión y CSRF', () => {
  it.each(['POST', 'PUT', 'PATCH', 'DELETE'])('envía cookies y CSRF en %s y acepta 204', async method => {
    document.cookie = 'mooc_csrf=token-prueba; Path=/'
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    await expect(cliente('/recurso', { method, body: { dato: 1 } })).resolves.toBeUndefined()
    const [url, opciones] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/recurso')
    expect(opciones.credentials).toBe('include')
    expect(opciones.headers.get('X-CSRF-Token')).toBe('token-prueba')
    expect(opciones.body).toBe('{"dato":1}')
  })
  it('consulta con cookies sin enviar CSRF ni Authorization', async () => {
    document.cookie = 'mooc_csrf=token-prueba; Path=/'
    const fetchMock = vi.fn().mockResolvedValue(new Response('{"id":"1"}'))
    vi.stubGlobal('fetch', fetchMock)
    await expect(cliente('/me')).resolves.toEqual({ id: '1' })
    const opciones = fetchMock.mock.calls[0][1]
    expect(opciones.credentials).toBe('include')
    expect(opciones.headers.has('X-CSRF-Token')).toBe(false)
    expect(opciones.headers.has('Authorization')).toBe(false)
  })
  it('interpreta errores del backend y notifica una sesión expirada', async () => {
    const expirada = vi.fn()
    window.addEventListener(SESION_EXPIRADA, expirada, { once: true })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{"error":{"message":"no autenticado","code":"unauthorized"}}', { status: 401 })))
    await expect(cliente('/me')).rejects.toMatchObject({ status: 401, message: 'no autenticado', code: 'unauthorized' })
    expect(expirada).toHaveBeenCalledOnce()
  })
  it('presenta una respuesta HTML como error controlado', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('<html>Error del proxy</html>', { status: 502 })))
    await expect(cliente('/me')).rejects.toBeInstanceOf(ErrorAPI)
  })
  it('rechaza URLs externas antes de enviar cookies', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    await expect(cliente('//otro-dominio/me')).rejects.toThrow('ruta relativa')
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
