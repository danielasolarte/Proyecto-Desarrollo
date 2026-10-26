// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { derivadoListo, VisorRecurso } from './VisorRecurso'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

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
})
