import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { cliente, ErrorAPI, SESION_EXPIRADA } from '../api/cliente'

export type Rol = 'student' | 'teacher' | 'admin'
export interface Usuario {
  id: string
  email: string
  full_name: string
  role: Rol
  status: string
}
interface ContextoSesion {
  usuario: Usuario | null
  cargando: boolean
  error: string | null
  recargar: () => Promise<void>
  ingresar: (email: string, password: string) => Promise<void>
  salir: () => Promise<void>
}
const Contexto = createContext<ContextoSesion | null>(null)
export const inicioPorRol: Record<Rol, string> = {
  student: '/mis-cursos', teacher: '/profesor/cursos', admin: '/admin',
}
export const nombreRol: Record<Rol, string> = {
  student: 'Estudiante', teacher: 'Profesor', admin: 'Administrador',
}

export function ProveedorSesion({ children }: { children: ReactNode }) {
  const [usuario, setUsuario] = useState<Usuario | null>(null)
  const [cargando, setCargando] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const recargar = useCallback(async () => {
    setCargando(true)
    setError(null)
    try { setUsuario(await cliente<Usuario>('/me')) } catch (e) {
      setUsuario(null)
      if (!(e instanceof ErrorAPI && e.status === 401)) {
        setError(e instanceof Error ? e.message : 'No pudimos consultar tu sesión.')
      }
    } finally { setCargando(false) }
  }, [])

  useEffect(() => {
    void recargar()
    const expirar = () => setUsuario(null)
    window.addEventListener(SESION_EXPIRADA, expirar)
    return () => window.removeEventListener(SESION_EXPIRADA, expirar)
  }, [recargar])

  async function ingresar(email: string, password: string) {
    await cliente('/auth/login', { method: 'POST', body: { email, password } })
    // Confirma que el navegador recibió la cookie; nunca guarda el token JSON.
    const actual = await cliente<Usuario>('/me')
    setUsuario(actual)
    setError(null)
  }

  async function salir() {
    try { await cliente('/auth/logout', { method: 'POST' }) } catch (e) {
      if (!(e instanceof ErrorAPI && e.status === 401)) throw e
    }
    setUsuario(null)
  }

  return <Contexto.Provider value={{ usuario, cargando, error, recargar, ingresar, salir }}>{children}</Contexto.Provider>
}

export function useSesion() {
  const contexto = useContext(Contexto)
  if (!contexto) throw new Error('useSesion requiere ProveedorSesion.')
  return contexto
}
