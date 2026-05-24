// k6-bridge.js - Load test for chatwoot-megaapi-bridge inbound webhook.
//
// Profiles (selected via PROFILE env):
//   smoke  : ramp 1->20 VUs over 1h, gate error_rate<1% p95<300ms p99<500ms
//   24h    : steady 50 VUs for 24h, same gates
//   spike  : 0->200 VUs in 30s, hold 5m, ramp down — soak resilience
//
// Required env:
//   BRIDGE_URL     - base URL of bridge service (e.g. https://bridge.example.com)
//   TENANT_SLUG    - tenant to target
//   WA_TOKEN       - Bearer token for the tenant
// Optional:
//   PROFILE        - smoke | 24h | spike (default: smoke)

import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('bridge_errors');
const responseTime = new Trend('bridge_response_ms');

const BRIDGE_URL = __ENV.BRIDGE_URL || 'http://localhost:8080';
const TENANT_SLUG = __ENV.TENANT_SLUG || 'demo';
const WA_TOKEN = __ENV.WA_TOKEN || 'dev-token';

const profiles = {
  smoke: {
    stages: [
      { duration: '5m', target: 5 },
      { duration: '50m', target: 20 },
      { duration: '5m', target: 0 },
    ],
  },
  '24h': {
    stages: [
      { duration: '10m', target: 50 },
      { duration: '23h40m', target: 50 },
      { duration: '10m', target: 0 },
    ],
  },
  spike: {
    stages: [
      { duration: '30s', target: 200 },
      { duration: '5m', target: 200 },
      { duration: '30s', target: 0 },
    ],
  },
};

const selected = __ENV.PROFILE || 'smoke';
const profile = profiles[selected] || profiles.smoke;

export const options = {
  stages: profile.stages,
  thresholds: {
    http_req_failed: ['rate<0.01'],          // <1% errors
    http_req_duration: ['p(95)<300', 'p(99)<500'],
    bridge_errors: ['rate<0.01'],
  },
  summaryTrendStats: ['min', 'avg', 'med', 'p(95)', 'p(99)', 'max'],
};

function newMessageId() {
  return `loadtest-${Date.now()}-${__VU}-${__ITER}`;
}

export default function () {
  const url = `${BRIDGE_URL}/v1/wa/${TENANT_SLUG}`;
  const payload = JSON.stringify({
    instanceId: 'loadtest',
    messages: [
      {
        key: { id: newMessageId(), remoteJid: '5511999999999@s.whatsapp.net', fromMe: false },
        message: { conversation: `load test ${__ITER}` },
        messageTimestamp: Math.floor(Date.now() / 1000),
      },
    ],
  });
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${WA_TOKEN}`,
    },
    timeout: '10s',
  };
  const res = http.post(url, payload, params);
  responseTime.add(res.timings.duration);
  const ok = check(res, {
    'status is 2xx': (r) => r.status >= 200 && r.status < 300,
  });
  errorRate.add(!ok);
}

export function handleSummary(data) {
  return {
    'loadtest-results/smoke.json': JSON.stringify(data, null, 2),
    stdout: JSON.stringify(
      {
        profile: selected,
        vus_max: data.metrics.vus_max && data.metrics.vus_max.values.max,
        iterations: data.metrics.iterations && data.metrics.iterations.values.count,
        error_rate: data.metrics.http_req_failed && data.metrics.http_req_failed.values.rate,
        p95_ms: data.metrics.http_req_duration && data.metrics.http_req_duration.values['p(95)'],
        p99_ms: data.metrics.http_req_duration && data.metrics.http_req_duration.values['p(99)'],
      },
      null,
      2,
    ),
  };
}
