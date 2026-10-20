import { Link } from 'react-router-dom'
import { inicioPorRol, nombreRol, useSesion } from '../auth/Sesion'
import { Vacio } from '../componentes/estados/Estados'

export function Inicio({ modulo }: { modulo: string }) {
  const { usuario } = useSesion()
  if (!usuario) return null
  return <>
    <div className="ruta-seccion">MI ESPACIO <span>/</span> {modulo.toLocaleUpperCase('es')}</div>
    <section className="bienvenida"><span className="etiqueta texto-verde">{nombreRol[usuario.role]}</span><h1>Hola, {usuario.full_name}.</h1><p>Este es tu espacio para {usuario.role === 'teacher' ? 'compartir conocimiento' : usuario.role === 'admin' ? 'acompañar a tu comunidad' : 'seguir aprendiendo'}.</p></section>
    <section className="tarjeta"><div className="tarjeta-cabecera"><h2>{modulo}</h2><span className="insignia">Próximamente</span></div>
      <Vacio titulo="Estamos preparando este espacio"><p>Esta sección estará disponible cuando se integren sus funcionalidades.</p></Vacio>
    </section>
    <p className="sesion-detalle">Sesión iniciada como <strong>{usuario.email}</strong></p>
  </>
}

export function SinPermiso() {
  const { usuario } = useSesion()
  return <section className="pagina-mensaje"><span className="etiqueta texto-verde">ACCESO RESTRINGIDO</span><h1>Esta sección pertenece a otro rol.</h1><p>Vuelve a tu espacio para continuar.</p><Link className="boton-principal" to={usuario ? inicioPorRol[usuario.role] : '/login'}>Volver a mi espacio</Link></section>
}
export function NoEncontrado() {
  return <section className="pagina-mensaje"><span className="etiqueta texto-verde">404</span><h1>No encontramos esta página.</h1><p>Revisa la dirección o vuelve al inicio.</p><Link className="boton-principal" to="/">Ir al inicio</Link></section>
}
