import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { Cargando, ErrorEstado } from '../componentes/estados/Estados'
import { useSesion, type Rol } from './Sesion'

export function RutaProtegida({ roles }: { roles?: Rol[] }) {
  const { usuario, cargando, error, recargar } = useSesion()
  const location = useLocation()
  if (cargando) return <Cargando />
  if (error) return <ErrorEstado mensaje={error} reintentar={() => void recargar()} />
  if (!usuario) return <Navigate to="/login" state={{ desde: location.pathname + location.search }} replace />
  if (roles && !roles.includes(usuario.role)) return <Navigate to="/sin-permiso" replace />
  return <Outlet />
}
