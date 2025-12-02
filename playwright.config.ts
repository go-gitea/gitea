import {devices} from '@playwright/test';
import {env} from 'node:process';
import type {PlaywrightTestConfig} from '@playwright/test';

const BASE_URL = env.GITEA_URL?.replace?.(/\/$/g, '') || 'http://localhost:3000';

export default {
  testDir: './tests/e2e/',
  testMatch: /.*\.test\.e2e\.ts/, // Match any .test.e2e.ts files

  /* Maximum time one test can run for. */
  timeout: 30 * 1000,

  expect: {

    /**
     * Maximum time expect() should wait for the condition to be met.
     * For example in `await expect(locator).toHaveText();`
     */
    timeout: 2000,
  },

  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: Boolean(env.CI),

  /* Retry on CI only */
  retries: env.CI ? 2 : 0,

  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: env.CI ? 'list' : [['list'], ['html', {outputFolder: 'tests/e2e/reports/', open: 'never'}]],

  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    headless: true,   // set to false to debug

    locale: 'en-US',

    /* Maximum time each action such as `click()` can take. Defaults to 0 (no limit). */
    actionTimeout: 1000,

    /* Maximum time allowed for navigation, such as `page.goto()`. */
    navigationTimeout: 5 * 1000,

    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: BASE_URL,

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',

    screenshot: 'only-on-failure',
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: 'chromium',

      /* Project-specific settings. */
      use: {
        ...devices['Desktop Chrome'],
      },
    },

    // disabled because of https://github.com/go-gitea/gitea/issues/21355
    // {
    //   name: 'firefox',
    //   use: {
    //     ...devices['Desktop Firefox'],
    //   },
    // },

    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
      },
    },

    /* Test against mobile viewports. */
    {
      name: 'Mobile Chrome',
      use: {
        ...devices['Pixel 5'],
      },
    },
    {
      name: 'Mobile Safari',
      use: {
        ...devices['iPhone 12'],
      },
    },
  ],

  /* Folder for test artifacts such as screenshots, videos, traces, etc. */
  outputDir: 'tests/e2e/test-artifacts/',
  /* Folder for test artifacts such as screenshots, videos, traces, etc. */
  snapshotDir: 'tests/e2e/test-snapshots/',
} satisfies PlaywrightTestConfig;
