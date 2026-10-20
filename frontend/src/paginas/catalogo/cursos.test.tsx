// @vitest-environment jsdom
import type { ReactElement } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { ErrorAPI } from '../../api/cliente'
import { problemasDePublicacion, siguientePosicion } from '../../api/cursos'
import { ProveedorSesion } from '../../auth/Sesion'
import { Catalogo } from './Catalogo'
import { DetalleCurso } from './DetalleCurso'
import { MisCursos } from '../profesor/MisCursos'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

const estudiante = { id: 'est-1', email: 'e@mooc.test', full_name: 'Est', role: 'student', status: 'active' }
const profesor = { id: 'profe-1', email: 'p@mooc.test', full_name: 'Profe', role: 'teacher', status: 'active' }
const json = (datos: unknown, status = 200) => new Response(JSON.stringify(datos), { status })

// Simula la API: cada ruta (método + camino) devuelve una respuesta fija.
function api(rutas: Record<string, () => Response>) {
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const clave = `${(init?.method || 'GET').toUpperCase()} ${url.replace(/\?.*$/, '')}`
    const r = rutas[clave]
    return r ? r() : new Response('{"error":{"message":"no simulado"}}', { status: 500 })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function montar(ruta: string, elemento: ReactElement, patron: string) {
  return render(<MemoryRouter initialEntries={[ruta]} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}><ProveedorSesion><Routes><Route path={patron} element={elemento} /></Routes></ProveedorSesion></MemoryRouter>)
}

const temario = {
  course_id: 'c1', slug: 'go', teacher_id: 'profe-1', title: 'Introducción a Go', summary: 'Fundamentos', category: 'programacion',
  modules: [{ id: 'm1', title: 'Fundamentos', units: [{ id: 'u1', title: 'Primeros pasos', resources: [{ id: 'r1', title: 'Lectura', type: 'rich_text', required: true }] }] }],
}

describe('utilidades de cursos', () => {
  it('calcula la siguiente posición libre', () => {
    expect(siguientePosicion([])).toBe(1)
    expect(siguientePosicion([{ position: 1 }, { position: 4 }])).toBe(5)
  })
  it('convierte el 400 de publicación en la lista de problemas', () => {
    const e = new ErrorAPI(400, 'error de validacion\nel curso no tiene categoria; el curso no tiene ningun modulo')
    expect(problemasDePublicacion(e)).toEqual(['el curso no tiene categoria', 'el curso no tiene ningun modulo'])
    expect(problemasDePublicacion(new ErrorAPI(500, 'x'))).toBeNull()
  })
})

describe('catálogo', () => {
  it('lista los cursos publicados', async () => {
    api({
      'GET /api/v1/me': () => json(estudiante),
      'GET /api/v1/catalog': () => json({ courses: [{ course_id: 'c1', slug: 'go', title: 'Introducción a Go', summary: 'Fundamentos', category: 'programacion', published_at: '2026-10-16T00:00:00Z' }], next_cursor: '' }),
    })
    montar('/catalogo', <Catalogo />, '/catalogo')
    expect((await screen.findByRole('link', { name: 'Introducción a Go' })).getAttribute('href')).toBe('/cursos/c1')
  })
  it('muestra el estado vacío', async () => {
    api({ 'GET /api/v1/me': () => json(estudiante), 'GET /api/v1/catalog': () => json({ courses: [], next_cursor: '' }) })
    montar('/catalogo', <Catalogo />, '/catalogo')
    expect(await screen.findByText('Todavía no hay cursos publicados')).toBeTruthy()
  })
})

describe('detalle del curso', () => {
  it('inscribe al estudiante y cambia a Ir al curso', async () => {
    const fetchMock = api({
      'GET /api/v1/me': () => json(estudiante),
      'GET /api/v1/courses/c1/outline': () => json(temario),
      'GET /api/v1/me/enrollments': () => json([]),
      'POST /api/v1/courses/c1/enrollments': () => json({ id: 'e1', course_id: 'c1', status: 'active' }, 201),
    })
    montar('/cursos/c1', <DetalleCurso />, '/cursos/:id')
    expect(await screen.findByText('Lectura')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /Inscribirme/ }))
    expect(await screen.findByRole('button', { name: /Ir al curso/ })).toBeTruthy()
    expect(fetchMock.mock.calls.some(([u, i]) => u === '/api/v1/courses/c1/enrollments' && i?.method === 'POST')).toBe(true)
  })
  it('un estudiante ya inscrito ve Ir al curso y no Inscribirme', async () => {
    api({
      'GET /api/v1/me': () => json(estudiante),
      'GET /api/v1/courses/c1/outline': () => json(temario),
      'GET /api/v1/me/enrollments': () => json([{ id: 'e1', course_id: 'c1', status: 'active' }]),
    })
    montar('/cursos/c1', <DetalleCurso />, '/cursos/:id')
    expect(await screen.findByRole('button', { name: /Ir al curso/ })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Inscribirme/ })).toBeNull()
  })
  it('un curso no publicado muestra el mensaje y no el temario', async () => {
    api({
      'GET /api/v1/me': () => json(estudiante),
      'GET /api/v1/courses/c9/outline': () => json({ error: { code: 'not_found', message: 'recurso no encontrado' } }, 404),
      'GET /api/v1/me/enrollments': () => json([]),
    })
    montar('/cursos/c9', <DetalleCurso />, '/cursos/:id')
    expect(await screen.findByText('Este curso no está publicado.')).toBeTruthy()
  })
  it('el profesor dueño ve Editar y no puede inscribirse', async () => {
    api({ 'GET /api/v1/me': () => json(profesor), 'GET /api/v1/courses/c1/outline': () => json(temario) })
    montar('/cursos/c1', <DetalleCurso />, '/cursos/:id')
    expect(await screen.findByRole('link', { name: 'Editar este curso' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Inscribirme/ })).toBeNull()
  })
})

describe('mis cursos del profesor', () => {
  it('muestra cada curso con su estado', async () => {
    api({
      'GET /api/v1/me': () => json(profesor),
      'GET /api/v1/me/courses': () => json({ courses: [
        { id: 'c1', slug: 'a', title: 'Borrador A', summary: '', category: 'x', status: 'draft', created_at: '2026-10-16T00:00:00Z', updated_at: '2026-10-16T00:00:00Z' },
        { id: 'c2', slug: 'b', title: 'Publicado B', summary: '', category: 'x', status: 'published', created_at: '2026-10-15T00:00:00Z', updated_at: '2026-10-15T00:00:00Z' },
      ], next_cursor: '' }),
    })
    montar('/profesor/cursos', <MisCursos />, '/profesor/cursos')
    expect(await screen.findByText('Borrador A')).toBeTruthy()
    expect(screen.getByText('Borrador')).toBeTruthy()
    expect(screen.getByText('Publicado')).toBeTruthy()
    expect(screen.getAllByRole('link', { name: 'Ver publicado' })).toHaveLength(1)
  })
  it('sin cursos muestra el estado vacío', async () => {
    api({ 'GET /api/v1/me': () => json(profesor), 'GET /api/v1/me/courses': () => json({ courses: [], next_cursor: '' }) })
    montar('/profesor/cursos', <MisCursos />, '/profesor/cursos')
    await waitFor(() => expect(screen.getByText('Todavía no has creado cursos')).toBeTruthy())
  })
})
