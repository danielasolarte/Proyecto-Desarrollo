// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { App } from '../App'
import { ProveedorSesion } from './Sesion'
import { RutaProtegida } from './RutaProtegida'

const usuario = { id: '1', email: 'estudiante@example.test', full_name: 'Estudiante de prueba', role: 'student', status: 'active' }
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
function abrir(ruta = '/') {
  vi.stubGlobal('__APP_COMMIT__', 'test')
  render(<MemoryRouter initialEntries={[ruta]} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}><ProveedorSesion><App /></ProveedorSesion></MemoryRouter>)
}
it('bloquea la pantalla de profesor para un estudiante autenticado', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(usuario))))
  render(<MemoryRouter initialEntries={['/profesor/cursos']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}><ProveedorSesion><Routes>
    <Route element={<RutaProtegida roles={['teacher']} />}><Route path="/profesor/cursos" element={<p>Contenido privado del profesor</p>} /></Route>
    <Route path="/sin-permiso" element={<p>Acceso restringido</p>} />
  </Routes></ProveedorSesion></MemoryRouter>)
  expect(await screen.findByText('Acceso restringido')).toBeTruthy()
  expect(screen.queryByText('Contenido privado del profesor')).toBeNull()
})
it('restaura la sesión desde /me y muestra el menú de estudiante', async () => {
  const fetchMock = vi.fn().mockImplementation((url: string) => {
    if (url === '/api/v1/me') {
      return Promise.resolve(
        new Response(JSON.stringify(usuario)),
      )
    }

    if (url === '/api/v1/me/enrollments') {
      return Promise.resolve(
        new Response(JSON.stringify([])),
      )
    }

    return Promise.resolve(
      new Response('{}', { status: 404 }),
    )
  })

  vi.stubGlobal('fetch', fetchMock)

  abrir()

  expect(
    await screen.findByRole('heading', { name: 'Mis cursos' }),
  ).toBeTruthy()

  expect(
    screen.getByRole('link', { name: 'Mi aprendizaje' }),
  ).toBeTruthy()

  expect(
    screen.getAllByRole('link', { name: 'Explorar cursos' }).length,
  ).toBeGreaterThan(0)
})
it('inicia sesión, verifica la cookie con /me y cierra la sesión', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(
      new Response(
        '{"error":{"message":"no autenticado"}}',
        { status: 401 },
      ),
    )
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          user: usuario,
          token: 'no-almacenar',
        }),
      ),
    )
    .mockResolvedValueOnce(
      new Response(JSON.stringify(usuario)),
    )
    .mockResolvedValueOnce(
      new Response(JSON.stringify([])),
    )
    .mockResolvedValueOnce(
      new Response(null, { status: 204 }),
    )

  vi.stubGlobal('fetch', fetchMock)

  abrir('/login')

  fireEvent.change(
    await screen.findByLabelText('Correo electrónico'),
    { target: { value: usuario.email } },
  )

  fireEvent.change(
    screen.getByLabelText('Contraseña'),
    { target: { value: 'contraseña-de-prueba' } },
  )

  fireEvent.click(
    screen.getByRole('button', {
      name: 'Entrar a mi espacio',
    }),
  )

  expect(
    await screen.findByRole('heading', { name: 'Mis cursos' }),
  ).toBeTruthy()

  expect(localStorage.length).toBe(0)
  expect(sessionStorage.length).toBe(0)

  fireEvent.click(
    screen.getByRole('button', { name: 'Cerrar sesión' }),
  )

  await waitFor(() =>
    expect(
      screen.getByRole('button', {
        name: 'Entrar a mi espacio',
      }),
    ).toBeTruthy(),
  )

  expect(fetchMock.mock.calls.map(c => c[0])).toEqual([
    '/api/v1/me',
    '/api/v1/auth/login',
    '/api/v1/me',
    '/api/v1/me/enrollments',
    '/api/v1/auth/logout',
  ])
})
it('muestra errores de login sin dar acceso', async () => {
  vi.stubGlobal('fetch', vi.fn()
    .mockResolvedValueOnce(new Response('{}', { status: 401 }))
    .mockResolvedValueOnce(new Response('{"error":{"message":"credenciales invalidas"}}', { status: 401 })))
  abrir('/login')
  fireEvent.change(await screen.findByLabelText('Correo electrónico'), { target: { value: usuario.email } })
  fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'incorrecta' } })
  fireEvent.click(screen.getByRole('button', { name: 'Entrar a mi espacio' }))
  expect(await screen.findByRole('alert')).toBeTruthy()
  expect(screen.getByText('credenciales invalidas')).toBeTruthy()
  expect(screen.queryByText('Hola, Estudiante de prueba.')).toBeNull()
})
it('pide login al abrir directamente una ruta protegida sin sesión', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 401 })))
  abrir('/profesor/cursos')
  expect(await screen.findByRole('button', { name: 'Entrar a mi espacio' })).toBeTruthy()
  expect(screen.queryByText('Mis cursos')).toBeNull()
})
it('vuelve al login cuando otra petición detecta una sesión expirada', async () => {
  const fetchMock = vi.fn().mockImplementation((url: string) => {
    if (url === '/api/v1/me') {
      return Promise.resolve(
        new Response(JSON.stringify(usuario)),
      )
    }

    if (url === '/api/v1/me/enrollments') {
      return Promise.resolve(
        new Response(JSON.stringify([])),
      )
    }

    return Promise.resolve(
      new Response('{}', { status: 404 }),
    )
  })

  vi.stubGlobal('fetch', fetchMock)

  abrir('/mis-cursos')

  expect(
    await screen.findByRole('heading', { name: 'Mis cursos' }),
  ).toBeTruthy()

  const { act } = await import('@testing-library/react')

  await act(async () => {
    window.dispatchEvent(
      new Event('mooc:sesion-expirada'),
    )
  })

  expect(
    await screen.findByRole('button', {
      name: 'Entrar a mi espacio',
    }),
  ).toBeTruthy()
})
