// k6 load scenario for POST /v1/wa/{slug}.
//
// Why: validates that the hot ACK path (lookup tenant -> auth -> body read ->
// json parse -> idempotency insert -> enqueue) sustains 1000 rps for 5 min
// with p99 < 10ms client-side and zero message loss vs k6 iteration count.
//
// Idempotency-aware: each iteration generates a globally-unique key.id so the
// `messages` UNIQUE (tenant_id, direction, external_id) never matches and
// every successful request becomes a real DB row. The wrapper compares
// inserted rows against k6 iterations to compute loss.

import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8090';
const SLUG = __ENV.SLUG || 'loadtest';
const BEARER = __ENV.BEARER;
const RATE = parseInt(__ENV.RATE || '1000', 10);
const DURATION = __ENV.DURATION || '5m';
const PRE_VUS = parseInt(__ENV.PRE_VUS || '200', 10);
const MAX_VUS = parseInt(__ENV.MAX_VUS || '500', 10);

if (!BEARER) {
  throw new Error('BEARER env var required (tenant webhook bearer token)');
}

// RUN_TAG keeps DB row attribution unambiguous if the same tenant gets
// multiple back-to-back runs (timestamp -> seconds since k6 process start).
const RUN_TAG = __ENV.RUN_TAG || `run-${Date.now()}`;

export const options = {
  discardResponseBodies: true,
  summaryTrendStats: ['min', 'avg', 'med', 'p(50)', 'p(90)', 'p(95)', 'p(99)', 'p(99.9)', 'max'],
  scenarios: {
    wa_webhook: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: PRE_VUS,
      maxVUs: MAX_VUS,
      gracefulStop: '30s',
    },
  },
  thresholds: {
    'http_req_duration{expected_response:true}': ['p(99)<10', 'p(95)<5'],
    http_req_failed: ['rate<0.001'],
    checks: ['rate>0.999'],
  },
};

const URL = `${BASE_URL}/v1/wa/${SLUG}`;
const PARAMS = {
  headers: {
    Authorization: `Bearer ${BEARER}`,
    'Content-Type': 'application/json',
  },
  // The ACK path is in-process; long socket timeouts would hide queue-full
  // backpressure as p99 spikes. Cap at 5s so failures surface fast.
  timeout: '5s',
};

export default function () {
  // Composite key: RUN_TAG-vu-iter is unique per (run, vu, iteration) and
  // sidesteps any collision with prior runs against the same tenant.
  const id = `${RUN_TAG}-${__VU}-${__ITER}`;
  const payload = JSON.stringify({
    key: {
      id: id,
      remoteJid: `5511999${(__VU % 1000).toString().padStart(3, '0')}@s.whatsapp.net`,
      fromMe: false,
    },
    pushName: `lt-${__VU}`,
    message: { conversation: `loadtest ${id}` },
  });

  const res = http.post(URL, payload, PARAMS);
  check(res, {
    'status 200': (r) => r.status === 200,
  });
}

export function handleSummary(data) {
  // Persist a machine-readable summary next to the script so the wrapper can
  // diff DB row count vs k6 iteration count without re-parsing stdout.
  return {
    stdout: textSummary(data),
    '/scripts/summary.json': JSON.stringify(data, null, 2),
  };
}

// Minimal text summary; k6's built-in textSummary is in a separate module
// that the bundled image may not always have, so we render the essentials.
function textSummary(data) {
  const m = data.metrics;
  const fmt = (x) => (x === undefined ? 'n/a' : Number(x).toFixed(2));
  const dur = m.http_req_duration ? m.http_req_duration.values : {};
  const fail = m.http_req_failed ? m.http_req_failed.values : {};
  const iters = m.iterations ? m.iterations.values : {};
  const checks = m.checks ? m.checks.values : {};
  const lines = [
    '',
    '=== k6 summary (wa-webhook) ===',
    `iterations:            ${iters.count || 0}`,
    `iterations/s (avg):    ${fmt(iters.rate)}`,
    `http_req_duration p50: ${fmt(dur['p(50)'])} ms`,
    `http_req_duration p95: ${fmt(dur['p(95)'])} ms`,
    `http_req_duration p99: ${fmt(dur['p(99)'])} ms`,
    `http_req_duration max: ${fmt(dur.max)} ms`,
    `http_req_failed rate:  ${fmt(fail.rate)}`,
    `checks pass rate:      ${fmt(checks.rate)}`,
    `RUN_TAG:               ${__ENV.RUN_TAG || ''}`,
    '',
  ];
  return lines.join('\n');
}
