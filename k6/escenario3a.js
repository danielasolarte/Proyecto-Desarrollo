// Escenario 3A (Entrega 3): ciclo de escalamiento del grupo de la API.
//
// Genera una carga en rampa contra el balanceador para que el autoscaler del
// MIG mooc-api-mig (objetivo 60 % de CPU, 1 a 3 instancias) suba y, al
// terminar, vuelva a bajar. La linea de tiempo (tamano del grupo, instancias,
// salud en el balanceador, reparto y CPU) la registra en paralelo
// deploy/gcp/e3/10-escenario3a.ps1 -Fase Monitor.
//
// Mezcla: consultas publicas del catalogo y, en una fraccion, logins de la
// cuenta de estudiante de prueba (bcrypt en el servidor: es la operacion que
// mas CPU consume, y por eso sube la utilizacion de forma controlable).
//
// Perfil (llegadas por segundo, no VUs fijos, para que la carga no dependa
// de la latencia):
//   base     BASE_RATE req/s durante 2 min
//   subida   hasta PEAK_RATE en 3 min
//   meseta   PEAK_RATE durante HOLD_M min
//   bajada   a 0 en 1 min  -> luego el monitor sigue para ver la reduccion
//
// Variables: BASE_URL (obligatoria, .../api/v1), STUDENT_EMAIL,
// STUDENT_PASSWORD, BASE_RATE (5), PEAK_RATE (30), HOLD_M (12),
// LOGIN_RATIO (0.1).
import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const BASE_URL = (__ENV.BASE_URL || '').replace(/\/$/, '');
const BASE_RATE = Number(__ENV.BASE_RATE || 5);
const PEAK_RATE = Number(__ENV.PEAK_RATE || 30);
const HOLD_M = Number(__ENV.HOLD_M || 12);
const LOGIN_RATIO = Number(__ENV.LOGIN_RATIO || 0.1);
const EMAIL = __ENV.STUDENT_EMAIL || '';
const PASSWORD = __ENV.STUDENT_PASSWORD || '';

const sinInstancia = new Counter('respuestas_sin_x_instance_id');

export const options = {
  scenarios: {
    carga: {
      executor: 'ramping-arrival-rate',
      startRate: BASE_RATE,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 400,
      stages: [
        { target: BASE_RATE, duration: '2m' },
        { target: PEAK_RATE, duration: '3m' },
        { target: PEAK_RATE, duration: `${HOLD_M}m` },
        { target: 0, duration: '1m' },
      ],
    },
  },
  thresholds: {
    // No es un criterio de aceptacion estricto: deja constancia de si el
    // grupo absorbio la carga mientras escalaba.
    http_req_failed: ['rate<0.05'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export function setup() {
  if (!BASE_URL) throw new Error('Falta BASE_URL (https://<dominio>/api/v1).');
  const r = http.get(`${BASE_URL}/catalog`);
  if (r.status !== 200) throw new Error(`El catalogo respondio ${r.status}; revisa que la API este encendida.`);
  if (!EMAIL || !PASSWORD) console.warn('Sin STUDENT_EMAIL/STUDENT_PASSWORD: solo se consulta el catalogo (menos CPU).');
}

export default function () {
  let res;
  if (EMAIL && PASSWORD && Math.random() < LOGIN_RATIO) {
    res = http.post(
      `${BASE_URL}/auth/login`,
      JSON.stringify({ email: EMAIL, password: PASSWORD }),
      // Jar desechable: cada login es independiente y no arrastra cookies.
      { headers: { 'Content-Type': 'application/json' }, tags: { op: 'login' }, jar: new http.CookieJar() },
    );
    check(res, { 'login responde 200': (r) => r.status === 200 });
  } else {
    res = http.get(`${BASE_URL}/catalog`, { tags: { op: 'catalog' } });
    check(res, { 'catalog responde 200': (r) => r.status === 200 });
  }
  if (!res.headers['X-Instance-Id']) sinInstancia.add(1);
}
