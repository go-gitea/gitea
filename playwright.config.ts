import {env} from 'node:process';
import {defineConfig, devices} from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e/',
  outputDir: './tests/e2e-output/',
  testMatch: /.*\.test\.ts/,
  forbidOnly: Boolean(env.CI),
  reporter: 'list',
  timeout: env.CI ? 30000 : 10000,
  expect: {
    timeout: env.CI ? 15000 : 5000,
  },
  use: {
    baseURL: env.E2E_URL?.replace?.(/\/$/g, '') || 'http://localhost:3000',
    locale: 'en-US',
    trace: 'off',
    screenshot: 'off',
    video: 'off',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        permissions: ['clipboard-read', 'clipboard-write'],
      },
    },
  ],
});
