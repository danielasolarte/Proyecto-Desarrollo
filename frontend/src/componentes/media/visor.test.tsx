// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { derivadoListo, VisorRecurso } from './VisorRecurso'

const hlsMock = vi.hoisted(() => {
  const instancia = { on: vi.fn(), loadSource: vi.fn(), attachMedia: vi.fn(), destroy: vi.fn() }
  const Hls = Object.assign(vi.fn(() => instancia), { isSupported: vi.fn(() => true), Events: { ERROR: 'hlsError' } })
  return { Hls, instancia }
})
vi.mock('hls.js', () => ({ default: hlsMock.Hls }))

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.restoreAllMocks(); vi.clearAllMocks(); hlsMock.Hls.isSupported.mockReturnValue(true) })

const json = (datos: unknown, status = 200) => new Response(JSON.stringify(datos), { status })

function api(rutas: Record<string, () => Response>) {
  const fetchMock = vi.fn(async (url: string) => {
    const r = rutas[url.replace(/\?.*$/, '')]
    return r ? r() : new Response('{"error":{"message":"no simulado"}}', { status: 500 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

const URL_FIRMADA = 'https://storage.googleapis.com/bucket/resources/r1/original?X-Goog-Signature=abc'

describe('derivadoListo', () => {
  it('exige el HLS en video y audio, y el PDF convertido en presentaciones', () => {
    expect(derivadoListo('video', { id: 'a', resource_id: 'r', kind: 'video', original_storage_key: 'o' })).toBe(false)
    expect(derivadoListo('video', { id: 'a', resource_id: 'r', kind: 'video', hls_manifest_key: 'h' })).toBe(true)
    expect(derivadoListo('audio', { id: 'a', resource_id: 'r', kind: 'audio', hls_manifest_key: 'h' })).toBe(true)
    expect(derivadoListo('presentation', { id: 'a', resource_id: 'r', kind: 'presentation', original_storage_key: 'o' })).toBe(false)
    expect(derivadoListo('presentation', { id: 'a', resource_id: 'r', kind: 'presentation', converted_pdf_key: 'p' })).toBe(true)
    expect(derivadoListo('pdf', { id: 'a', resource_id: 'r', kind: 'pdf', original_storage_key: 'o' })).toBe(true)
  })
})

describe('VisorRecurso', () => {
  it('no ofrece un video que todavía se está procesando', async () => {
    const f = api({
      '/api/v1/resources/r1/media': () => json({ id: 'a1', resource_id: 'r1', kind: 'video', original_storage_key: 'o' }),
    })
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Clase 1', type: 'video' }} />)
    expect(await screen.findByText(/se está procesando/)).toBeTruthy()
    expect(f.mock.calls.some(([u]) => String(u).includes('/playback'))).toBe(false)
    expect(document.querySelector('video')).toBeNull()
  })

  it('muestra el PDF con la URL de lectura del bucket', async () => {
    api({
      '/api/v1/resources/r1/media': () => json({ id: 'a1', resource_id: 'r1', kind: 'pdf', original_storage_key: 'o' }),
      '/api/v1/resources/r1/playback': () => json({ url: URL_FIRMADA }),
    })
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Lectura', type: 'pdf' }} />)
    const marco = await screen.findByTitle('Lectura')
    expect(marco.getAttribute('src')).toBe(URL_FIRMADA)
    expect(screen.getByText(/Abrir el documento/).getAttribute('href')).toBe(URL_FIRMADA)
  })

  it('muestra la imagen', async () => {
    api({
      '/api/v1/resources/r1/media': () => json({ id: 'a1', resource_id: 'r1', kind: 'image', original_storage_key: 'o' }),
      '/api/v1/resources/r1/playback': () => json({ url: URL_FIRMADA }),
    })
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Diagrama', type: 'image' }} />)
    expect((await screen.findByAltText('Diagrama')).getAttribute('src')).toBe(URL_FIRMADA)
  })

  it('avisa cuando el recurso no tiene archivo', async () => {
    api({ '/api/v1/resources/r1/media': () => json({ error: { message: 'media asset not found' } }, 404) })
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Clase 1', type: 'video' }} />)
    expect(await screen.findByText('Este recurso todavía no tiene archivo.')).toBeTruthy()
  })

  it('muestra el error y permite reintentar', async () => {
    let fallar = true
    api({
      '/api/v1/resources/r1/media': () => (fallar
        ? json({ error: { message: 'Servicio no disponible' } }, 503)
        : json({ id: 'a1', resource_id: 'r1', kind: 'file', original_storage_key: 'o' })),
      '/api/v1/resources/r1/playback': () => json({ url: URL_FIRMADA }),
    })
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Anexo', type: 'file' }} />)
    await screen.findByRole('alert')
    fallar = false
    fireEvent.click(screen.getByText('Intentar de nuevo'))
    expect((await screen.findByText(/Descargar el archivo/)).getAttribute('href')).toBe(URL_FIRMADA)
  })

  it('usa hls.js aunque el navegador diga que reproduce HLS nativo (Chrome reciente)', async () => {
    api({
      '/api/v1/resources/r1/media': () => json({ id: 'a1', resource_id: 'r1', kind: 'video', hls_manifest_key: 'h' }),
      '/api/v1/resources/r1/playback': () => json({ url: URL_FIRMADA }),
    })
    vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockReturnValue('maybe')
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Clase 1', type: 'video' }} />)
    await vi.waitFor(() => expect(hlsMock.instancia.loadSource).toHaveBeenCalledWith(URL_FIRMADA))
    const video = document.querySelector('video')!
    expect(hlsMock.instancia.attachMedia).toHaveBeenCalledWith(video)
    expect(video.getAttribute('src')).toBeNull()
  })

  it('reproduce de forma nativa cuando hls.js no es compatible (Safari en iPhone)', async () => {
    api({
      '/api/v1/resources/r1/media': () => json({ id: 'a1', resource_id: 'r1', kind: 'video', hls_manifest_key: 'h' }),
      '/api/v1/resources/r1/playback': () => json({ url: URL_FIRMADA }),
    })
    hlsMock.Hls.isSupported.mockReturnValue(false)
    vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockReturnValue('maybe')
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Clase 1', type: 'video' }} />)
    await vi.waitFor(() => expect(document.querySelector('video')?.getAttribute('src')).toBe(URL_FIRMADA))
    expect(hlsMock.instancia.loadSource).not.toHaveBeenCalled()
  })

  it('avisa cuando el navegador no tiene ni hls.js ni HLS nativo', async () => {
    api({
      '/api/v1/resources/r1/media': () => json({ id: 'a1', resource_id: 'r1', kind: 'video', hls_manifest_key: 'h' }),
      '/api/v1/resources/r1/playback': () => json({ url: URL_FIRMADA }),
    })
    hlsMock.Hls.isSupported.mockReturnValue(false)
    vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockReturnValue('')
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Clase 1', type: 'video' }} />)
    expect(await screen.findByText(/no puede reproducir el contenido/)).toBeTruthy()
    expect(document.querySelector('video')?.getAttribute('src')).toBeNull()
  })

  it('muestra un error cuando hls.js reporta un fallo fatal', async () => {
    api({
      '/api/v1/resources/r1/media': () => json({ id: 'a1', resource_id: 'r1', kind: 'audio', hls_manifest_key: 'h' }),
      '/api/v1/resources/r1/playback': () => json({ url: URL_FIRMADA }),
    })
    render(<VisorRecurso recurso={{ id: 'r1', title: 'Podcast', type: 'audio' }} />)
    await vi.waitFor(() => expect(hlsMock.instancia.on).toHaveBeenCalled())
    const [, manejador] = hlsMock.instancia.on.mock.calls[0] as [string, (e: string, d: { fatal: boolean }) => void]
    act(() => manejador('hlsError', { fatal: true }))
    expect(await screen.findByText(/No pudimos reproducir el contenido/)).toBeTruthy()
    expect(document.querySelector('audio')).toBeTruthy()
  })
})
