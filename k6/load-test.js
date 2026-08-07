import http from 'k6/http';
import { sleep, check } from 'k6';

const BASELINE_URL = __ENV.BASELINE_URL || 'http://localhost:30301';
const STAGING_URL  = __ENV.STAGING_URL  || 'http://localhost:30302';

export const options = {
  stages: [
    { duration: '30s', target: 20 },
    { duration: '1m',  target: 20 },
    { duration: '30s', target: 0  },
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],
  },
};

const ENDPOINTS = [
  '/',
  '/explore/repos',
  '/explore/users',
  '/user/login',
];

export default function () {
  const isBaseline = __ITER % 2 === 0;
  const base = isBaseline ? BASELINE_URL : STAGING_URL;

  const endpoint = ENDPOINTS[Math.floor(Math.random() * ENDPOINTS.length)];
  const res = http.get(`${base}${endpoint}`, { timeout: '10s' });

  check(res, {
    'status is 200 or 302': (r) => r.status === 200 || r.status === 302,
  });

  sleep(0.5);
}