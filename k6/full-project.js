import http from 'k6/http';
import { check, fail, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const PROFILE = __ENV.PROFILE || 'smoke';
const TOTAL_VUS = Number(__ENV.TOTAL_VUS || 100);

const ADMIN_EMAIL = __ENV.ADMIN_EMAIL || 'admin.k6@mooc.test';
const ADMIN_PASSWORD = __ENV.ADMIN_PASSWORD || 'Admin12345';
const TEACHER_PASSWORD = __ENV.TEACHER_PASSWORD || 'Teacher12345';
const STUDENT_PASSWORD = __ENV.STUDENT_PASSWORD || 'Student12345';

const PLAYBACK_RESOURCE_ID = __ENV.PLAYBACK_RESOURCE_ID || '';
const VERIFICATION_CODE = __ENV.VERIFICATION_CODE || '';

const profileConfig = {
  smoke: {
    studentPool: 5,
    ramp: '5s',
    hold: '15s',
    down: '5s',
    vus: {
      student: 5,
      public: 2,
      auth: 1,
      admin: 1,
      authoring: 1,
      media: 1,
    },
  },
  local: {
    studentPool: 60,
    ramp: '15s',
    hold: '60s',
    down: '15s',
    vus: {
      student: 60,
      public: 20,
      auth: 5,
      admin: 5,
      authoring: 5,
      media: 5,
    },
  },
  acceptance: {
    // Perfil de aceptación: 2.000 VUs concurrentes en total.
    studentPool: 100,
    ramp: '30s',
    hold: '90s',
    down: '30s',
    vus: {
      student: 1500,
      public: 250,
      auth: 75,
      admin: 50,
      authoring: 75,
      media: 50,
    },
  },
};

if (!profileConfig[PROFILE] && PROFILE !== 'capacity') {
  throw new Error(
    `PROFILE inválido: ${PROFILE}. Usa smoke, local, acceptance o capacity.`
  );
}

const cfg = PROFILE === 'capacity'
  ? { studentPool: 20 }
  : profileConfig[PROFILE];

const STUDENT_POOL = Number(__ENV.STUDENT_POOL || cfg.studentPool);

function ramping(exec, target) {
  return {
    executor: 'ramping-vus',
    exec,
    startVUs: 0,
    stages: [
      { duration: cfg.ramp, target },
      { duration: cfg.hold, target },
      { duration: cfg.down, target: 0 },
    ],
    gracefulRampDown: '10s',
  };
}

function distributeVUs(total) {
  const student = Math.max(1, Math.round(total * 0.125));
  const auth = Math.max(1, Math.round(total * 0.15));
  const admin = Math.max(1, Math.round(total * 0.075));
  const authoring = Math.max(1, Math.round(total * 0.075));
  const media = Math.max(1, Math.round(total * 0.075));

  const publicReads =
    total - student - auth - admin - authoring - media;

  return {
    student,
    public: publicReads,
    auth,
    admin,
    authoring,
    media,
  };
}

function capacityScenario(exec, vus) {
  return {
    executor: 'ramping-vus',
    exec,
    startVUs: 0,
    stages: [
      { duration: '15s', target: vus },
      { duration: '45s', target: vus },
      { duration: '15s', target: 0 },
    ],
    gracefulRampDown: '10s',
  };
}

const capacityVus = distributeVUs(TOTAL_VUS);

const scenarios =
  PROFILE === 'capacity'
    ? {
        student_learning: capacityScenario(
          'studentLearning',
          capacityVus.student
        ),

        public_reads: capacityScenario(
          'publicReads',
          capacityVus.public
        ),

        authentication: capacityScenario(
          'authentication',
          capacityVus.auth
        ),

        administration: capacityScenario(
          'administration',
          capacityVus.admin
        ),

        authoring_editor: capacityScenario(
          'authoringEditor',
          capacityVus.authoring
        ),

        multimedia: capacityScenario(
          'multimedia',
          capacityVus.media
        ),
      }
    : {
        student_learning: ramping(
          'studentLearning',
          cfg.vus.student
        ),

        public_reads: ramping(
          'publicReads',
          cfg.vus.public
        ),

        authentication: ramping(
          'authentication',
          cfg.vus.auth
        ),

        administration: ramping(
          'administration',
          cfg.vus.admin
        ),

        authoring_editor: ramping(
          'authoringEditor',
          cfg.vus.authoring
        ),

        multimedia: ramping(
          'multimedia',
          cfg.vus.media
        ),
      };

export const options = {
  scenarios,

  // El enunciado exige evidenciar p95 y límites de rendimiento, pero no fija
  // un número de latencia concreto. Estos son objetivos documentados por el
  // equipo para la Etapa 1 y pueden endurecerse si el curso define otros.
  thresholds: {
    checks: ['rate>0.99'],
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<1000'],

    'http_req_duration{module:catalog}': ['p(95)<500'],
    'http_req_duration{module:courses}': ['p(95)<700'],
    'http_req_duration{module:auth}': ['p(95)<1500'],
    'http_req_duration{module:admin}': ['p(95)<800'],
    'http_req_duration{module:quizzes}': ['p(95)<1000'],
    'http_req_duration{module:progress}': ['p(95)<1000'],
    'http_req_duration{module:media}': ['p(95)<1500'],
    'http_req_duration{module:badges}': ['p(95)<700'],
  },
};

const functionalErrors = new Rate('functional_errors');

function headers(token = '') {
  const h = { 'Content-Type': 'application/json' };
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

function json(res) {
  try {
    return res.json();
  } catch (_) {
    return {};
  }
}

function pick(obj, ...keys) {
  for (const key of keys) {
    if (obj && obj[key] !== undefined && obj[key] !== null) return obj[key];
  }
  return undefined;
}

function assertStatus(res, allowed, label) {
  const ok = check(res, {
    [`${label} status ${allowed.join('/')}`]: (r) => allowed.includes(r.status),
  });
  functionalErrors.add(!ok);
  return ok;
}

function post(path, body, token, module, expected = [200, 201]) {
  const res = http.post(
    `${BASE_URL}${path}`,
    body === undefined ? null : JSON.stringify(body),
    { headers: headers(token), tags: { module } },
  );
  assertStatus(res, expected, `${module} POST ${path}`);
  return res;
}

function put(path, body, token, module, expected = [200]) {
  const res = http.put(
    `${BASE_URL}${path}`,
    JSON.stringify(body),
    { headers: headers(token), tags: { module } },
  );
  assertStatus(res, expected, `${module} PUT ${path}`);
  return res;
}

function get(path, token, module, expected = [200]) {
  const h = token ? { Authorization: `Bearer ${token}` } : {};
  const res = http.get(
    `${BASE_URL}${path}`,
    { headers: h, tags: { module } },
  );
  assertStatus(res, expected, `${module} GET ${path}`);
  return res;
}

function setupMust(res, allowed, label) {
  if (!allowed.includes(res.status)) {
    fail(`${label} falló. status=${res.status} body=${res.body}`);
  }
  return json(res);
}

function login(email, password) {
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email, password }),
    { headers: headers(), tags: { module: 'auth', phase: 'setup' } },
  );
  if (res.status !== 200) return null;
  return json(res);
}

function ensureAdmin() {
  let logged = login(ADMIN_EMAIL, ADMIN_PASSWORD);
  if (logged && logged.token) return logged.token;

  const boot = http.post(
    `${BASE_URL}/auth/bootstrap-admin`,
    JSON.stringify({
      email: ADMIN_EMAIL,
      password: ADMIN_PASSWORD,
      full_name: 'Admin k6',
    }),
    { headers: headers(), tags: { module: 'auth', phase: 'setup' } },
  );

  if (![201, 409].includes(boot.status)) {
    fail(`Bootstrap admin falló: status=${boot.status} body=${boot.body}`);
  }

  logged = login(ADMIN_EMAIL, ADMIN_PASSWORD);
  if (!logged || !logged.token) {
    fail(
      `No fue posible iniciar sesión con ${ADMIN_EMAIL}. ` +
      `Si ya existe otro admin en la BD, ejecuta k6 con ` +
      `-e ADMIN_EMAIL=<correo> -e ADMIN_PASSWORD=<clave>, o usa una BD limpia.`
    );
  }

  return logged.token;
}

function createTeacher(adminToken, runId) {
  const email = `teacher.k6.${runId}@mooc.test`;
  const res = http.post(
    `${BASE_URL}/admin/users/teachers`,
    JSON.stringify({
      email,
      password: TEACHER_PASSWORD,
      full_name: 'Profesor k6',
    }),
    { headers: headers(adminToken), tags: { module: 'admin', phase: 'setup' } },
  );
  setupMust(res, [201], 'Crear profesor');

  const logged = login(email, TEACHER_PASSWORD);
  if (!logged || !logged.token) fail('Login profesor falló durante setup.');

  return { email, token: logged.token };
}

function createCourse(teacherToken, runId, titleSuffix) {
  const res = http.post(
    `${BASE_URL}/courses`,
    JSON.stringify({
      title: `Curso k6 ${titleSuffix} ${runId}`,
      summary: `Curso sintético para prueba de carga ${titleSuffix}`,
      category: 'load-test',
    }),
    { headers: headers(teacherToken), tags: { module: 'courses', phase: 'setup' } },
  );
  const body = setupMust(res, [201], `Crear curso ${titleSuffix}`);
  return {
    courseId: pick(body.course, 'id', 'ID'),
    versionId: pick(body.version, 'id', 'ID'),
  };
}

function createModule(teacherToken, versionId, title) {
  const res = http.post(
    `${BASE_URL}/versions/${versionId}/modules`,
    JSON.stringify({ title, position: 1 }),
    { headers: headers(teacherToken), tags: { module: 'courses', phase: 'setup' } },
  );
  const body = setupMust(res, [201], `Crear módulo ${title}`);
  return pick(body, 'id', 'ID');
}

function createUnit(teacherToken, moduleId, title) {
  const res = http.post(
    `${BASE_URL}/modules/${moduleId}/units`,
    JSON.stringify({ title, position: 1 }),
    { headers: headers(teacherToken), tags: { module: 'courses', phase: 'setup' } },
  );
  const body = setupMust(res, [201], `Crear unidad ${title}`);
  return pick(body, 'id', 'ID');
}

function createResource(teacherToken, unitId, resource) {
  const res = http.post(
    `${BASE_URL}/units/${unitId}/resources`,
    JSON.stringify(resource),
    { headers: headers(teacherToken), tags: { module: 'courses', phase: 'setup' } },
  );
  const body = setupMust(res, [201], `Crear recurso ${resource.title}`);
  return pick(body, 'id', 'ID');
}

function buildLearningCourse(teacherToken, runId) {
  const { courseId, versionId } = createCourse(teacherToken, runId, 'learning');
  const moduleId = createModule(teacherToken, versionId, 'Módulo Quiz');
  const unitId = createUnit(teacherToken, moduleId, 'Unidad Quiz');

  const quizResourceId = createResource(teacherToken, unitId, {
    type: 'quiz',
    title: 'Quiz de carga',
    position: 1,
    visible: true,
    required: true,
    downloadable: false,
  });

  const quizRes = http.post(
    `${BASE_URL}/resources/${quizResourceId}/quiz`,
    JSON.stringify({
      passing_score: 60,
      feedback_mode: 'after_submit',
    }),
    { headers: headers(teacherToken), tags: { module: 'quizzes', phase: 'setup' } },
  );
  const quiz = setupMust(quizRes, [201], 'Crear quiz');
  const quizId = pick(quiz, 'id', 'ID');

  const questionRes = http.post(
    `${BASE_URL}/quizzes/${quizId}/questions`,
    JSON.stringify({
      text: '¿Cuánto es 2 + 2?',
      position: 1,
      points: 1,
    }),
    { headers: headers(teacherToken), tags: { module: 'quizzes', phase: 'setup' } },
  );
  const question = setupMust(questionRes, [201], 'Crear pregunta');
  const questionId = pick(question, 'id', 'ID');

  const correctRes = http.post(
    `${BASE_URL}/questions/${questionId}/options`,
    JSON.stringify({ text: '4', position: 1, is_correct: true }),
    { headers: headers(teacherToken), tags: { module: 'quizzes', phase: 'setup' } },
  );
  const correct = setupMust(correctRes, [201], 'Crear opción correcta');
  const correctOptionId = pick(correct, 'id', 'ID');

  const wrongRes = http.post(
    `${BASE_URL}/questions/${questionId}/options`,
    JSON.stringify({ text: '5', position: 2, is_correct: false }),
    { headers: headers(teacherToken), tags: { module: 'quizzes', phase: 'setup' } },
  );
  setupMust(wrongRes, [201], 'Crear opción incorrecta');

  const badgeRes = http.post(
    `${BASE_URL}/courses/${courseId}/badge`,
    JSON.stringify({
      name: 'Badge k6',
      description: 'Insignia sintética de prueba de carga',
      image_url: 'https://example.com/badge-k6.png',
    }),
    { headers: headers(teacherToken), tags: { module: 'badges', phase: 'setup' } },
  );
  setupMust(badgeRes, [201], 'Crear badge');

  const publishRes = http.post(
    `${BASE_URL}/courses/${courseId}/publish`,
    null,
    { headers: headers(teacherToken), tags: { module: 'courses', phase: 'setup' } },
  );
  setupMust(publishRes, [200], 'Publicar curso');

  return { courseId, quizId, questionId, correctOptionId, quizResourceId };
}

function buildAuthoringCourse(teacherToken, runId) {
  const { courseId, versionId } = createCourse(teacherToken, runId, 'authoring');
  const moduleId = createModule(teacherToken, versionId, 'Módulo Editor');
  const unitId = createUnit(teacherToken, moduleId, 'Unidad Editor');

  const resourceId = createResource(teacherToken, unitId, {
    type: 'rich_text',
    title: 'Contenido Markdown k6',
    position: 1,
    visible: true,
    required: true,
    downloadable: false,
  });

  return { courseId, resourceId };
}

function buildMediaCourse(teacherToken, runId) {
  const { courseId, versionId } = createCourse(teacherToken, runId, 'media');
  const moduleId = createModule(teacherToken, versionId, 'Módulo Multimedia');
  const unitId = createUnit(teacherToken, moduleId, 'Unidad Multimedia');

  const resourceId = createResource(teacherToken, unitId, {
    type: 'video',
    title: 'Video sintético k6',
    position: 1,
    visible: true,
    required: false,
    downloadable: false,
  });

  return { courseId, resourceId };
}

function createStudent(index, runId, learningCourseId) {
  const email = `student.k6.${runId}.${index}@mooc.test`;

  const registerRes = http.post(
    `${BASE_URL}/auth/register`,
    JSON.stringify({
      email,
      password: STUDENT_PASSWORD,
      full_name: `Estudiante k6 ${index}`,
    }),
    { headers: headers(), tags: { module: 'auth', phase: 'setup' } },
  );
  const registered = setupMust(registerRes, [201], `Registrar estudiante ${index}`);
  const verificationToken = pick(
    registered,
    'verification_token_dev',
    'verificationTokenDev',
  );

  const verifyRes = http.post(
    `${BASE_URL}/auth/verify-email`,
    JSON.stringify({ token: verificationToken }),
    { headers: headers(), tags: { module: 'auth', phase: 'setup' } },
  );
  setupMust(verifyRes, [200], `Verificar estudiante ${index}`);

  const logged = login(email, STUDENT_PASSWORD);
  if (!logged || !logged.token) fail(`Login estudiante ${index} falló.`);

  const enrollRes = http.post(
    `${BASE_URL}/courses/${learningCourseId}/enrollments`,
    null,
    { headers: headers(logged.token), tags: { module: 'catalog', phase: 'setup' } },
  );
  setupMust(enrollRes, [201], `Inscribir estudiante ${index}`);

  return { email, token: logged.token };
}

export function setup() {
  const runId = `${Date.now()}`;

  const adminToken = ensureAdmin();
  const teacher = createTeacher(adminToken, runId);

  const learning = buildLearningCourse(teacher.token, runId);
  const authoring = buildAuthoringCourse(teacher.token, runId);
  const media = buildMediaCourse(teacher.token, runId);

  const students = [];
  for (let i = 0; i < STUDENT_POOL; i += 1) {
    students.push(createStudent(i, runId, learning.courseId));
  }

  return {
    runId,
    adminToken,
    teacher,
    students,
    learning,
    authoring,
    media,
  };
}

function studentForVU(data) {
  return data.students[(__VU - 1) % data.students.length];
}

let capacityStudent = null;

function capacityStudentForVU(data) {
  if (capacityStudent) {
    return capacityStudent;
  }

  capacityStudent = createStudent(
    `capacity-vu-${__VU}`,
    data.runId,
    data.learning.courseId
  );

  return capacityStudent;
}

export function publicReads(data) {
  get('/catalog?q=k6&category=load-test', '', 'catalog', [200]);
  get(`/courses/${data.learning.courseId}`, '', 'courses', [200]);
  get(`/courses/${data.learning.courseId}/preview`, '', 'courses', [200]);

  if (VERIFICATION_CODE) {
    get(`/badges/verify/${VERIFICATION_CODE}`, '', 'badges', [200]);
  }

  sleep(0.2);
}

export function authentication(data) {
  const student = studentForVU(data);
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({
      email: student.email,
      password: STUDENT_PASSWORD,
    }),
    { headers: headers(), tags: { module: 'auth' } },
  );
  assertStatus(res, [200], 'auth login');
  sleep(0.5);
}

export function administration(data) {
  get('/admin/users', data.adminToken, 'admin', [200]);
  get('/admin/audit-log', data.adminToken, 'admin', [200]);
  sleep(0.5);
}

export function authoringEditor(data) {
  const token = data.teacher.token;
  const markdown = `# Autosave k6\n\nVU=${__VU}, iteración=${__ITER}, ts=${Date.now()}`;

  put(
    `/resources/${data.authoring.resourceId}/content/draft`,
    { markdown },
    token,
    'courses',
    [200],
  );

  get(
    `/resources/${data.authoring.resourceId}/content/draft`,
    token,
    'courses',
    [200],
  );

  get(
    `/courses/${data.authoring.courseId}/preview`,
    token,
    'courses',
    [200],
  );

  get(
    `/quizzes/${data.learning.quizId}/authoring`,
    token,
    'quizzes',
    [200],
  );

  sleep(0.4);
}

export function multimedia(data) {
  const token = data.teacher.token;

  const initRes = post(
    `/resources/${data.media.resourceId}/uploads/initiate`,
    {
      mime_type: 'video/mp4',
      size_bytes: 1048576,
      checksum_sha256: '',
    },
    token,
    'media',
    [201],
  );

  if (initRes.status === 201) {
    const body = json(initRes);
    const session = pick(body, 'upload_session', 'uploadSession') || {};
    const sessionId = pick(session, 'ID', 'id');

    if (sessionId) {
      get(`/uploads/${sessionId}/parts/1/url`, token, 'media', [200]);
      get(`/uploads/${sessionId}/parts`, token, 'media', [200]);
    }
  }

  // Si se proporciona un recurso ya procesado, también se carga la ruta de
  // consumo/playback real de Persona 3.
  if (PLAYBACK_RESOURCE_ID) {
    get(`/resources/${PLAYBACK_RESOURCE_ID}/media`, token, 'media', [200]);
    get(`/resources/${PLAYBACK_RESOURCE_ID}/playback`, token, 'media', [200]);
  }

  sleep(0.5);
}

export function studentLearning(data) {
  const student =
    PROFILE === 'capacity'
      ? capacityStudentForVU(data)
      : studentForVU(data);
  const token = student.token;

  get('/me/enrollments', token, 'catalog', [200]);

  const attemptRes = post(
    `/quizzes/${data.learning.quizId}/attempts`,
    undefined,
    token,
    'quizzes',
    [201],
  );

  if (attemptRes.status !== 201) {
    sleep(0.2);
    return;
  }

  const attempt = json(attemptRes);
  const attemptId = pick(attempt, 'id', 'ID');
  if (!attemptId) {
    functionalErrors.add(true);
    sleep(0.2);
    return;
  }

  put(
    `/attempts/${attemptId}/answers/${data.learning.questionId}`,
    { selected_option_id: data.learning.correctOptionId },
    token,
    'quizzes',
    [200],
  );

  const idempotencyKey = `k6-${data.runId}-${__VU}-${__ITER}-${Date.now()}`;

  const submitParams = {
    headers: {
      Authorization: `Bearer ${token}`,
      'Idempotency-Key': idempotencyKey,
    },
    tags: {
      module: 'quizzes',
      test: 'concurrent-idempotency',
    },
  };

  const submitUrl = `${BASE_URL}/attempts/${attemptId}/submit`;

  // Dos requests simultáneos contra el mismo attempt y con la misma key.
  const [submitRes1, submitRes2] = http.batch([
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
  ]);

  assertStatus(
    submitRes1,
    [200],
    'quizzes concurrent submit 1',
  );

  assertStatus(
    submitRes2,
    [200],
    'quizzes concurrent submit 2',
  );

  const submittedAttempt1 = json(submitRes1);
  const submittedAttempt2 = json(submitRes2);

  const submittedAt1 = pick(
    submittedAttempt1,
    'submitted_at',
    'submittedAt',
    'SubmittedAt',
  );

  const submittedAt2 = pick(
    submittedAttempt2,
    'submitted_at',
    'submittedAt',
    'SubmittedAt',
  );

  if (
    submittedAt1 &&
    submittedAt2 &&
    submittedAt1 !== submittedAt2
  ) {
    functionalErrors.add(true);

    console.error(
      `Concurrent idempotency failed for attempt ${attemptId}: ` +
      `submitted_at differs (${submittedAt1} != ${submittedAt2})`,
    );
  }

  // Repetición inmediata con la misma clave: carga también el camino
  // idempotente, sin crear un segundo resultado.
  const replayRes = http.post(
    `${BASE_URL}/attempts/${attemptId}/submit`,
    null,
    {
      headers: {
        Authorization: `Bearer ${token}`,
        'Idempotency-Key': idempotencyKey,
      },
      tags: { module: 'quizzes' },
    },
  );
  assertStatus(replayRes, [200], 'quizzes submit idempotent replay');

  get(
    `/courses/${data.learning.courseId}/progress`,
    token,
    'progress',
    [200],
  );

  sleep(0.2);
}
