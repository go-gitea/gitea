import http from 'k6/http';
import { sleep, check } from 'k6';

const BASELINE_URL = __ENV.BASELINE_URL || 'http://localhost:30301';
const STAGING_URL  = __ENV.STAGING_URL  || 'http://localhost:30302';

// Identical load profile for both environments.
// 1 min warm-up at low VUs so caches, connection pools, and DB buffers
// reach steady state before the measurement window begins.
const STAGES = [
  { duration: '1m',  target: 5  }, // warm-up
  { duration: '30s', target: 20 }, // ramp up
  { duration: '2m',  target: 20 }, // measurement window
  { duration: '30s', target: 0  }, // ramp down
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
    'http_req_duration{env:staging}': ['p(95)<2000'],
    'http_req_failed{env:staging}':   ['rate<0.05'],
  },
};

// HTML pages — these rely heavily on Redis session/template cache
const HTML_ENDPOINTS = [
  '/',
  '/explore/repos',
  '/explore/users',
  '/user/login',
];

// API endpoints — heavily cache-dependent, shows regression most clearly
const API_ENDPOINTS = [
  '/api/v1/repos/search?limit=10',
  '/api/v1/topics/search?limit=10',
];

const ALL_ENDPOINTS = [...HTML_ENDPOINTS, ...API_ENDPOINTS];

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
