import {env} from 'node:process';
import {defineConfig, devices} from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e/',
  outputDir: './tests/e2e-output/',
  testMatch: /.*\.test\.ts/,
  forbidOnly: Boolean(env.CI),
  reporter: 'list',
  timeout: env.CI ? 12000 : 6000,
  expect: {
    timeout: env.CI ? 6000 : 3000,
  },
  use: {
    baseURL: env.GITEA_TEST_E2E_URL?.replace?.(/\/$/g, ''),
    locale: 'en-US',
    actionTimeout: env.CI ? 6000 : 3000,
    navigationTimeout: env.CI ? 12000 : 6000,
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        permissions: ['clipboard-read', 'clipboard-write'],
      },
    },
    ...env.CI ? [{
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
      },
    }] : [],
  ],
});
