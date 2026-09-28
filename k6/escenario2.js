// Escenario 2: carga, procesamiento y consumo multimedia.
//
// Simula dos poblaciones concurrentes:
//   - "uploaders": profesores que suben un archivo directo a Cloud Storage
//     (carga multipart de una sola parte, suficiente para los tamaños de
//     prueba) y esperan a que el worker lo deje "ready" (hls_manifest_key
//     presente).
//   - "viewers": estudiantes que consumen HLS de contenido YA procesado
//     antes de que arranque la medición (se prepara en setup()), a la
//     cadencia de reproducción declarada (no descargan todos los
//     segmentos de golpe).
//
// El tráfico de control (auth, initiate/complete, /media, /playback) va
// etiquetado module:'media' (contra la API). La transferencia real del
// archivo y la lectura del manifiesto/segmentos van etiquetadas
// module:'storage' (contra Cloud Storage directamente) -- así el reporte
// puede separar ambos tráficos como exige el enunciado.
//
// Requisito de infraestructura para que "viewers" funcione: el prefijo
// resources/*/hls/* del bucket debe ser de lectura publica, porque
// GetPlaybackURL solo prefirma el manifest.m3u8, no cada segmento .ts
// (ver internal/media/http/handler.go). Si el bucket quedo con
// uniform bucket-level access, la forma mas simple es dar
// roles/storage.objectViewer a allUsers sobre todo el bucket (aceptable
// para datos sinteticos de laboratorio que se borran tras la entrega).

import http from 'k6/http';
import { check, fail, sleep } from 'k6';
import { Trend, Counter, Rate } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';

const ADMIN_EMAIL = __ENV.ADMIN_EMAIL || 'admin.k6@mooc.test';
const ADMIN_PASSWORD = __ENV.ADMIN_PASSWORD || 'Admin12345';
const TEACHER_PASSWORD = __ENV.TEACHER_PASSWORD || 'Teacher12345';
const STUDENT_PASSWORD = __ENV.STUDENT_PASSWORD || 'Student12345';

// Perfiles declarados (sección "Escenario 2" del enunciado: al menos 3
// perfiles de duración/tamaño/resolución). Genera estos archivos con
// ffmpeg antes de correr (ver README de esta carpeta / instrucciones).
const PROFILES = [
  { name: 'corto_360p', path: __ENV.VIDEO_SMALL || './media/corto_360p.mp4', mime: 'video/mp4' },
  { name: 'medio_720p', path: __ENV.VIDEO_MEDIUM || './media/medio_720p.mp4', mime: 'video/mp4' },
  { name: 'largo_1080p', path: __ENV.VIDEO_LARGE || './media/largo_1080p.mp4', mime: 'video/mp4' },
];

// open() solo puede llamarse en el scope de inicio (no dentro de una
// función exec de VU), por eso los 3 archivos se leen una sola vez aquí.
const FILES = PROFILES.map((p) => ({ ...p, bytes: open(p.path, 'b') }));

// Cadencia de reproducción declarada: ffmpeg genera segmentos de 6s
// (ver -hls_segment_filename en internal/media/worker/processor.go).
const SEGMENT_SECONDS = Number(__ENV.SEGMENT_SECONDS || 6);

// Nivel de carga: baseline | l1 | l2 | l3 | peak (documenta cuál usaste en
// cada corrida). Ajusta los VUs según lo que tu infraestructura aguante;
// estos son punto de partida, no un valor fijo del enunciado.
const LEVELS = {
  baseline: { uploaders: 1, viewers: 2, duration: '2m' },
  l1: { uploaders: 2, viewers: 5, duration: '3m' },
  l2: { uploaders: 4, viewers: 10, duration: '3m' },
  l3: { uploaders: 6, viewers: 20, duration: '3m' },
  peak: { uploaders: 8, viewers: 30, duration: '3m' }, // repetición cerca del límite
};
const LEVEL = __ENV.LEVEL || 'baseline';
if (!LEVELS[LEVEL]) {
  throw new Error(`LEVEL inválido: ${LEVEL}. Usa baseline, l1, l2, l3 o peak.`);
}
const level = LEVELS[LEVEL];

const READY_POLL_MS = 3000;
const READY_TIMEOUT_MS = Number(__ENV.READY_TIMEOUT_MS || 120000);
const SETUP_READY_TIMEOUT_MS = Number(__ENV.SETUP_READY_TIMEOUT_MS || 240000);

// ---------- métricas custom (mínimo que pide el enunciado para Escenario 2) ----------

const uploadAuthorizeMs = new Trend('upload_authorize_ms');   // API: initiate + url de la parte
const uploadTransferMs = new Trend('upload_transfer_ms');     // storage: PUT directo del archivo
const uploadConfirmMs = new Trend('upload_confirm_ms');       // API: complete
const uploadTimeToReadyMs = new Trend('upload_time_to_ready_ms'); // cola + procesamiento
const uploadReadyTimeouts = new Counter('upload_ready_timeouts');
const uploadFailures = new Rate('upload_functional_failures');

const manifestLatencyMs = new Trend('hls_manifest_ms');       // storage: GET playlist.m3u8
const segmentLatencyMs = new Trend('hls_segment_ms');         // storage: GET segment_NNN.ts
const hlsErrors = new Rate('hls_errors');

// ---------- helpers HTTP (mismo estilo que k6/full-project.js) ----------

function headers(token = '') {
  const h = { 'Content-Type': 'application/json' };
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

function json(res) {
  try { return res.json(); } catch (_) { return {}; }
}

function pick(obj, ...keys) {
  for (const k of keys) {
    if (obj && obj[k] !== undefined && obj[k] !== null) return obj[k];
  }
  return undefined;
}

function must(res, allowed, label) {
  if (!allowed.includes(res.status)) {
    fail(`${label} falló. status=${res.status} body=${res.body}`);
  }
  return json(res);
}

function login(email, password) {
  const res = http.post(`${BASE_URL}/auth/login`, JSON.stringify({ email, password }), {
    headers: headers(), tags: { module: 'media', phase: 'setup' },
  });
  if (res.status !== 200) return null;
  return json(res);
}

function ensureAdmin() {
  let logged = login(ADMIN_EMAIL, ADMIN_PASSWORD);
  if (logged && logged.token) return logged.token;

  const boot = http.post(`${BASE_URL}/auth/bootstrap-admin`, JSON.stringify({
    email: ADMIN_EMAIL, password: ADMIN_PASSWORD, full_name: 'Admin k6 escenario2',
  }), { headers: headers(), tags: { module: 'media', phase: 'setup' } });
  if (![201, 409].includes(boot.status)) {
    fail(`Bootstrap admin falló: status=${boot.status} body=${boot.body}`);
  }
  logged = login(ADMIN_EMAIL, ADMIN_PASSWORD);
  if (!logged || !logged.token) fail('No fue posible autenticar admin en setup.');
  return logged.token;
}

function createTeacher(adminToken, runId) {
  const email = `teacher.e2.${runId}@mooc.test`;
  const res = http.post(`${BASE_URL}/admin/users/teachers`, JSON.stringify({
    email, password: TEACHER_PASSWORD, full_name: 'Profesor Escenario2',
  }), { headers: headers(adminToken), tags: { module: 'media', phase: 'setup' } });
  must(res, [201], 'Crear profesor');
  const logged = login(email, TEACHER_PASSWORD);
  if (!logged || !logged.token) fail('Login profesor falló en setup.');
  return { email, token: logged.token };
}

function createStudent(index, runId, courseId) {
  const email = `student.e2.${runId}.${index}@mooc.test`;
  const reg = http.post(`${BASE_URL}/auth/register`, JSON.stringify({
    email, password: STUDENT_PASSWORD, full_name: `Estudiante Escenario2 ${index}`,
  }), { headers: headers(), tags: { module: 'media', phase: 'setup' } });
  const body = must(reg, [201], `Registrar estudiante ${index}`);
  const verifyToken = pick(body, 'verification_token_dev', 'verificationTokenDev');

  must(http.post(`${BASE_URL}/auth/verify-email`, JSON.stringify({ token: verifyToken }), {
    headers: headers(), tags: { module: 'media', phase: 'setup' },
  }), [200], `Verificar estudiante ${index}`);

  const logged = login(email, STUDENT_PASSWORD);
  if (!logged || !logged.token) fail(`Login estudiante ${index} falló.`);

  must(http.post(`${BASE_URL}/courses/${courseId}/enrollments`, null, {
    headers: headers(logged.token), tags: { module: 'media', phase: 'setup' },
  }), [201], `Inscribir estudiante ${index}`);

  return { email, token: logged.token };
}

function createCourseSkeleton(teacherToken, runId, suffix) {
  const courseRes = must(http.post(`${BASE_URL}/courses`, JSON.stringify({
    title: `Curso Escenario2 ${suffix} ${runId}`,
    summary: 'Curso sintético para Escenario 2 (multimedia)',
    category: 'load-test',
  }), { headers: headers(teacherToken), tags: { module: 'media', phase: 'setup' } }), [201], 'Crear curso');

  const courseId = pick(courseRes.course, 'id', 'ID');
  const versionId = pick(courseRes.version, 'id', 'ID');

  const moduleId = pick(must(http.post(`${BASE_URL}/versions/${versionId}/modules`, JSON.stringify({
    title: 'Módulo Multimedia', position: 1,
  }), { headers: headers(teacherToken), tags: { module: 'media', phase: 'setup' } }), [201], 'Crear módulo'), 'id', 'ID');

  const unitId = pick(must(http.post(`${BASE_URL}/modules/${moduleId}/units`, JSON.stringify({
    title: 'Unidad Multimedia', position: 1,
  }), { headers: headers(teacherToken), tags: { module: 'media', phase: 'setup' } }), [201], 'Crear unidad'), 'id', 'ID');

  // OJO: publicar aquí (antes de crear recursos) falla -- la API exige que
  // el curso tenga al menos un recurso visible para publicarse. El publish
  // se hace en setup(), después de crear los recursos de video.
  return { courseId, unitId };
}

function publishCourse(teacherToken, courseId) {
  must(http.post(`${BASE_URL}/courses/${courseId}/publish`, null, {
    headers: headers(teacherToken), tags: { module: 'media', phase: 'setup' },
  }), [200], 'Publicar curso');
}

function createVideoResource(teacherToken, unitId, title, position) {
  const res = must(http.post(`${BASE_URL}/units/${unitId}/resources`, JSON.stringify({
    type: 'video', title, position, visible: true, required: false, downloadable: false,
  }), { headers: headers(teacherToken), tags: { module: 'media', phase: 'setup' } }), [201], `Crear recurso ${title}`);
  return pick(res, 'id', 'ID');
}

// uploadFile hace el ciclo completo (initiate -> url de la parte -> PUT
// directo a storage -> complete) y devuelve { assetOk, timings }. Se
// reusa tanto en setup() (para dejar contenido "ya disponible" para los
// viewers) como en la función exec de uploaders.
function uploadFile(teacherToken, resourceId, file, tagsExtra) {
  const t0 = Date.now();

  const initRes = http.post(`${BASE_URL}/resources/${resourceId}/uploads/initiate`, JSON.stringify({
    mime_type: file.mime,
    size_bytes: file.bytes.byteLength,
    checksum_sha256: '',
  }), { headers: headers(teacherToken), tags: { module: 'media', ...tagsExtra } });

  const okInit = check(initRes, { 'initiate 201': (r) => r.status === 201 });
  uploadFailures.add(!okInit);
  if (!okInit) return { ok: false };

  const initBody = json(initRes);
  const session = pick(initBody, 'upload_session', 'uploadSession') || {};
  const sessionId = pick(session, 'ID', 'id');

  const urlRes = http.get(`${BASE_URL}/uploads/${sessionId}/parts/1/url`, {
    headers: headers(teacherToken), tags: { module: 'media', ...tagsExtra },
  });
  const okUrl = check(urlRes, { 'part url 200': (r) => r.status === 200 });
  uploadFailures.add(!okUrl);
  if (!okUrl) return { ok: false };

  const partUrl = pick(json(urlRes), 'url');
  const t1 = Date.now();
  uploadAuthorizeMs.add(t1 - t0);

  const putRes = http.put(partUrl, file.bytes, {
    headers: { 'Content-Type': file.mime },
    tags: { module: 'storage', ...tagsExtra },
  });
  const okPut = check(putRes, { 'storage PUT 2xx': (r) => r.status >= 200 && r.status < 300 });
  uploadFailures.add(!okPut);
  if (!okPut) return { ok: false };
  const t2 = Date.now();
  uploadTransferMs.add(t2 - t1);

  const completeRes = http.post(`${BASE_URL}/uploads/${sessionId}/complete`, null, {
    headers: headers(teacherToken), tags: { module: 'media', ...tagsExtra },
  });
  const okComplete = check(completeRes, { 'complete 200': (r) => r.status === 200 });
  uploadFailures.add(!okComplete);
  if (!okComplete) return { ok: false };
  const t3 = Date.now();
  uploadConfirmMs.add(t3 - t2);

  return { ok: true, resourceId, startedReadyWaitAt: t3 };
}

// waitUntilReady hace polling de /resources/:id/media hasta que aparezca
// hls_manifest_key, o hasta el timeout. No cuenta como "carga funcional
// exitosa" solo por status 200 -- valida el campo real, como exige el
// enunciado ("una respuesta HTTP exitosa no demuestra por sí sola...").
function waitUntilReady(teacherToken, resourceId, sinceMs, timeoutMs, tagsExtra) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const res = http.get(`${BASE_URL}/resources/${resourceId}/media`, {
      headers: headers(teacherToken), tags: { module: 'media', ...tagsExtra },
    });
    if (res.status === 200) {
      const asset = json(res);
      if (asset && asset.hls_manifest_key) {
        uploadTimeToReadyMs.add(Date.now() - sinceMs);
        return true;
      }
    }
    sleep(READY_POLL_MS / 1000);
  }
  uploadReadyTimeouts.add(1);
  return false;
}

// ---------- setup: crea curso, profesor, 3 recursos YA procesados (para
// viewers) y estudiantes inscritos ----------

export function setup() {
  const runId = `${Date.now()}`;
  const adminToken = ensureAdmin();
  const teacher = createTeacher(adminToken, runId);

  // Curso SIN publicar: aquí es donde "uploaders" crea un recurso nuevo en
  // cada iteración. Crear recursos es una acción de autoría (no requiere
  // curso publicado); publicarlo bloquearía la versión para más ediciones
  // (423 "esta version ya fue publicada y no se puede editar").
  const { courseId: uploadCourseId, unitId: uploadUnitId } = createCourseSkeleton(teacher.token, runId, 'cargas');

  // Curso que SÍ se publica: aquí viven los 3 recursos "ya disponibles"
  // que consumen los viewers -- para verlos, los estudiantes necesitan
  // estar inscritos, y para inscribirse el curso debe estar publicado.
  const { courseId, unitId } = createCourseSkeleton(teacher.token, runId, 'visualización');

  // Un recurso "ya disponible" por perfil, precargado ANTES de medir.
  const availableResources = [];
  FILES.forEach((file, idx) => {
    const resourceId = createVideoResource(teacher.token, unitId, `Disponible ${file.name}`, idx + 1);
    const up = uploadFile(teacher.token, resourceId, file, { phase: 'setup' });
    if (!up.ok) fail(`No se pudo precargar el recurso de referencia para el perfil ${file.name}`);
    const ready = waitUntilReady(teacher.token, resourceId, up.startedReadyWaitAt, SETUP_READY_TIMEOUT_MS, { phase: 'setup' });
    if (!ready) fail(`El recurso de referencia del perfil ${file.name} no quedó "ready" dentro del timeout de setup`);
    availableResources.push({ resourceId, profile: file.name });
  });

  // Ya hay recursos visibles en el curso -- ahora sí se puede publicar.
  publishCourse(teacher.token, courseId);

  const students = [];
  for (let i = 0; i < level.viewers; i += 1) {
    students.push(createStudent(i, runId, courseId));
  }

  return { runId, teacher, uploadUnitId, availableResources, students };
}

// ---------- exec: uploaders (profesores subiendo, tasa creciente por nivel) ----------

export function uploaders(data) {
  const file = FILES[__ITER % FILES.length];
  const resourceId = createVideoResource(
    data.teacher.token,
    data.uploadUnitId,
    `Carga VU${__VU} iter${__ITER} ${file.name}`,
    100 + __VU * 1000 + __ITER,
  );

  const up = uploadFile(data.teacher.token, resourceId, file, { phase: 'load', profile: file.name });
  if (up.ok) {
    waitUntilReady(data.teacher.token, resourceId, up.startedReadyWaitAt, READY_TIMEOUT_MS, { phase: 'load', profile: file.name });
  }
  sleep(1);
}

// ---------- exec: viewers (consumo HLS de contenido ya disponible, a la
// cadencia declarada -- no descarga todos los segmentos de golpe) ----------

export function viewers(data) {
  const student = data.students[__VU % data.students.length];
  const target = data.availableResources[__VU % data.availableResources.length];

  const playbackRes = http.get(`${BASE_URL}/resources/${target.resourceId}/playback`, {
    headers: headers(student.token), tags: { module: 'media', phase: 'load' },
  });
  const okPlayback = check(playbackRes, { 'playback 200': (r) => r.status === 200 });
  hlsErrors.add(!okPlayback);
  if (!okPlayback) { sleep(1); return; }

  const manifestUrl = pick(json(playbackRes), 'url');

  const t0 = Date.now();
  const manifestRes = http.get(manifestUrl, { tags: { module: 'storage', phase: 'load' } });
  const okManifest = check(manifestRes, { 'manifest 200': (r) => r.status === 200 });
  hlsErrors.add(!okManifest);
  if (!okManifest) { sleep(1); return; }
  manifestLatencyMs.add(Date.now() - t0);

  // segmentos referenciados por el manifiesto (rutas relativas, ver
  // -hls_segment_filename en el worker). Se resuelven contra la MISMA
  // base del manifiesto, sin query string, porque solo el manifiesto
  // viene prefirmado -- los segmentos deben ser de lectura pública.
  const base = manifestUrl.split('?')[0].replace(/[^/]+$/, '');
  const segmentNames = manifestRes.body
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l && !l.startsWith('#'));

  for (const seg of segmentNames) {
    const segT0 = Date.now();
    const segRes = http.get(`${base}${seg}`, { tags: { module: 'storage', phase: 'load' } });
    const okSeg = check(segRes, { 'segment 200': (r) => r.status === 200 });
    hlsErrors.add(!okSeg);
    if (okSeg) segmentLatencyMs.add(Date.now() - segT0);

    // Cadencia de reproducción real: un segmento de 6s no se pide de
    // inmediato tras el anterior. Descargar todo de golpe (sin este
    // sleep) es OTRO patrón de carga y debe reportarse como tal.
    sleep(SEGMENT_SECONDS);
  }
}

// ---------- opciones ----------

export const options = {
  scenarios: {
    uploaders: {
      executor: 'ramping-vus',
      exec: 'uploaders',
      startVUs: 0,
      stages: [
        { duration: '20s', target: level.uploaders },
        { duration: level.duration, target: level.uploaders },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '30s',
    },
    viewers: {
      executor: 'ramping-vus',
      exec: 'viewers',
      startVUs: 0,
      stages: [
        { duration: '20s', target: level.viewers },
        { duration: level.duration, target: level.viewers },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '30s',
    },
  },
  setupTimeout: '5m',
  thresholds: {
    upload_functional_failures: ['rate<0.05'],
    hls_errors: ['rate<0.05'],
  },
};
