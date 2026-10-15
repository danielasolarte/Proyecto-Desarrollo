// Node 20+. Usa solo la API pública; no modifica la base directamente.
const base = new URL(process.env.MOOC_API_URL || 'http://localhost:8080/api/v1')
if (base.protocol !== 'https:' && !(base.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(base.hostname))) {
  throw new Error('Usa HTTPS en la nube; HTTP solo está permitido en localhost.')
}
if (base.username || base.password || base.search || base.hash) throw new Error('La URL del API no debe contener credenciales, parámetros ni fragmentos.')
const env = nombre => {
  const valor = process.env[nombre]
  if (!valor) throw new Error(`Falta la variable ${nombre}. Consulta frontend/README.md.`)
  return valor
}
class ErrorSolicitud extends Error {
  constructor(status, mensaje) { super(mensaje); this.status = status }
}
async function solicitar(ruta, { method = 'GET', body, token } = {}) {
  const respuesta = await fetch(`${base.href.replace(/\/$/, '')}${ruta}`, {
    method, redirect: 'error', signal: AbortSignal.timeout(20000),
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const texto = await respuesta.text()
  let datos
  try { datos = texto ? JSON.parse(texto) : undefined } catch { throw new Error('El API no devolvió JSON.') }
  if (!respuesta.ok) throw new ErrorSolicitud(respuesta.status, datos?.error?.message || datos?.message || `HTTP ${respuesta.status}`)
  return datos
}
const credenciales = rol => ({
  email: env(`MOOC_${rol}_EMAIL`),
  password: env(`MOOC_${rol}_PASSWORD`),
  full_name: `Pruebas E3 ${rol.toLowerCase()}`,
})
async function ejecutar() {
  const admin = credenciales('ADMIN')
  const profesor = credenciales('TEACHER')
  const estudiante = credenciales('STUDENT')
  const correos = [admin, profesor, estudiante].map(c => c.email.trim().toLowerCase())
  if (new Set(correos).size !== 3) throw new Error('Usa un correo diferente para cada rol.')
  for (const c of [admin, profesor, estudiante]) {
    if (c.password.length < 8) throw new Error('Cada contraseña debe tener al menos ocho caracteres.')
  }
  let sesion
  try { sesion = await solicitar('/auth/login', { method: 'POST', body: admin }) } catch (e) {
    if (!(e instanceof ErrorSolicitud && e.status === 401)) throw e
    try { await solicitar('/auth/bootstrap-admin', { method: 'POST', body: admin }) } catch (bootstrapError) {
      if (bootstrapError instanceof ErrorSolicitud && bootstrapError.status === 409) {
        throw new Error('Ya existe un administrador. Usa sus credenciales en MOOC_ADMIN_EMAIL y MOOC_ADMIN_PASSWORD.')
      }
      throw bootstrapError
    }
    sesion = await solicitar('/auth/login', { method: 'POST', body: admin })
  }
  const token = sesion.token
  if (!token) throw new Error('El login no devolvió un token para el script de preparación.')
  try {
    if (sesion.user.role !== 'admin') throw new Error('La cuenta MOOC_ADMIN_EMAIL no es administradora.')
    const usuarios = await solicitar('/admin/users', { token })
    async function asegurar(cuenta, rol) {
      let usuario = usuarios.find(u => u.email.toLowerCase() === cuenta.email.trim().toLowerCase())
      if (!usuario) {
        if (rol === 'teacher') {
          usuario = await solicitar('/admin/users/teachers', { method: 'POST', body: cuenta, token })
        } else {
          const registro = await solicitar('/auth/register', { method: 'POST', body: cuenta })
          usuario = registro.user
          if (registro.verification_token_dev) {
            usuario = await solicitar('/auth/verify-email', { method: 'POST', body: { token: registro.verification_token_dev } })
          }
        }
      }
      if (usuario.role !== rol) throw new Error(`La cuenta de ${rol} ya existe con otro rol; no se modificó.`)
      if (rol === 'student' && usuario.status === 'pending_verification') {
        usuario = await solicitar(`/admin/users/${usuario.id}`, { method: 'PATCH', body: { status: 'active' }, token })
      }
      if (usuario.status !== 'active') throw new Error(`La cuenta de ${rol} está inactiva; revísala con administración.`)
      // Repetir el script no cambia contraseñas ni crea cuentas duplicadas.
      const prueba = await solicitar('/auth/login', { method: 'POST', body: cuenta })
      await solicitar('/auth/logout', { method: 'POST', token: prueba.token })
      console.log(`Cuenta ${rol}: activa y login verificado.`)
    }
    await asegurar(profesor, 'teacher')
    await asegurar(estudiante, 'student')
    console.log('Cuentas listas. No se guardaron contraseñas ni tokens en archivos.')
  } finally {
    await solicitar('/auth/logout', { method: 'POST', token })
  }
}
ejecutar().catch(error => { console.error(error.message); process.exitCode = 1 })
