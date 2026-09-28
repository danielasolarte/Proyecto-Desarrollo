// Escenario 1 (Entrega 2): actividad académica concurrente.
//
// Estudiantes que consultan el catálogo, acceden a cursos, se inscriben,
// registran progreso válido y presentan quizzes. Las sesiones (tokens) se
// preparan en setup(), ANTES de la corrida: el login NO forma parte del
// recorrido medido. La ráfaga de logins se mide aparte con PROFILE=login_burst.
//
// Perfiles:
//   smoke        prueba corta para validar el script (6 VUs)
//   level        un nivel de carga: TOTAL_VUS, WARMUP_S y HOLD_S
//   login_burst  variante separada: solo inicios de sesión
//
// La mezcla de VUs es SIEMPRE la misma en cada nivel (50% lectores,
// 30% progreso, 20% quizzes); solo cambia el total de VUs.

import http from 'k6/http';
import { check, fail, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';

// ---------- Configuración ----------
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const PROFILE = __ENV.PROFILE || 'smoke';
const TOTAL_VUS = Number(__ENV.TOTAL_VUS || 20);
const WARMUP_S = Number(__ENV.WARMUP_S || (PROFILE === 'smoke' ? 10 : 30));
const HOLD_S = Number(__ENV.HOLD_S || (PROFILE === 'smoke' ? 90 : 180));
const RAMPDOWN_S = 15;
const REQUEST_TIMEOUT = __ENV.REQUEST_TIMEOUT || '30s';

// La API exige >= 30 s de actividad acumulada antes de aceptar "complete"
// (minCompletionSeconds en internal/progress/usecase). Dos heartbeats
// espaciados 16 s suman 32 s: es una pausa dictada por la regla de negocio.
const PROGRESS_STEP_S = Number(__ENV.PROGRESS_STEP_S || 16);

const COURSES = Number(__ENV.COURSES || 3);
const TEXT_RESOURCES = Number(__ENV.TEXT_RESOURCES || 8);

const ADMIN_EMAIL = __ENV.ADMIN_EMAIL || 'admin.k6@mooc.test';
const ADMIN_PASSWORD = __ENV.ADMIN_PASSWORD || 'Admin12345';
const TEACHER_PASSWORD = __ENV.TEACHER_PASSWORD || 'Teacher12345';
const STUDENT_PASSWORD = __ENV.STUDENT_PASSWORD || 'Student12345';

// ---------- Métricas propias ----------
// Rechazos esperados por reglas de negocio no se cuentan aquí: cada
// petición declara sus códigos esperados (responseCallback), así
// http_req_failed solo cuenta fallos de solicitudes válidas.
const functionalErrors = new Rate('functional_errors');
// Envío duplicado concurrente (misma Idempotency-Key):
//  - inconsistent: el resultado observable difiere (otro intento, otra nota, error).
//  - double_grading: el resultado es igual pero AMBAS peticiones calificaron
//    (submitted_at distinto con precisión de nanosegundos = no vino de la BD).
const dupSubmitInconsistent = new Counter('dup_submit_inconsistent');
const dupSubmitDoubleGrading = new Counter('dup_submit_double_grading');
const progressRegressions = new Counter('progress_regressions');
const journeysDone = new Counter('journeys_completed');

// ---------- Escenarios ----------
function distribute(total) {
  const learners = Math.max(1, Math.round(total * 0.3));
  const quiz = Math.max(1, Math.round(total * 0.2));
  const readers = Math.max(1, total - learners - quiz);
  return { readers, learners, quiz };
}

function rampScenario(exec, vus) {
  return {
    executor: 'ramping-vus',
    exec,
    startVUs: 0,
    stages: [
      { duration: `${WARMUP_S}s`, target: vus },
      { duration: `${HOLD_S}s`, target: vus },
      { duration: `${RAMPDOWN_S}s`, target: 0 },
    ],
    gracefulRampDown: '60s',
    gracefulStop: '60s',
  };
}

const effectiveVUs = PROFILE === 'smoke' ? 6 : TOTAL_VUS;
const mix = distribute(effectiveVUs);

const scenarios =
  PROFILE === 'login_burst'
    ? { login_burst: rampScenario('loginBurst', TOTAL_VUS) }
    : {
        readers: rampScenario('reader', mix.readers),
        learners: rampScenario('learner', mix.learners),
        quiz_takers: rampScenario('quizTaker', mix.quiz),
      };

// Objetivos de p95 por operación (ms). Son criterios de éxito definidos ANTES
// de correr; no se modifican para "aprobar" una corrida.
const P95_MS = {
  catalog_search: 500,
  course_get: 500,
  course_preview: 500,
  enroll: 800,
  my_enrollments: 500,
  progress_open: 800,
  progress_heartbeat: 800,
  progress_complete: 1000,
  course_progress: 800,
  quiz_start: 1000,
  quiz_answer: 800,
  quiz_submit: 1500,
  quiz_get_attempt: 800,
  login: 1500,
};

const thresholds = {
  checks: ['rate>0.99'],
  functional_errors: ['rate<0.01'],
  dup_submit_inconsistent: ['count==0'],
  dup_submit_double_grading: ['count==0'],
  progress_regressions: ['count==0'],
  'http_req_failed{phase:steady}': ['rate<0.01'],
  'http_req_duration{phase:steady}': ['p(95)<1000'],
};
for (const op of Object.keys(P95_MS)) {
  thresholds[`http_req_duration{op:${op},phase:steady}`] = [`p(95)<${P95_MS[op]}`];
}

export const options = {
  scenarios,
  thresholds,
  setupTimeout: '10m',
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

// ---------- Utilidades ----------
let T0 = 0; // inicio real de la carga (fin de setup); por VU

function phase() {
  return Date.now() - T0 < WARMUP_S * 1000 ? 'warmup' : 'steady';
}

function tagsFor(op) {
  return { op, phase: phase() };
}

function hdr(token, extra) {
  const h = { 'Content-Type': 'application/json' };
  if (token) h.Authorization = `Bearer ${token}`;
  return Object.assign(h, extra || {});
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

function pause() {
  return 1 + Math.random() * 2; // 1-3 s entre acciones de un estudiante
}

function validate(res, tags, label, fn) {
  const ok = check(res, { [label]: fn }, tags);
  functionalErrors.add(!ok, tags);
  return ok;
}

function call(method, path, body, token, op, expected, extraHeaders) {
  const tags = tagsFor(op);
  const res = http.request(
    method,
    `${BASE_URL}${path}`,
    body === undefined || body === null ? null : JSON.stringify(body),
    {
      headers: hdr(token, extraHeaders),
      tags,
      timeout: REQUEST_TIMEOUT,
      responseCallback: http.expectedStatuses(...expected),
    },
  );
  validate(
    res,
    tags,
    `${op} responde ${expected.join('/')}`,
    (r) => expected.indexOf(r.status) !== -1,
  );
  return res;
}

let debugCount = 0;
function debugLog(msg) {
  if (debugCount < 5) {
    debugCount += 1;
    console.warn(`[e1 VU ${__VU}] ${msg}`);
  }
}

function trackPercentage(s, courseProgress, where) {
  const pct = pick(courseProgress, 'progress_percentage', 'ProgressPercentage');
  if (typeof pct !== 'number') {
    debugLog(`sin porcentaje en ${where}: ${JSON.stringify(courseProgress)}`);
    return;
  }
  if (pct + 0.01 < s.lastPct) { // la API redondea a 2 decimales al leer
    progressRegressions.add(1);
    debugLog(`retroceso de progreso en ${where}: antes=${s.lastPct} ahora=${pct}`);
  }
  s.lastPct = Math.max(s.lastPct, pct);
}

// Estado por VU (cada VU tiene su propio runtime, así que esto no se comparte).
let S = null;

function vuState(data) {
  T0 = data.startedAt;
  if (!S) {
    S = {
      student: data.students[(__VU - 1) % data.students.length],
      course: data.courses[(__VU - 1) % data.courses.length],
      enrolled: false,
      iter: 0,
      lastPct: 0,
    };
  }
  return S;
}

function ensureEnrolled(s) {
  if (s.enrolled) return;
  const res = call(
    'POST',
    `/courses/${s.course.id}/enrollments`,
    null,
    s.student.token,
    'enroll',
    [201],
  );
  s.enrolled = res.status === 201;
}

// ---------- Recorridos medidos ----------
export function reader(data) {
  const s = vuState(data);
  const cid = s.course.id;
  const token = s.student.token;

  ensureEnrolled(s);

  const cat = call(
    'GET',
    `/catalog?category=${data.category}&limit=10`,
    null,
    '',
    'catalog_search',
    [200],
  );
  validate(cat, tagsFor('catalog_search'), 'el catálogo lista el curso sembrado',
    (r) => r.body.indexOf(cid) !== -1);
  sleep(pause());

  const course = call('GET', `/courses/${cid}`, null, token, 'course_get', [200]);
  validate(course, tagsFor('course_get'), 'el detalle corresponde al curso',
    (r) => r.body.indexOf(cid) !== -1);
  sleep(pause());

  call('GET', `/courses/${cid}/preview`, null, token, 'course_preview', [200]);
  sleep(pause());

  const mine = call('GET', '/me/enrollments', null, token, 'my_enrollments', [200]);
  validate(mine, tagsFor('my_enrollments'), 'mis inscripciones incluyen el curso',
    (r) => r.body.indexOf(cid) !== -1);

  s.iter += 1;
  journeysDone.add(1, { journey: 'reader' });
  sleep(pause());
}

export function learner(data) {
  const s = vuState(data);
  const cid = s.course.id;
  const token = s.student.token;

  ensureEnrolled(s);

  // Un recurso distinto por iteración: cada vuelta escribe progreso nuevo.
  const rid = s.course.textResources[s.iter % s.course.textResources.length];

  call('POST', `/resources/${rid}/progress/open`, null, token, 'progress_open', [200]);
  sleep(PROGRESS_STEP_S);
  call('POST', `/resources/${rid}/progress/heartbeat`, {}, token, 'progress_heartbeat', [200]);
  sleep(PROGRESS_STEP_S);
  call('POST', `/resources/${rid}/progress/heartbeat`, {}, token, 'progress_heartbeat', [200]);

  const done = call('POST', `/resources/${rid}/progress/complete`, null, token,
    'progress_complete', [200]);
  validate(done, tagsFor('progress_complete'), 'el recurso queda completed', (r) => {
    const rp = json(r).resource_progress || {};
    return pick(rp, 'status', 'Status') === 'completed';
  });
  trackPercentage(s, json(done).course_progress, 'learner/complete');

  const pr = call('GET', `/courses/${cid}/progress`, null, token, 'course_progress', [200]);
  trackPercentage(s, json(pr), 'learner/GET progress');

  s.iter += 1;
  journeysDone.add(1, { journey: 'learner' });
  sleep(pause());
}

export function quizTaker(data) {
  const s = vuState(data);
  const cid = s.course.id;
  const token = s.student.token;

  ensureEnrolled(s);

  // 1. Crear intento. La clave correcta nunca debe viajar al cliente.
  const started = call('POST', `/quizzes/${s.course.quizId}/attempts`, null, token,
    'quiz_start', [201]);
  if (started.status !== 201) {
    sleep(pause());
    return;
  }
  validate(started, tagsFor('quiz_start'), 'el intento no expone is_correct',
    (r) => r.body.indexOf('is_correct') === -1);
  const attemptId = pick(json(started), 'id', 'ID');
  if (!attemptId) {
    functionalErrors.add(true, tagsFor('quiz_start'));
    sleep(pause());
    return;
  }
  sleep(pause());

  // 2. Responder (opción correcta).
  call('PUT', `/attempts/${attemptId}/answers/${s.course.questionId}`,
    { selected_option_id: s.course.correctOptionId }, token, 'quiz_answer', [200]);
  sleep(pause());

  // 3. Envío DUPLICADO CONCURRENTE con la misma Idempotency-Key.
  //    Debe existir una sola calificación: mismo intento, mismo submitted_at.
  const key = `e1-${data.runId}-${__VU}-${s.iter}`;
  const url = `${BASE_URL}/attempts/${attemptId}/submit`;
  const mk = () => ['POST', url, null, {
    headers: hdr(token, { 'Idempotency-Key': key }),
    tags: tagsFor('quiz_submit'),
    timeout: REQUEST_TIMEOUT,
    responseCallback: http.expectedStatuses(200),
  }];
  const [a, b] = http.batch([mk(), mk()]);
  const bodyA = json(a);
  const bodyB = json(b);

  const consistent = a.status === 200 && b.status === 200 &&
    pick(bodyA, 'id') === attemptId && pick(bodyB, 'id') === attemptId &&
    pick(bodyA, 'score') === 1 && pick(bodyB, 'score') === 1 &&
    pick(bodyA, 'passed') === true && pick(bodyB, 'passed') === true;
  validate(a, tagsFor('quiz_submit'), 'envío duplicado: mismo intento y misma nota', () => consistent);

  if (!consistent) {
    dupSubmitInconsistent.add(1);
    debugLog(`envío duplicado inconsistente: statusA=${a.status} statusB=${b.status} ` +
      `idA=${pick(bodyA, 'id')} idB=${pick(bodyB, 'id')} ` +
      `scoreA=${pick(bodyA, 'score')} scoreB=${pick(bodyB, 'score')}`);
  } else if (pick(bodyA, 'submitted_at') !== pick(bodyB, 'submitted_at')) {
    dupSubmitDoubleGrading.add(1);
    debugLog(`envío duplicado calificado dos veces: ` +
      `submittedA=${pick(bodyA, 'submitted_at')} submittedB=${pick(bodyB, 'submitted_at')}`);
  }

  // 4. Estado final persistido del intento.
  const fetched = call('GET', `/attempts/${attemptId}`, null, token, 'quiz_get_attempt', [200]);
  validate(fetched, tagsFor('quiz_get_attempt'), 'el intento quedó submitted y aprobado', (r) => {
    // GET /attempts/:id responde { attempt: {...}, answers: [...] }
    const attempt = json(r).attempt || {};
    return pick(attempt, 'status') === 'submitted' && pick(attempt, 'passed') === true;
  });

  // 5. El progreso del curso nunca retrocede tras aprobar.
  const pr = call('GET', `/courses/${cid}/progress`, null, token, 'course_progress', [200]);
  trackPercentage(s, json(pr), 'quiz/GET progress');

  s.iter += 1;
  journeysDone.add(1, { journey: 'quiz' });
  sleep(pause());
}

// ---------- Variante separada: ráfaga de inicios de sesión ----------
export function loginBurst(data) {
  T0 = data.startedAt;
  const student = data.students[(__VU - 1) % data.students.length];
  const res = call('POST', '/auth/login',
    { email: student.email, password: STUDENT_PASSWORD }, '', 'login', [200]);
  validate(res, tagsFor('login'), 'el login devuelve token', (r) => !!pick(json(r), 'token'));
  sleep(0.5);
}

// ---------- Preparación de datos (antes de la corrida) ----------
function must(res, allowed, label) {
  if (allowed.indexOf(res.status) === -1) {
    fail(`${label} falló. status=${res.status} body=${res.body}`);
  }
  return json(res);
}

function setupPost(path, body, token, label, allowed) {
  const res = http.post(
    `${BASE_URL}${path}`,
    body === null || body === undefined ? null : JSON.stringify(body),
    { headers: hdr(token), tags: { op: 'setup', phase: 'setup' } },
  );
  return must(res, allowed || [201], label);
}

function login(email, password) {
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email, password }),
    { headers: hdr(''), tags: { op: 'setup', phase: 'setup' } },
  );
  return res.status === 200 ? json(res) : null;
}

function ensureAdmin() {
  let logged = login(ADMIN_EMAIL, ADMIN_PASSWORD);
  if (logged && logged.token) return logged.token;

  const boot = http.post(
    `${BASE_URL}/auth/bootstrap-admin`,
    JSON.stringify({ email: ADMIN_EMAIL, password: ADMIN_PASSWORD, full_name: 'Admin k6' }),
    { headers: hdr(''), tags: { op: 'setup', phase: 'setup' } },
  );
  if ([201, 409].indexOf(boot.status) === -1) {
    fail(`Bootstrap admin falló: status=${boot.status} body=${boot.body}`);
  }
  logged = login(ADMIN_EMAIL, ADMIN_PASSWORD);
  if (!logged || !logged.token) {
    fail('No fue posible iniciar sesión como admin; usa -e ADMIN_EMAIL y -e ADMIN_PASSWORD.');
  }
  return logged.token;
}

function createTeacher(adminToken, runId) {
  const email = `teacher.e1.${runId}@mooc.test`;
  setupPost('/admin/users/teachers',
    { email, password: TEACHER_PASSWORD, full_name: 'Profesor E1' },
    adminToken, 'Crear profesor');
  const logged = login(email, TEACHER_PASSWORD);
  if (!logged || !logged.token) fail('Login del profesor falló en setup.');
  return logged.token;
}

function buildCourse(teacherToken, runId, category, n) {
  const created = setupPost('/courses', {
    title: `Curso E1 ${n} ${runId}`,
    summary: `Curso sintético del Escenario 1 (${n})`,
    category,
  }, teacherToken, `Crear curso ${n}`);
  const courseId = pick(created.course, 'id', 'ID');
  const versionId = pick(created.version, 'id', 'ID');

  const moduleId = pick(setupPost(`/versions/${versionId}/modules`,
    { title: 'Módulo 1', position: 1 }, teacherToken, 'Crear módulo'), 'id', 'ID');
  const unitId = pick(setupPost(`/modules/${moduleId}/units`,
    { title: 'Unidad 1', position: 1 }, teacherToken, 'Crear unidad'), 'id', 'ID');

  const textResources = [];
  for (let i = 1; i <= TEXT_RESOURCES; i += 1) {
    const rid = pick(setupPost(`/units/${unitId}/resources`, {
      type: 'rich_text', title: `Lectura ${i}`, position: i,
      visible: true, required: true, downloadable: false,
    }, teacherToken, `Crear lectura ${i}`), 'id', 'ID');

    const put = http.put(
      `${BASE_URL}/resources/${rid}/content/draft`,
      JSON.stringify({ markdown: `# Lectura ${i}\n\nContenido sintético para carga.` }),
      { headers: hdr(teacherToken), tags: { op: 'setup', phase: 'setup' } },
    );
    must(put, [200], `Guardar borrador de lectura ${i}`);
    setupPost(`/resources/${rid}/content/publish`, null, teacherToken,
      `Publicar contenido de lectura ${i}`, [200]);
    textResources.push(rid);
  }

  const quizResourceId = pick(setupPost(`/units/${unitId}/resources`, {
    type: 'quiz', title: 'Quiz final', position: TEXT_RESOURCES + 1,
    visible: true, required: true, downloadable: false,
  }, teacherToken, 'Crear recurso quiz'), 'id', 'ID');

  const quizId = pick(setupPost(`/resources/${quizResourceId}/quiz`,
    { passing_score: 60, feedback_mode: 'after_submit' }, teacherToken, 'Crear quiz'), 'id', 'ID');
  const questionId = pick(setupPost(`/quizzes/${quizId}/questions`,
    { text: '¿Cuánto es 2 + 2?', position: 1, points: 1 }, teacherToken, 'Crear pregunta'), 'id', 'ID');
  const correctOptionId = pick(setupPost(`/questions/${questionId}/options`,
    { text: '4', position: 1, is_correct: true }, teacherToken, 'Crear opción correcta'), 'id', 'ID');
  setupPost(`/questions/${questionId}/options`,
    { text: '5', position: 2, is_correct: false }, teacherToken, 'Crear opción incorrecta');

  setupPost(`/courses/${courseId}/badge`, {
    name: `Insignia E1 ${n}`,
    description: 'Insignia sintética del Escenario 1',
    image_url: 'https://example.com/badge-e1.png',
  }, teacherToken, 'Crear insignia');

  setupPost(`/courses/${courseId}/publish`, null, teacherToken, 'Publicar curso', [200]);

  return { id: courseId, textResources, quizId, questionId, correctOptionId };
}

// Registro, verificación y login de N estudiantes en lotes paralelos.
function createStudents(count, runId) {
  const students = [];
  const CHUNK = 20;
  for (let start = 0; start < count; start += CHUNK) {
    const end = Math.min(count, start + CHUNK);
    const emails = [];
    for (let i = start; i < end; i += 1) emails.push(`student.e1.${runId}.${i}@mooc.test`);

    const reg = http.batch(emails.map((email, k) => ['POST', `${BASE_URL}/auth/register`,
      JSON.stringify({ email, password: STUDENT_PASSWORD, full_name: `Estudiante E1 ${start + k}` }),
      { headers: hdr(''), tags: { op: 'setup', phase: 'setup' } }]));
    const tokens = reg.map((r, k) => {
      const body = must(r, [201], `Registrar ${emails[k]}`);
      return pick(body, 'verification_token_dev', 'verificationTokenDev');
    });

    const ver = http.batch(tokens.map((t) => ['POST', `${BASE_URL}/auth/verify-email`,
      JSON.stringify({ token: t }),
      { headers: hdr(''), tags: { op: 'setup', phase: 'setup' } }]));
    ver.forEach((r, k) => must(r, [200], `Verificar ${emails[k]}`));

    const log = http.batch(emails.map((email) => ['POST', `${BASE_URL}/auth/login`,
      JSON.stringify({ email, password: STUDENT_PASSWORD }),
      { headers: hdr(''), tags: { op: 'setup', phase: 'setup' } }]));
    log.forEach((r, k) => {
      const body = must(r, [200], `Login ${emails[k]}`);
      students.push({ email: emails[k], token: pick(body, 'token') });
    });
  }
  return students;
}

export function setup() {
  const runId = `${Date.now()}`;
  const category = `k6e1-${runId}`;

  const adminToken = ensureAdmin();
  const teacherToken = createTeacher(adminToken, runId);

  const courses = [];
  for (let n = 1; n <= COURSES; n += 1) courses.push(buildCourse(teacherToken, runId, category, n));

  // Un estudiante propio por VU (+ reserva): sin conflictos artificiales.
  const students = createStudents(effectiveVUs + 2, runId);

  return { runId, category, courses, students, startedAt: Date.now() };
}
