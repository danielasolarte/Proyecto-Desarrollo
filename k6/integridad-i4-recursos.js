import http from 'k6/http'
import { check, fail } from 'k6'

const BASE_URL =
  __ENV.BASE_URL || 'http://host.docker.internal:8080/api/v1'

const STUDENT_EMAIL = __ENV.STUDENT_EMAIL
const STUDENT_PASSWORD = __ENV.STUDENT_PASSWORD

const RESOURCE_ID = __ENV.RESOURCE_ID
const EXPECTED_QUIZ_ID = __ENV.EXPECTED_QUIZ_ID

const REQUIRE_TWO_INSTANCES =
  (__ENV.REQUIRE_TWO_INSTANCES || 'false').toLowerCase() === 'true'

const REPLICA_PROBES = Number(__ENV.REPLICA_PROBES || 20)

export const options = {
  vus: 1,
  iterations: 1,
  noConnectionReuse: true,

  thresholds: {
    checks: ['rate==1'],
    http_req_failed: ['rate==0'],
  },
}

function requireEnv(name, value) {
  if (!value) {
    fail(`Falta la variable ${name}`)
  }
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

function authHeaders(token) {
  return {
    Authorization: `Bearer ${token}`,
  }
}

export default function () {
  requireEnv('STUDENT_EMAIL', STUDENT_EMAIL)
  requireEnv('STUDENT_PASSWORD', STUDENT_PASSWORD)
  requireEnv('RESOURCE_ID', RESOURCE_ID)
  requireEnv('EXPECTED_QUIZ_ID', EXPECTED_QUIZ_ID)

  console.log('========================================')
  console.log('I4 — RECURSO ENTRE REPLICAS')
  console.log('========================================')

  const loginRes = http.post(
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

  check(loginRes, {
    'login responde 200': r => r.status === 200,
  })

  if (loginRes.status !== 200) {
    fail(`Login falló: ${loginRes.status} ${loginRes.body}`)
  }

  const token = json(loginRes).token

  if (!token) {
    fail('Login no devolvió token')
  }

  const instancias = new Set()
  let quizIdEsperado = null

  for (let i = 0; i < REPLICA_PROBES; i += 1) {
    const res = http.get(
      `${BASE_URL}/resources/${RESOURCE_ID}/quiz`,
      {
        headers: authHeaders(token),
      },
    )

    check(res, {
      'I4 recurso responde 200': r => r.status === 200,
    })

    if (res.status !== 200) {
      fail(`I4 falló: ${res.status} ${res.body}`)
    }

    const instancia = instanceId(res)

    if (instancia !== 'sin-instance-id') {
      instancias.add(instancia)
    }

    const body = json(res)

    const quizId =
      body.id ||
      body.ID

    if (quizIdEsperado === null) {
      quizIdEsperado = quizId
    }

    check(null, {
      'I4 quiz consistente entre respuestas': () =>
        quizId === quizIdEsperado,

      'I4 corresponde al quiz esperado': () =>
        quizId === EXPECTED_QUIZ_ID,
    })

    console.log(
      `I4 solicitud ${i + 1}: ${instancia} → quiz ${quizId}`,
    )
  }

  if (REQUIRE_TWO_INSTANCES) {
    check(null, {
      'I4 recurso atendido por al menos 2 replicas': () =>
        instancias.size >= 2,
    })
  }

  console.log('')
  console.log(
    `I4 instancias observadas: ${Array.from(instancias).join(', ')}`,
  )
}