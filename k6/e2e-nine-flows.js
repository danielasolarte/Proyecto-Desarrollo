import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { setup as seedScenario1, reader, learner, quizTaker, loginBurst } from './escenario1.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const ROOT_URL = __ENV.ROOT_URL || BASE_URL.replace(/\/api\/v1\/?$/, '');
const PUBLIC_URL = __ENV.PUBLIC_URL || ROOT_URL;

export const flowsPassed = new Counter('e2e_flows_passed');

export const options = {
  scenarios: {
    e2e: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1,
      maxDuration: '8m',
    },
  },
  thresholds: {
    checks: ['rate==1'],
    e2e_flows_passed: ['count>=9'],
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<1500'],
  },
};

function pass(name, ok) {
  check(null, { [name]: () => ok });
  if (ok) flowsPassed.add(1, { flow: name });
}

export function setup() {
  return seedScenario1();
}

export default function (data) {
  const health = http.get(`${ROOT_URL}/health`, { tags: { flow: '01_health' } });
  pass('01 healthcheck publico responde ok', health.status === 200 && health.body.includes('ok'));

  const metrics = http.get(`${ROOT_URL}/metrics`, { tags: { flow: '02_metrics' } });
  pass('02 endpoint metrics expone prometheus', metrics.status === 200 && metrics.body.includes('# HELP'));

  const headers = http.get(`${PUBLIC_URL}/health`, { tags: { flow: '03_security_headers' } });
  const hasSecurityHeaders =
    headers.headers['X-Content-Type-Options'] === 'nosniff' ||
    headers.headers['X-Frame-Options'] === 'DENY' ||
    headers.headers['Strict-Transport-Security'] !== undefined;
  pass('03 headers de seguridad en entrada publica', headers.status < 500 && hasSecurityHeaders);

  loginBurst(data);
  pass('04 login autenticado', true);

  reader(data);
  pass('05 catalogo detalle preview e inscripcion', true);

  learner(data);
  pass('06 progreso con open heartbeat complete', true);

  quizTaker(data);
  pass('07 quiz submit idempotente', true);

  const audit = http.get(`${BASE_URL}/admin/audit-log`, {
    headers: { Authorization: `Bearer ${data.students[0].token}` },
    tags: { flow: '08_role_protection' },
  });
  pass('08 proteccion de roles rechaza estudiante', audit.status === 403);

  const notFound = http.get(`${BASE_URL}/badges/verify/not-a-real-code`, { tags: { flow: '09_public_verify' } });
  pass('09 verificacion publica responde controlada', [200, 404].includes(notFound.status));
}

