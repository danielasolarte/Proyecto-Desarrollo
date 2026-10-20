import type { ReactNode } from 'react'

export function Cargando({ mensaje = 'Cargando tu espacio…' }: { mensaje?: string }) {
  return <div className="estado" role="status"><span className="spinner" aria-hidden="true" />{mensaje}</div>
}
export function Vacio({ titulo, children }: { titulo: string; children?: ReactNode }) {
  return <div className="estado estado-vacio"><span className="estado-icono" aria-hidden="true">＋</span><h2>{titulo}</h2>{children}</div>
}
export function ErrorEstado({ mensaje, reintentar }: { mensaje: string; reintentar?: () => void }) {
  return <div className="aviso aviso-error" role="alert"><p>{mensaje}</p>{reintentar && <button className="boton-secundario" onClick={reintentar}>Intentar de nuevo</button>}</div>
}
export { ErrorEstado as Error }
export function Exito({ children }: { children: ReactNode }) {
  return <div className="aviso aviso-exito" role="status">{children}</div>
}
export function Procesando({ mensaje = 'Tu archivo se está procesando. Aún no está disponible.' }: { mensaje?: string }) {
  return <Cargando mensaje={mensaje} />
}
