import http from 'k6/http'
import { check, fail } from 'k6'

const BASE_URL = __ENV.BASE_URL
const STUDENT_EMAIL = __ENV.STUDENT_EMAIL
const STUDENT_PASSWORD = __ENV.STUDENT_PASSWORD
const QUIZ_ID = __ENV.QUIZ_ID
const QUESTION_ID = __ENV.QUESTION_ID
const CORRECT_OPTION_ID = __ENV.CORRECT_OPTION_ID

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
    'I5 login 200': r => r.status === 200,
  })

  const token = json(login).token

  if (!token) {
    fail('No se obtuvo token')
  }

  const headers = {
    Authorization: `Bearer ${token}`,
  }

  const attemptRes = http.post(
    `${BASE_URL}/quizzes/${QUIZ_ID}/attempts`,
    null,
    { headers },
  )

  check(attemptRes, {
    'I5 attempt creado 201': r => r.status === 201,
  })

  const attempt = json(attemptRes)

  const attemptId =
    attempt.id ||
    attempt.ID

  if (!attemptId) {
    fail('No se obtuvo attempt ID')
  }

  const writeRes = http.put(
    `${BASE_URL}/attempts/${attemptId}/answers/${QUESTION_ID}`,
    JSON.stringify({
      selected_option_id: CORRECT_OPTION_ID,
    }),
    {
      headers: {
        ...headers,
        'Content-Type': 'application/json',
      },
    },
  )

  check(writeRes, {
    'I5 respuesta guardada 200': r => r.status === 200,
  })

  const readRes = http.get(
    `${BASE_URL}/attempts/${attemptId}`,
    { headers },
  )

  check(readRes, {
    'I5 lectura previa 200': r => r.status === 200,
  })

  console.log('========================================')
  console.log('I5 — ESTADO ANTES DEL RETIRO')
  console.log('========================================')
  console.log(`I5_ATTEMPT_ID=${attemptId}`)
  console.log(
    `I5_INSTANCE_ID=${instanceId(writeRes)}`,
  )
  console.log(
    `I5_READ_INSTANCE=${instanceId(readRes)}`,
  )
  console.log('========================================')
}