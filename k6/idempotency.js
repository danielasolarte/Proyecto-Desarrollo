import http from 'k6/http';
import { check, fail } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';

const STUDENT_EMAIL = __ENV.STUDENT_EMAIL;
const STUDENT_PASSWORD = __ENV.STUDENT_PASSWORD;

const QUIZ_ID = __ENV.QUIZ_ID;
const QUESTION_ID = __ENV.QUESTION_ID;
const CORRECT_OPTION_ID = __ENV.CORRECT_OPTION_ID;

export const options = {
  vus: 1,
  iterations: 1,

  thresholds: {
    checks: ['rate==1'],
    http_req_failed: ['rate==0'],
  },
};

function requireEnv(name, value) {
  if (!value) {
    fail(`Falta la variable de entorno ${name}`);
  }
}

function body(res) {
  try {
    return res.json();
  } catch (_) {
    return {};
  }
}

export default function () {
  requireEnv('STUDENT_EMAIL', STUDENT_EMAIL);
  requireEnv('STUDENT_PASSWORD', STUDENT_PASSWORD);
  requireEnv('QUIZ_ID', QUIZ_ID);
  requireEnv('QUESTION_ID', QUESTION_ID);
  requireEnv('CORRECT_OPTION_ID', CORRECT_OPTION_ID);

  // 1. Login del estudiante.
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
        test: 'idempotency',
        phase: 'login',
      },
    },
  );

  const loginOk = check(loginRes, {
    'login status 200': (r) => r.status === 200,
  });

  if (!loginOk) {
    fail(`Login falló: ${loginRes.status} ${loginRes.body}`);
  }

  const token = body(loginRes).token;

  if (!token) {
    fail('Login no devolvió token');
  }

  // 2. Crear un attempt nuevo.
  const attemptRes = http.post(
    `${BASE_URL}/quizzes/${QUIZ_ID}/attempts`,
    null,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
      tags: {
        test: 'idempotency',
        phase: 'create-attempt',
      },
    },
  );

  const attemptOk = check(attemptRes, {
    'create attempt status 201': (r) => r.status === 201,
  });

  if (!attemptOk) {
    fail(`Crear attempt falló: ${attemptRes.status} ${attemptRes.body}`);
  }

  const attempt = body(attemptRes);
  const attemptId = attempt.id || attempt.ID;

  if (!attemptId) {
    fail(`No se encontró attempt id: ${attemptRes.body}`);
  }

  // 3. Responder la pregunta.
  const answerRes = http.put(
    `${BASE_URL}/attempts/${attemptId}/answers/${QUESTION_ID}`,
    JSON.stringify({
      selected_option_id: CORRECT_OPTION_ID,
    }),
    {
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      tags: {
        test: 'idempotency',
        phase: 'answer',
      },
    },
  );

  const answerOk = check(answerRes, {
    'answer status 200': (r) => r.status === 200,
  });

  if (!answerOk) {
    fail(`Responder pregunta falló: ${answerRes.status} ${answerRes.body}`);
  }

  // 4. Misma Idempotency-Key para dos requests simultáneos.
  const idempotencyKey = `idempotency-test-${Date.now()}`;

  const submitUrl = `${BASE_URL}/attempts/${attemptId}/submit`;

  const params = {
    headers: {
      Authorization: `Bearer ${token}`,
      'Idempotency-Key': idempotencyKey,
    },
    tags: {
      test: 'idempotency',
      phase: 'concurrent-submit',
    },
  };

  const [submit1, submit2] = http.batch([
    {
      method: 'POST',
      url: submitUrl,
      body: null,
      params,
    },
    {
      method: 'POST',
      url: submitUrl,
      body: null,
      params,
    },
  ]);

  check(submit1, {
    'concurrent submit 1 status 200': (r) => r.status === 200,
  });

  check(submit2, {
    'concurrent submit 2 status 200': (r) => r.status === 200,
  });

  const result1 = body(submit1);
  const result2 = body(submit2);

  const submittedAt1 =
    result1.submitted_at ||
    result1.submittedAt ||
    result1.SubmittedAt;

  const submittedAt2 =
    result2.submitted_at ||
    result2.submittedAt ||
    result2.SubmittedAt;

  check(null, {
    'same submitted_at returned by both requests': () =>
      Boolean(submittedAt1) &&
      Boolean(submittedAt2) &&
      submittedAt1 === submittedAt2,
  });

  const score1 = result1.score ?? result1.Score;
  const score2 = result2.score ?? result2.Score;

  check(null, {
    'same score returned by both requests': () =>
      score1 !== undefined &&
      score2 !== undefined &&
      score1 === score2,
  });

  // 5. Replay posterior con la misma key.
  const replayRes = http.post(
    submitUrl,
    null,
    params,
  );

  check(replayRes, {
    'idempotent replay status 200': (r) => r.status === 200,
  });

  const replay = body(replayRes);

  const replaySubmittedAt =
    replay.submitted_at ||
    replay.submittedAt ||
    replay.SubmittedAt;

  check(null, {
    'replay returns same submitted_at': () =>
      Boolean(submittedAt1) &&
      Boolean(replaySubmittedAt) &&
      submittedAt1 === replaySubmittedAt,
  });

  console.log(`Attempt: ${attemptId}`);
  console.log(`Idempotency-Key: ${idempotencyKey}`);
  console.log(`submitted_at: ${submittedAt1}`);
  console.log('Idempotency test completed.');
}