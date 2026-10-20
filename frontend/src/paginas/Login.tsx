import { useState, type FormEvent } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { inicioPorRol, useSesion } from '../auth/Sesion'
import { Cargando, ErrorEstado } from '../componentes/estados/Estados'

export function Login() {
  const { usuario, cargando, ingresar, error: errorSesion } = useSesion()
  const location = useLocation()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [enviando, setEnviando] = useState(false)
  const [error, setError] = useState('')
  if (cargando) return <Cargando />
  if (usuario) {
    const desde: unknown = location.state?.desde
    const destino = typeof desde === 'string' && desde.startsWith('/') && !desde.startsWith('//') && !desde.startsWith('/login')
      ? desde : inicioPorRol[usuario.role]
    return <Navigate to={destino} replace />
  }
  async function enviar(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setEnviando(true)
    setError('')
    try { await ingresar(email.trim(), password) } catch (e) {
      setError(e instanceof Error ? e.message : 'No pudimos iniciar sesión.')
    } finally { setEnviando(false) }
  }
  return <main id="contenido" className="login-layout">
    <section className="presentacion" aria-labelledby="bienvenida">
      <span className="etiqueta">UN ESPACIO PARA CRECER</span>
      <h1 id="bienvenida">Aprender abre<br />nuevos caminos.</h1>
      <p>Conecta con nuevas ideas, comparte lo que sabes y construye tu próximo paso.</p>
      <div className="ilustracion" aria-hidden="true"><div className="orbita orbita-uno" /><div className="orbita orbita-dos" /><span className="libro">m<span>↗</span></span><span className="estrella">✳</span></div>
      <span className="pie-presentacion">Tu curiosidad es el punto de partida.</span>
    </section>
    <section className="login-panel" aria-labelledby="titulo-login">
      <div className="login-formulario"><span className="etiqueta texto-verde">TU PLATAFORMA MOOC</span>
        <h2 id="titulo-login">Qué bueno verte.</h2><p className="subtitulo">Inicia sesión para continuar tu recorrido.</p>
        <form onSubmit={enviar} aria-busy={enviando}>
          <label htmlFor="email">Correo electrónico</label>
          <input id="email" type="email" autoComplete="username" placeholder="tu@correo.com" required value={email} onChange={e => setEmail(e.target.value)} disabled={enviando} />
          <label htmlFor="password">Contraseña</label>
          <input id="password" type="password" autoComplete="current-password" required value={password} onChange={e => setPassword(e.target.value)} disabled={enviando} />
          {(error || errorSesion) && <ErrorEstado mensaje={error || errorSesion!} />}
          <button className="boton-principal" type="submit" disabled={enviando}>{enviando ? 'Iniciando sesión…' : 'Entrar a mi espacio'}<span aria-hidden="true">→</span></button>
        </form>
        <p className="ayuda">Usa la cuenta que te asignaron para acceder como estudiante, profesor o administrador.</p>
      </div>
    </section>
  </main>
}
