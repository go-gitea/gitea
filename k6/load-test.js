import http from 'k6/http';
import { sleep, check, group } from 'k6';
import encoding from 'k6/encoding';

const BASELINE_URL = __ENV.BASELINE_URL || 'http://localhost:30301';
const STAGING_URL  = __ENV.STAGING_URL  || 'http://localhost:30302';

const WARMUP_S   = parseInt(__ENV.WARMUP_S   || '60',  10);
const RAMPUP_S   = parseInt(__ENV.RAMPUP_S   || '30',  10);
const MEASURE_S  = parseInt(__ENV.MEASURE_S  || '120', 10);
const RAMPDOWN_S = parseInt(__ENV.RAMPDOWN_S || '30',  10);
const VUS        = parseInt(__ENV.VUS        || '25',  10);

// Test credentials — seeded in setup()
const ADMIN_USER = 'apia-admin';
const ADMIN_PASS = 'Apia2024!';
const TEST_REPO  = 'test-repo';

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

// Seed both environments with a user + repo + issues before the test starts
export function setup() {
  for (const baseURL of [BASELINE_URL, STAGING_URL]) {
    seedEnv(baseURL);
  }
}

function seedEnv(baseURL) {
  const headers = { 'Content-Type': 'application/json' };

  // Auth header — user is pre-created by CI before k6 starts
  const auth = { Authorization: `Basic ${encoding.b64encode(ADMIN_USER + ':' + ADMIN_PASS)}` };
  const h = { ...headers, ...auth };

  // Create a repo
  http.post(`${baseURL}/api/v1/user/repos`, JSON.stringify({
    name: TEST_REPO,
    description: 'APIA load test repository',
    private: false,
    auto_init: true,
    default_branch: 'main',
  }), { headers: h });

  // Create 20 issues so issue listing queries have something to return
  for (let i = 1; i <= 20; i++) {
    http.post(`${baseURL}/api/v1/repos/${ADMIN_USER}/${TEST_REPO}/issues`, JSON.stringify({
      title: `Test issue ${i}`,
      body: `This is test issue number ${i} created by APIA load test seeding.`,
    }), { headers: h });
  }
}


function runTest(baseURL) {
  if (Math.random() < 0.30) {
    anonymousJourney(baseURL);
  } else {
    authenticatedJourney(baseURL);
  }
}

function anonymousJourney(baseURL) {
  group('anonymous', () => {
    // Homepage
    check(http.get(`${baseURL}/`, { timeout: '10s' }), {
      'homepage 200': (r) => r.status === 200,
    });
    sleep(0.5);

    // Explore repos listing
    check(http.get(`${baseURL}/explore/repos`, { timeout: '10s' }), {
      'explore repos 200': (r) => r.status === 200,
    });
    sleep(0.5);

    // API repo search
    check(http.get(`${baseURL}/api/v1/repos/search?limit=10&q=test`, { timeout: '10s' }), {
      'api search 200': (r) => r.status === 200,
    });
    sleep(1);
  });
}

function authenticatedJourney(baseURL) {
  group('authenticated', () => {
    // Login
    const loginPage = http.get(`${baseURL}/user/login`, { timeout: '10s' });
    const csrf = extractCSRF(loginPage.body);

    const loginRes = http.post(`${baseURL}/user/login`, {
      _csrf:     csrf,
      user_name: ADMIN_USER,
      password:  ADMIN_PASS,
    }, {
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      redirects: 5,
      timeout: '10s',
    });

    check(loginRes, {
      'login succeeded': (r) => r.status === 200 && !r.url.includes('user/login'),
    });

    const jar = loginRes.cookies;
    const cookies = Object.entries(jar)
      .map(([k, v]) => `${k}=${v[0].value}`)
      .join('; ');
    const sessionHeaders = { Cookie: cookies };

    sleep(0.5);

    group('dashboard', () => {
      check(http.get(`${baseURL}/`, { headers: sessionHeaders, timeout: '10s' }), {
        'dashboard 200': (r) => r.status === 200,
      });
    });
    sleep(0.5);

    group('repo view', () => {
      check(http.get(`${baseURL}/${ADMIN_USER}/${TEST_REPO}`, {
        headers: sessionHeaders, timeout: '10s',
      }), {
        'repo page 200': (r) => r.status === 200,
      });
    });
    sleep(0.5);

    group('issue list', () => {
      check(http.get(`${baseURL}/${ADMIN_USER}/${TEST_REPO}/issues`, {
        headers: sessionHeaders, timeout: '10s',
      }), {
        'issues 200': (r) => r.status === 200,
      });
    });
    sleep(0.5);

    group('api user repos', () => {
      check(http.get(`${baseURL}/api/v1/user/repos?limit=10`, {
        headers: {
          ...sessionHeaders,
          Authorization: `Basic ${encoding.b64encode(ADMIN_USER + ':' + ADMIN_PASS)}`,
        },
        timeout: '10s',
      }), {
        'api repos 200': (r) => r.status === 200,
      });
    });
    sleep(0.5);

    group('api issues', () => {
      check(http.get(`${baseURL}/api/v1/repos/${ADMIN_USER}/${TEST_REPO}/issues?limit=10&type=issues&state=open`, {
        headers: {
          ...sessionHeaders,
          Authorization: `Basic ${encoding.b64encode(ADMIN_USER + ':' + ADMIN_PASS)}`,
        },
        timeout: '10s',
      }), {
        'api issues 200': (r) => r.status === 200,
      });
    });
    sleep(0.5);

    group('user settings', () => {
      check(http.get(`${baseURL}/user/settings`, {
        headers: sessionHeaders, timeout: '10s',
      }), {
        'settings 200': (r) => r.status === 200,
      });
    });
    sleep(1);
  });
}

function extractCSRF(body) {
  if (!body) return '';
  const s = body.toString();
  // Gitea >= 1.20 puts CSRF in a <meta> tag
  let m = s.match(/<meta\s+name="_csrf"\s+content="([^"]+)"/);
  if (m) return m[1];
  // Fallback: hidden input, value before or after name
  m = s.match(/name="_csrf"[^>]*value="([^"]+)"/);
  if (m) return m[1];
  m = s.match(/value="([^"]+)"[^>]*name="_csrf"/);
  if (m) return m[1];
  return '';
}

export function testBaseline() { runTest(BASELINE_URL); }
export function testStaging()  { runTest(STAGING_URL);  }
