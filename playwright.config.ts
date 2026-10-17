import {env} from 'node:process';
import {defineConfig, devices} from '@playwright/test';

const timeoutFactor = Number(env.GITEA_TEST_E2E_TIMEOUT_FACTOR) || 1;
const timeout = 5000 * timeoutFactor;

export default defineConfig({
  workers: '50%',
  fullyParallel: true,
  testDir: './tests/e2e/',
  outputDir: './tests/e2e-output/',
  testMatch: /.*\.test\.ts/,
  forbidOnly: Boolean(env.CI),
  reporter: 'list',
  timeout: 2 * timeout,
  expect: {
    timeout,
  },
  use: {
    baseURL: env.GITEA_TEST_E2E_URL?.replace?.(/\/$/, ''),
    locale: 'en-US',
    actionTimeout: timeout,
    navigationTimeout: 2 * timeout,
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        permissions: ['clipboard-read', 'clipboard-write'],
      },
    },
    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
      },
    },
  ],
});
