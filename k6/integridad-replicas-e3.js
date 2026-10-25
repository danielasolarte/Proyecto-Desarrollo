import http from 'k6/http'
import { check, fail } from 'k6'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1'

const STUDENT_EMAIL = __ENV.STUDENT_EMAIL
const STUDENT_PASSWORD = __ENV.STUDENT_PASSWORD

const QUIZ_ID = __ENV.QUIZ_ID
const QUESTION_ID = __ENV.QUESTION_ID
const CORRECT_OPTION_ID = __ENV.CORRECT_OPTION_ID

// En local lo dejamos false porque solo existe una API.
// En GCP lo cambiaremos a true.
const REQUIRE_TWO_INSTANCES =
  (__ENV.REQUIRE_TWO_INSTANCES || 'false').toLowerCase() === 'true'

const REPLICA_PROBES = Number(__ENV.REPLICA_PROBES || 20)

export const options = {
  vus: 1,
  iterations: 1,

  // Facilita que distintas solicitudes puedan ser distribuidas
  // entre diferentes instancias detrás del balanceador.
  noConnectionReuse: true,

  thresholds: {
    checks: ['rate==1'],
    http_req_failed: ['rate==0'],
  },
}

function requireEnv(name, value) {
  if (!value) {
    fail(`Falta la variable de entorno ${name}`)
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

function authHeaders(token, extra = {}) {
  return {
    Authorization: `Bearer ${token}`,
    ...extra,
  }
}

function descartarCookies() {
  const jar = http.cookieJar()
  const actuales = jar.cookiesForURL(BASE_URL)

  for (const nombre of Object.keys(actuales)) {
    jar.delete(BASE_URL, nombre)
  }
}

function answerQuestionId(answer) {
  return (
    answer.question_id ||
    answer.questionId ||
    answer.QuestionID
  )
}

function answerOptionId(answer) {
  return (
    answer.selected_option_id ||
    answer.selectedOptionId ||
    answer.SelectedOptionID
  )
}

export default function () {
  requireEnv('STUDENT_EMAIL', STUDENT_EMAIL)
  requireEnv('STUDENT_PASSWORD', STUDENT_PASSWORD)
  requireEnv('QUIZ_ID', QUIZ_ID)
  requireEnv('QUESTION_ID', QUESTION_ID)
  requireEnv('CORRECT_OPTION_ID', CORRECT_OPTION_ID)

  console.log('========================================')
  console.log('INTEGRIDAD ENTRE RÉPLICAS — ENTREGA 3')
  console.log('========================================')

  /*
   * LOGIN
   */

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
      tags: {
        prueba: 'integridad-replicas',
        fase: 'login',
      },
    },
  )

  // Los scripts trabajan con Bearer para no mezclar CSRF/cookies.
  descartarCookies()

  const loginOk = check(loginRes, {
    'login responde 200': r => r.status === 200,
  })

  if (!loginOk) {
    fail(`Login falló: ${loginRes.status} ${loginRes.body}`)
  }

  const loginBody = json(loginRes)
  const token = loginBody.token

  if (!token) {
    fail('El login no devolvió token')
  }

  console.log(`Login atendido por: ${instanceId(loginRes)}`)

  /*
   * I1 — SESIÓN VÁLIDA ENTRE RÉPLICAS
   */

  console.log('')
  console.log('--- I1: sesión entre réplicas ---')

  const instanciasMe = new Set()
  let identidadEsperada = null

  for (let i = 0; i < REPLICA_PROBES; i += 1) {
    const res = http.get(
      `${BASE_URL}/me`,
      {
        headers: authHeaders(token),
        tags: {
          prueba: 'I1',
          fase: 'me',
        },
      },
    )

    check(res, {
      'I1 /me responde 200': r => r.status === 200,
    })

    if (res.status !== 200) {
      fail(`I1 falló /me: ${res.status} ${res.body}`)
    }

    const instancia = instanceId(res)

    if (instancia !== 'sin-instance-id') {
      instanciasMe.add(instancia)
    }

    const usuario = json(res)

    const identidadActual =
      usuario.id ||
      usuario.ID ||
      usuario.email

    if (identidadEsperada === null) {
      identidadEsperada = identidadActual
    }

    check(null, {
      'I1 identidad consistente entre solicitudes': () =>
        identidadActual === identidadEsperada,
    })

    console.log(
      `I1 solicitud ${i + 1}: ${instancia}`,
    )
  }

  console.log(
    `I1 instancias observadas: ${Array.from(instanciasMe).join(', ')}`,
  )

  if (REQUIRE_TWO_INSTANCES) {
    check(null, {
      'I1 se observaron al menos 2 instancias': () =>
        instanciasMe.size >= 2,
    })
  }

  /*
   * Crear intento para I2 e I3
   */

  const attemptRes = http.post(
    `${BASE_URL}/quizzes/${QUIZ_ID}/attempts`,
    null,
    {
      headers: authHeaders(token),
      tags: {
        prueba: 'integridad-replicas',
        fase: 'crear-attempt',
      },
    },
  )

  const attemptOk = check(attemptRes, {
    'crear attempt responde 201': r => r.status === 201,
  })

  if (!attemptOk) {
    fail(
      `Crear attempt falló: ${attemptRes.status} ${attemptRes.body}`,
    )
  }

  const attempt = json(attemptRes)
  const attemptId =
    attempt.id ||
    attempt.ID

  if (!attemptId) {
    fail(`No se encontró attempt id: ${attemptRes.body}`)
  }

  console.log('')
  console.log(`Attempt creado: ${attemptId}`)
  console.log(
    `Creación atendida por: ${instanceId(attemptRes)}`,
  )

  /*
   * I2 — ESCRITURA EN UNA RÉPLICA Y LECTURA DESDE OTRA
   */

  console.log('')
  console.log('--- I2: escritura / lectura consistente ---')

  const writeRes = http.put(
    `${BASE_URL}/attempts/${attemptId}/answers/${QUESTION_ID}`,
    JSON.stringify({
      selected_option_id: CORRECT_OPTION_ID,
    }),
    {
      headers: authHeaders(token, {
        'Content-Type': 'application/json',
      }),
      tags: {
        prueba: 'I2',
        fase: 'write-answer',
      },
    },
  )

  const writeOk = check(writeRes, {
    'I2 escritura responde 200': r => r.status === 200,
  })

  if (!writeOk) {
    fail(
      `I2 escritura falló: ${writeRes.status} ${writeRes.body}`,
    )
  }

  const instanciaEscritura = instanceId(writeRes)

  console.log(
    `I2 escritura atendida por: ${instanciaEscritura}`,
  )

  const instanciasLectura = new Set()
  let lecturaDesdeOtraReplica = false
  let respuestaPersistida = false

  for (let i = 0; i < REPLICA_PROBES; i += 1) {
    const readRes = http.get(
      `${BASE_URL}/attempts/${attemptId}`,
      {
        headers: authHeaders(token),
        tags: {
          prueba: 'I2',
          fase: 'read-attempt',
        },
      },
    )

    check(readRes, {
      'I2 lectura responde 200': r => r.status === 200,
    })

    if (readRes.status !== 200) {
      fail(
        `I2 lectura falló: ${readRes.status} ${readRes.body}`,
      )
    }

    const instanciaLectura = instanceId(readRes)

    if (instanciaLectura !== 'sin-instance-id') {
      instanciasLectura.add(instanciaLectura)
    }

    if (
      instanciaEscritura !== 'sin-instance-id' &&
      instanciaLectura !== 'sin-instance-id' &&
      instanciaLectura !== instanciaEscritura
    ) {
      lecturaDesdeOtraReplica = true
    }

    const detalle = json(readRes)
    const answers = detalle.answers || []

    const encontrada = answers.find(
      answer =>
        String(answerQuestionId(answer)) ===
        String(QUESTION_ID),
    )

    if (
      encontrada &&
      String(answerOptionId(encontrada)) ===
        String(CORRECT_OPTION_ID)
    ) {
      respuestaPersistida = true
    }

    console.log(
      `I2 lectura ${i + 1}: ${instanciaLectura}`,
    )
  }

  check(null, {
    'I2 la respuesta escrita aparece en lecturas posteriores': () =>
      respuestaPersistida,
  })

  if (REQUIRE_TWO_INSTANCES) {
    check(null, {
      'I2 al menos una lectura fue atendida por otra réplica': () =>
        lecturaDesdeOtraReplica,
    })
  }

  console.log(
    `I2 instancias de lectura: ${Array.from(instanciasLectura).join(', ')}`,
  )

  /*
   * I3 — DOBLE SUBMIT / IDEMPOTENCIA
   */

  console.log('')
  console.log('--- I3: submit idempotente ---')

  const idempotencyKey =
    `e3-integridad-${Date.now()}-${attemptId}`

  const submitUrl =
    `${BASE_URL}/attempts/${attemptId}/submit`

  const submitParams = {
    headers: authHeaders(token, {
      'Idempotency-Key': idempotencyKey,
    }),
    tags: {
      prueba: 'I3',
      fase: 'concurrent-submit',
    },
  }

  // Cuatro solicitudes simultáneas aumentan la posibilidad de que,
  // detrás del balanceador, participen varias instancias.
  const submits = http.batch([
    {
      method: 'POST',
      url: submitUrl,
      body: null,
      params: submitParams,
    },
    {
      method: 'POST',
      url: submitUrl,
      body: null,
      params: submitParams,
    },
    {
      method: 'POST',
      url: submitUrl,
      body: null,
      params: submitParams,
    },
    {
      method: 'POST',
      url: submitUrl,
      body: null,
      params: submitParams,
    },
  ])

  const submitInstances = new Set()

  const resultados = submits.map((res, index) => {
    const instancia = instanceId(res)

    if (instancia !== 'sin-instance-id') {
      submitInstances.add(instancia)
    }

    check(res, {
      [`I3 submit ${index + 1} responde 200`]:
        r => r.status === 200,
    })

    console.log(
      `I3 submit ${index + 1}: ${instancia} → HTTP ${res.status}`,
    )

    return json(res)
  })

  const primerResultado = resultados[0]

  const submittedAtEsperado =
    primerResultado.submitted_at ||
    primerResultado.submittedAt ||
    primerResultado.SubmittedAt

  const scoreEsperado =
    primerResultado.score ??
    primerResultado.Score

  check(null, {
    'I3 todos devuelven el mismo submitted_at': () =>
      Boolean(submittedAtEsperado) &&
      resultados.every(resultado => {
        const value =
          resultado.submitted_at ||
          resultado.submittedAt ||
          resultado.SubmittedAt

        return value === submittedAtEsperado
      }),

    'I3 todos devuelven el mismo score': () =>
      scoreEsperado !== undefined &&
      resultados.every(resultado => {
        const value =
          resultado.score ??
          resultado.Score

        return value === scoreEsperado
      }),
  })

  if (REQUIRE_TWO_INSTANCES) {
    check(null, {
      'I3 los submits fueron atendidos por al menos 2 réplicas': () =>
        submitInstances.size >= 2,
    })
  }

  /*
   * Verificación final persistida
   */

  const finalRes = http.get(
    `${BASE_URL}/attempts/${attemptId}`,
    {
      headers: authHeaders(token),
      tags: {
        prueba: 'I3',
        fase: 'final-state',
      },
    },
  )

  const finalBody = json(finalRes)
  const finalAttempt =
    finalBody.attempt ||
    finalBody

  check(finalRes, {
    'I3 consulta final responde 200': r => r.status === 200,
  })

  check(null, {
    'I3 estado final es submitted': () =>
      (
        finalAttempt.status ||
        finalAttempt.Status
      ) === 'submitted',
  })

  const finalSubmittedAt =
    finalAttempt.submitted_at ||
    finalAttempt.submittedAt ||
    finalAttempt.SubmittedAt

  const finalScore =
    finalAttempt.score ??
    finalAttempt.Score

  check(null, {
    'I3 estado persistido conserva submitted_at': () =>
      Boolean(submittedAtEsperado) &&
      finalSubmittedAt === submittedAtEsperado,

    'I3 estado persistido conserva score': () =>
      finalScore === scoreEsperado,
  })

  console.log('')
  console.log('========================================')
  console.log('RESUMEN')
  console.log('========================================')
  console.log(
    `I1 instancias: ${Array.from(instanciasMe).join(', ')}`,
  )
  console.log(
    `I2 escritura: ${instanciaEscritura}`,
  )
  console.log(
    `I2 lecturas: ${Array.from(instanciasLectura).join(', ')}`,
  )
  console.log(
    `I3 submits: ${Array.from(submitInstances).join(', ')}`,
  )
  console.log(`Attempt: ${attemptId}`)
  console.log(`Idempotency-Key: ${idempotencyKey}`)
  console.log(`Score: ${finalScore}`)
  console.log(`Submitted at: ${finalSubmittedAt}`)
  console.log('========================================')
}