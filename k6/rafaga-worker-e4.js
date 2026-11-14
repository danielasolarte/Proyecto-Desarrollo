// Ráfaga de cargas para medir el worker de la E4 (Cloud Run + Pub/Sub).
//
// Un profesor sube RAFAGA archivos casi al mismo tiempo. Cada carga crea un
// recurso, pide la URL firmada, sube el archivo directo a Cloud Storage,
// confirma la carga (el backend publica el trabajo en Pub/Sub) y espera a
// que el recurso quede listo. Así se mide, para el informe de escalamiento:
//
//   - upload_request_ms     creación de la solicitud de carga (API), objetivo
//                           p95 < 2 s sin contar la transferencia
//   - upload_transfer_ms    PUT directo a Cloud Storage (no es carga de la API)
//   - upload_confirm_ms     confirmación (API: guarda el job y publica)
//   - time_to_ready_ms      desde la confirmación hasta que el derivado existe:
//                           cola de Pub/Sub + escaneo + conversión o HLS.
//                           Objetivos: presentación p95 < 2 min, video corto
//                           p95 < 5 min
//   - ready_timeouts        cargas que no quedaron listas en READY_TIMEOUT_MS
//
// Mientras corre, Monitoring muestra los mensajes pendientes de la
// suscripción subiendo y bajando, y las instancias del worker sin pasar de 3
// (consultas en docs/entrega4/evidencias.md, sección 4).
//
// Variables:
//   BASE_URL          https://<backend o frontend>/api/v1 (obligatoria)
//   TEACHER_EMAIL     cuenta de profesor existente (obligatoria)
//   TEACHER_PASSWORD  su clave (obligatoria; nunca se escribe en el repo)
//   TIPO              presentacion | video (por defecto presentacion)
//   ARCHIVO           ruta del archivo a subir (por defecto según TIPO)
//   RAFAGA            cuántas cargas simultáneas (por defecto 9)
//   READY_TIMEOUT_MS  cuánto esperar a que cada carga quede lista (15 min)
//
// Uso: k6/run-rafaga-worker-e4.ps1 (lo corre con Docker y guarda resultados).

import http from 'k6/http';
import { check, fail, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const BASE_URL = (__ENV.BASE_URL || '').replace(/\/+$/, '');
const TEACHER_EMAIL = __ENV.TEACHER_EMAIL || '';
const TEACHER_PASSWORD = __ENV.TEACHER_PASSWORD || '';
if (!BASE_URL || !TEACHER_EMAIL || !TEACHER_PASSWORD) {
  throw new Error('Faltan BASE_URL, TEACHER_EMAIL o TEACHER_PASSWORD.');
}

const TIPOS = {
  presentacion: {
    resourceType: 'presentation',
    mime: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    archivo: './fixtures/presentacion_prueba.pptx',
    // El worker escanea y convierte a PDF con LibreOffice.
    listo: (asset) => Boolean(asset && asset.converted_pdf_key),
  },
  video: {
    resourceType: 'video',
    mime: 'video/mp4',
    archivo: './media/corto_360p_15s.mp4',
    // El worker escanea y genera HLS con ffmpeg.
    listo: (asset) => Boolean(asset && asset.hls_manifest_key),
  },
};

const TIPO = __ENV.TIPO || 'presentacion';
const tipo = TIPOS[TIPO];
if (!tipo) throw new Error(`TIPO inválido: ${TIPO}. Usa presentacion o video.`);

const ARCHIVO = __ENV.ARCHIVO || tipo.archivo;
const BYTES = open(ARCHIVO, 'b');
const RAFAGA = Number(__ENV.RAFAGA || 9);
const READY_TIMEOUT_MS = Number(__ENV.READY_TIMEOUT_MS || 15 * 60 * 1000);
const POLL_MS = Number(__ENV.POLL_MS || 5000);

const uploadRequestMs = new Trend('upload_request_ms', true);
const uploadTransferMs = new Trend('upload_transfer_ms', true);
const uploadConfirmMs = new Trend('upload_confirm_ms', true);
const timeToReadyMs = new Trend('time_to_ready_ms', true);
const readyTimeouts = new Counter('ready_timeouts');
const uploadFailures = new Rate('upload_failures');

// ---------- utilidades ----------

function headers(token) {
  const h = { 'Content-Type': 'application/json' };
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

function body(res) {
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
    fail(`${label} falló: status=${res.status} body=${String(res.body).slice(0, 300)}`);
  }
  return body(res);
}

// La API también emite cookies de sesión; este script usa Bearer, así que
// el login va en un jar desechable para que las peticiones siguientes no
// exijan el token CSRF.
function login() {
  const res = http.post(`${BASE_URL}/auth/login`,
    JSON.stringify({ email: TEACHER_EMAIL, password: TEACHER_PASSWORD }),
    { headers: headers(), jar: new http.CookieJar(), tags: { fase: 'setup' } });
  const b = must(res, [200], 'Login del profesor');
  if (!b.token) fail('El login no devolvió token.');
  return b.token;
}

// ---------- setup: un curso borrador con una unidad para las cargas ----------

export function setup() {
  const token = login();
  const corrida = new Date().toISOString().replace(/[:.]/g, '-');
  const t = { headers: headers(token), tags: { fase: 'setup' } };

  const curso = must(http.post(`${BASE_URL}/courses`, JSON.stringify({
    title: `Ráfaga worker E4 ${TIPO} ${corrida}`,
    summary: 'Curso sintético para la prueba de escalamiento del worker (E4).',
    category: 'load-test',
  }), t), [201], 'Crear curso');
  const versionId = pick(curso.version, 'id', 'ID');

  const moduloId = pick(must(http.post(`${BASE_URL}/versions/${versionId}/modules`,
    JSON.stringify({ title: 'Módulo ráfaga', position: 1 }), t), [201], 'Crear módulo'), 'id', 'ID');
  const unidadId = pick(must(http.post(`${BASE_URL}/modules/${moduloId}/units`,
    JSON.stringify({ title: 'Unidad ráfaga', position: 1 }), t), [201], 'Crear unidad'), 'id', 'ID');

  console.log(`inicio_rafaga=${new Date().toISOString()} tipo=${TIPO} rafaga=${RAFAGA} archivo=${ARCHIVO} bytes=${BYTES.byteLength}`);
  return { token, unidadId };
}

// ---------- una carga por iteración ----------

export default function (data) {
  const t = { headers: headers(data.token), tags: { fase: 'carga', tipo: TIPO } };
  const n = __VU; // un VU por carga: 1..RAFAGA

  const recurso = http.post(`${BASE_URL}/units/${data.unidadId}/resources`, JSON.stringify({
    type: tipo.resourceType, title: `Carga ${n} ${TIPO}`, position: n,
    visible: true, required: false, downloadable: false,
  }), t);
  if (!check(recurso, { 'recurso 201': (r) => r.status === 201 })) { uploadFailures.add(1); return; }
  const resourceId = pick(body(recurso), 'id', 'ID');

  // 1. Solicitud de carga: initiate + URL firmada de la parte.
  const t0 = Date.now();
  const init = http.post(`${BASE_URL}/resources/${resourceId}/uploads/initiate`, JSON.stringify({
    mime_type: tipo.mime, size_bytes: BYTES.byteLength, checksum_sha256: '',
  }), t);
  if (!check(init, { 'initiate 201': (r) => r.status === 201 })) { uploadFailures.add(1); return; }
  const sesion = pick(body(init), 'upload_session', 'uploadSession') || {};
  const sessionId = pick(sesion, 'ID', 'id');
  const urlRes = http.get(`${BASE_URL}/uploads/${sessionId}/parts/1/url`, t);
  if (!check(urlRes, { 'url de la parte 200': (r) => r.status === 200 })) { uploadFailures.add(1); return; }
  const t1 = Date.now();
  uploadRequestMs.add(t1 - t0);

  // 2. Transferencia directa a Cloud Storage.
  const put = http.put(pick(body(urlRes), 'url'), BYTES, {
    headers: { 'Content-Type': tipo.mime }, tags: { fase: 'storage', tipo: TIPO },
  });
  if (!check(put, { 'PUT a storage 2xx': (r) => r.status >= 200 && r.status < 300 })) { uploadFailures.add(1); return; }
  const t2 = Date.now();
  uploadTransferMs.add(t2 - t1);

  // 3. Confirmación: el backend guarda el job y lo publica en Pub/Sub.
  const done = http.post(`${BASE_URL}/uploads/${sessionId}/complete`, null, t);
  if (!check(done, { 'complete 200': (r) => r.status === 200 })) { uploadFailures.add(1); return; }
  const t3 = Date.now();
  uploadConfirmMs.add(t3 - t2);
  uploadFailures.add(0);

  // 4. Espera hasta que el worker deje el derivado (PDF o HLS).
  const limite = t3 + READY_TIMEOUT_MS;
  while (Date.now() < limite) {
    sleep(POLL_MS / 1000);
    const m = http.get(`${BASE_URL}/resources/${resourceId}/playback`, {
      ...t,
      tags: { fase: 'espera', tipo: TIPO },
    });

    if (m.status === 200) {
      const ms = Date.now() - t3;
      timeToReadyMs.add(ms);
      console.log(`listo carga=${n} resource_id=${resourceId} time_to_ready_ms=${ms}`);
      return;
    }
  }
  readyTimeouts.add(1);
  console.warn(`sin terminar carga=${n} resource_id=${resourceId} tras ${READY_TIMEOUT_MS} ms`);
}

export function teardown() {
  console.log(`fin_rafaga=${new Date().toISOString()}`);
}

// Todas las cargas salen casi a la vez (un VU por carga) para llenar la
// cola más rápido de lo que 3 workers la vacían.
export const options = {
  scenarios: {
    rafaga: {
      executor: 'per-vu-iterations',
      vus: RAFAGA,
      iterations: 1,
      maxDuration: `${Math.ceil(READY_TIMEOUT_MS / 60000) + 5}m`,
    },
  },
  setupTimeout: '2m',
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'max'],
  thresholds: {
    upload_failures: ['rate==0'],
    ready_timeouts: ['count==0'],
    upload_request_ms: ['p(95)<2000'],
  },
};
