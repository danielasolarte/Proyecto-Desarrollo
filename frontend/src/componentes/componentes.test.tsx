// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { partesPendientes } from '../api/media'
import { problemaPregunta } from '../api/quiz'
import { CargaArchivo } from './media/CargaArchivo'
import { EditorQuiz } from './quiz/EditorQuiz'

afterEach(() => { cleanup(); vi.unstubAllGlobals(); localStorage.clear() })

const json = (datos: unknown, status = 200) => new Response(JSON.stringify(datos), { status })

function api(rutas: Record<string, (init?: RequestInit) => Response>) {
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const clave = `${(init?.method || 'GET').toUpperCase()} ${url.replace(/\?.*$/, '')}`
    const r = rutas[clave]
    return r ? r(init) : new Response('{"error":{"message":"no simulado"}}', { status: 500 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

const llamadas = (f: ReturnType<typeof api>, metodo: string, url: string) =>
  f.mock.calls.filter(([u, i]) => u === url && (i?.method || 'GET').toUpperCase() === metodo)

describe('utilidades', () => {
  it('calcula las partes que faltan por subir', () => {
    expect(partesPendientes(10, 4, [])).toEqual([1, 2, 3])
    expect(partesPendientes(10, 4, [2])).toEqual([1, 3])
    expect(partesPendientes(0, 4, [])).toEqual([1])
  })
  it('valida una pregunta antes de enviarla', () => {
    expect(problemaPregunta('', 1, [{ texto: 'a' }, { texto: 'b' }], 0)).toMatch(/enunciado/)
    expect(problemaPregunta('¿?', 1, [{ texto: 'a' }, { texto: '' }], 0)).toMatch(/dos opciones/)
    expect(problemaPregunta('¿?', 1, [{ texto: 'a' }, { texto: 'b' }, { texto: '' }], 2)).toMatch(/correcta/)
    expect(problemaPregunta('¿?', 0, [{ texto: 'a' }, { texto: 'b' }], 0)).toMatch(/puntos/)
    expect(problemaPregunta('¿?', 1, [{ texto: 'a' }, { texto: 'b' }], 1)).toBeNull()
  })
})

describe('EditorQuiz', () => {
  it('crea el quiz cuando el recurso no tiene uno', async () => {
    let creado = false
    const quiz = { ID: 'q1', ResourceID: 'r1', PassingScore: 70, MaxAttempts: 2, TimeLimitSeconds: 600, FeedbackMode: 'after_submit' }
    const f = api({
      'GET /api/v1/resources/r1/quiz': () => (creado ? json({ quiz, questions: [] }) : json({ error: { message: 'quiz not found' } }, 404)),
      'POST /api/v1/resources/r1/quiz': () => { creado = true; return json(quiz, 201) },
    })
    render(<EditorQuiz resourceId="r1" />)
    fireEvent.change(await screen.findByLabelText('Nota mínima para aprobar (%)'), { target: { value: '70' } })
    fireEvent.change(screen.getByLabelText('Intentos permitidos'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('Tiempo límite (minutos)'), { target: { value: '10' } })
    fireEvent.click(screen.getByRole('button', { name: 'Crear quiz' }))
    expect(await screen.findByText(/Aún no hay preguntas/)).toBeTruthy()
    const [, init] = llamadas(f, 'POST', '/api/v1/resources/r1/quiz')[0]
    expect(JSON.parse(String(init?.body))).toEqual({ passing_score: 70, max_attempts: 2, time_limit_seconds: 600, feedback_mode: 'after_submit' })
  })

  it('agrega una pregunta con sus opciones y marca la correcta', async () => {
    const quiz = { ID: 'q1', ResourceID: 'r1', PassingScore: 60, MaxAttempts: null, TimeLimitSeconds: null, FeedbackMode: 'none' }
    let preguntas: unknown[] = []
    const f = api({
      'GET /api/v1/resources/r1/quiz': () => json({ quiz, questions: preguntas }),
      'POST /api/v1/quizzes/q1/questions': () => json({ ID: 'p1', Text: '¿2+2?', Position: 1, Points: 1, Options: null }, 201),
      'POST /api/v1/questions/p1/options': init => {
        const b = JSON.parse(String(init?.body))
        if (b.position === 2) preguntas = [{ ID: 'p1', Text: '¿2+2?', Position: 1, Points: 1, Options: [{ ID: 'o1', Text: '3', Position: 1, IsCorrect: false }, { ID: 'o2', Text: '4', Position: 2, IsCorrect: true }] }]
        return json({ ID: `o${b.position}`, ...b }, 201)
      },
    })
    render(<EditorQuiz resourceId="r1" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Agregar pregunta' }))
    fireEvent.change(screen.getByLabelText('Pregunta'), { target: { value: '¿2+2?' } })
    fireEvent.change(screen.getByLabelText('Texto de la opción 1'), { target: { value: '3' } })
    fireEvent.change(screen.getByLabelText('Texto de la opción 2'), { target: { value: '4' } })
    fireEvent.click(screen.getByLabelText('Opción 2 es la correcta'))
    fireEvent.click(screen.getByRole('button', { name: 'Guardar pregunta' }))
    expect(await screen.findByText('¿2+2?')).toBeTruthy()
    const opciones = llamadas(f, 'POST', '/api/v1/questions/p1/options').map(([, i]) => JSON.parse(String(i?.body)))
    expect(opciones).toEqual([{ text: '3', position: 1, is_correct: false }, { text: '4', position: 2, is_correct: true }])
    expect(screen.getByText('(correcta)').parentElement?.textContent).toContain('4')
  })

  it('no permite guardar una pregunta sin opción correcta con texto', async () => {
    api({ 'GET /api/v1/resources/r1/quiz': () => json({ quiz: { ID: 'q1', PassingScore: 60, FeedbackMode: 'none' }, questions: [] }) })
    render(<EditorQuiz resourceId="r1" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Agregar pregunta' }))
    fireEvent.change(screen.getByLabelText('Pregunta'), { target: { value: '¿?' } })
    fireEvent.change(screen.getByLabelText('Texto de la opción 1'), { target: { value: 'a' } })
    fireEvent.click(screen.getByRole('button', { name: 'Guardar pregunta' }))
    expect((await screen.findByRole('alert')).textContent).toMatch(/dos opciones/)
  })
})

// XMLHttpRequest falso: registra los PUT a las URL prefirmadas y responde 200.
const puts: { url: string; tamano: number }[] = []
class XHRFalso {
  status = 0
  upload: { onprogress: ((e: { loaded: number }) => void) | null } = { onprogress: null }
  onload: (() => void) | null = null
  onerror: (() => void) | null = null
  onabort: (() => void) | null = null
  private url = ''
  open(_m: string, url: string) { this.url = url }
  abort() { this.onabort?.() }
  send(cuerpo: Blob) {
    setTimeout(() => {
      puts.push({ url: this.url, tamano: cuerpo.size })
      this.upload.onprogress?.({ loaded: cuerpo.size })
      this.status = 200
      this.onload?.()
    }, 0)
  }
}

describe('CargaArchivo', () => {
  it('sube el archivo por partes, confirma la carga y avisa al editor', async () => {
    puts.length = 0
    vi.stubGlobal('XMLHttpRequest', XHRFalso)
    const f = api({
      'POST /api/v1/resources/r1/uploads/initiate': () => json({ upload_session: { id: 's1', resource_id: 'r1', status: 'uploading', expires_at: '' }, recommended_part_bytes: 4 }, 201),
      'GET /api/v1/uploads/s1/parts/1/url': () => json({ url: 'https://bucket/p1' }),
      'GET /api/v1/uploads/s1/parts/2/url': () => json({ url: 'https://bucket/p2' }),
      'GET /api/v1/uploads/s1/parts/3/url': () => json({ url: 'https://bucket/p3' }),
      'POST /api/v1/uploads/s1/complete': () => json({ id: 'a1', resource_id: 'r1', kind: 'video' }),
    })
    const alCambiar = vi.fn()
    render(<CargaArchivo resourceId="r1" tipo="video" estado="pending" alCambiar={alCambiar} />)
    const archivo = new File(['0123456789'], 'clase.mp4', { type: 'video/mp4' })
    fireEvent.change(screen.getByLabelText('Archivo'), { target: { files: [archivo] } })
    fireEvent.click(screen.getByRole('button', { name: 'Subir archivo' }))
    await waitFor(() => expect(alCambiar).toHaveBeenCalled())
    expect(puts.map(p => p.tamano).sort()).toEqual([2, 4, 4])
    expect(llamadas(f, 'POST', '/api/v1/uploads/s1/complete')).toHaveLength(1)
    const [, init] = llamadas(f, 'POST', '/api/v1/resources/r1/uploads/initiate')[0]
    expect(JSON.parse(String(init?.body))).toMatchObject({ mime_type: 'video/mp4', size_bytes: 10 })
    expect(localStorage.getItem('mooc:carga:r1')).toBeNull()
    expect(await screen.findByText(/en cola para procesarse/)).toBeTruthy()
  })

  it('reanuda una carga anterior subiendo solo las partes que faltan', async () => {
    puts.length = 0
    vi.stubGlobal('XMLHttpRequest', XHRFalso)
    const archivo = new File(['0123456789'], 'clase.mp4', { type: 'video/mp4', lastModified: 1 })
    localStorage.setItem('mooc:carga:r1', JSON.stringify({ sesionId: 's1', huella: `clase.mp4|10|1`, tamanoParte: 4 }))
    const f = api({
      'GET /api/v1/uploads/s1/parts': () => json({ parts: [{ part_number: 1, etag: 'x', size_bytes: 4 }, { part_number: 2, etag: 'y', size_bytes: 4 }] }),
      'GET /api/v1/uploads/s1/parts/3/url': () => json({ url: 'https://bucket/p3' }),
      'POST /api/v1/uploads/s1/complete': () => json({ id: 'a1', resource_id: 'r1', kind: 'video' }),
    })
    const alCambiar = vi.fn()
    render(<CargaArchivo resourceId="r1" tipo="video" alCambiar={alCambiar} />)
    fireEvent.change(screen.getByLabelText('Archivo'), { target: { files: [archivo] } })
    fireEvent.click(screen.getByRole('button', { name: 'Continuar carga' }))
    await waitFor(() => expect(alCambiar).toHaveBeenCalled())
    expect(puts).toEqual([{ url: 'https://bucket/p3', tamano: 2 }])
    expect(llamadas(f, 'POST', '/api/v1/resources/r1/uploads/initiate')).toHaveLength(0)
  })

  it('no muestra el selector si la versión no es editable', () => {
    render(<CargaArchivo resourceId="r1" tipo="pdf" estado="ready" editable={false} alCambiar={() => {}} />)
    expect(screen.queryByLabelText(/archivo/i)).toBeNull()
  })
})
