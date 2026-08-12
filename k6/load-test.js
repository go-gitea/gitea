import http from 'k6/http';
import { sleep, check } from 'k6';

const BASELINE_URL = __ENV.BASELINE_URL || 'http://localhost:30301';
const STAGING_URL  = __ENV.STAGING_URL  || 'http://localhost:30302';

// Stage durations — injected from CI so pipeline.py and k6 share the same values.
// Defaults here must match K6_STAGE_* in apia-validation.yml.
const WARMUP_S   = parseInt(__ENV.WARMUP_S   || '60',  10);
const RAMPUP_S   = parseInt(__ENV.RAMPUP_S   || '30',  10);
const MEASURE_S  = parseInt(__ENV.MEASURE_S  || '120', 10);
const RAMPDOWN_S = parseInt(__ENV.RAMPDOWN_S || '30',  10);
const VUS        = parseInt(__ENV.VUS        || '25',  10);

const STAGES = [
  { duration: `${WARMUP_S}s`,   target: Math.max(1, Math.floor(VUS * 0.2)) },
  { duration: `${RAMPUP_S}s`,   target: VUS },
  { duration: `${MEASURE_S}s`,  target: VUS },
  { duration: `${RAMPDOWN_S}s`, target: 0   },
];

export const options = {
  scenarios: {
    baseline: {
      executor: 'ramping-vus',
      exec: 'testBaseline',
      stages: STAGES,
      tags: { env: 'baseline' },
    },
    staging: {
      executor: 'ramping-vus',
      exec: 'testStaging',
      stages: STAGES,
      tags: { env: 'staging' },
    },
  },
  thresholds: {
    'http_req_duration{env:staging}':  ['p(95)<2000'],
    'http_req_duration{env:baseline}': ['p(95)<9999999'],
    'http_req_failed{env:staging}':    ['rate<0.05'],
  },
};

// TEST_ENDPOINTS env var overrides defaults — comma-separated list of paths.
// Set it per-repo to test any app. Defaults are Gitea-specific.
const ALL_ENDPOINTS = __ENV.TEST_ENDPOINTS
  ? __ENV.TEST_ENDPOINTS.split(',').map(e => e.trim())
  : ['/', '/explore/repos', '/explore/users', '/user/login',
     '/api/v1/repos/search?limit=10', '/api/v1/topics/search?limit=10'];

function runTest(baseURL) {
  const endpoint = ALL_ENDPOINTS[Math.floor(Math.random() * ALL_ENDPOINTS.length)];
  const res = http.get(`${baseURL}${endpoint}`, { timeout: '10s' });

  check(res, {
    'status 200 or 302': (r) => r.status === 200 || r.status === 302,
  });

  sleep(0.5);
}

export function testBaseline() { runTest(BASELINE_URL); }
export function testStaging()  { runTest(STAGING_URL);  }
