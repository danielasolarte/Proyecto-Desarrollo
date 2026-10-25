import http from 'k6/http'
import { check, fail } from 'k6'

const BASE_URL = __ENV.BASE_URL
const STUDENT_EMAIL = __ENV.STUDENT_EMAIL
const STUDENT_PASSWORD = __ENV.STUDENT_PASSWORD
const ATTEMPT_ID = __ENV.ATTEMPT_ID
const PREVIOUS_INSTANCE_ID = __ENV.PREVIOUS_INSTANCE_ID

export const options = {
  vus: 1,
  iterations: 1,

  thresholds: {
    checks: ['rate==1'],
    http_req_failed: ['rate==0'],
  },
}

function json(res) {
  try {
    return res.json()
  } catch (_) {
    return {}
  }
}

function instanceId(res) {
  return (
    res.headers['X-Instance-Id'] ||
    res.headers['x-instance-id'] ||
    'sin-instance-id'
  )
}

export default function () {
  if (!ATTEMPT_ID) {
    fail('Falta ATTEMPT_ID')
  }

  const login = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({
      email: STUDENT_EMAIL,
      password: STUDENT_PASSWORD,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
      },
    },
  )

  check(login, {
    'I5 login después del retiro responde 200':
      r => r.status === 200,
  })

  const token = json(login).token

  if (!token) {
    fail('No se obtuvo token')
  }

  const res = http.get(
    `${BASE_URL}/attempts/${ATTEMPT_ID}`,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    },
  )

  check(res, {
    'I5 dato continúa existiendo': r => r.status === 200,
  })

  const nuevaInstancia = instanceId(res)

  console.log('========================================')
  console.log('I5 — ESTADO DESPUES DEL RETIRO')
  console.log('========================================')
  console.log(`Attempt: ${ATTEMPT_ID}`)
  console.log(`Instancia anterior: ${PREVIOUS_INSTANCE_ID}`)
  console.log(`Instancia actual: ${nuevaInstancia}`)

  if (
    PREVIOUS_INSTANCE_ID &&
    PREVIOUS_INSTANCE_ID !== 'sin-instance-id'
  ) {
    check(null, {
      'I5 responde una instancia diferente': () =>
        nuevaInstancia !== PREVIOUS_INSTANCE_ID,
    })
  }

  const body = json(res)

  const attempt =
    body.attempt ||
    body

  check(null, {
    'I5 attempt conserva identificador': () =>
      (attempt.id || attempt.ID) === ATTEMPT_ID,
  })

  console.log('========================================')
}