import {defineConfig} from 'vitest/config';
import {playwright} from '@vitest/browser-playwright';
import {sharedPlugins, vueDefines} from './tools/shared.ts';
import {env} from 'node:process';

export default defineConfig({
  test: {
    testTimeout: 20000,
    allowOnly: true,
    passWithNoTests: true,
    globals: true,
    watch: false,
    projects: [
      {
        extends: true,
        test: {
          name: 'browser',
          include: ['web_src/**/*.test.ts'],
          setupFiles: ['web_src/js/vitest.setup.ts'],
          browser: {
            enabled: true,
            provider: playwright(),
            headless: true,
            screenshotFailures: false,
            instances: ((env.PLAYWRIGHT_BROWSERS || 'chromium firefox')
              .split(' ') as Array<'chromium' | 'firefox' | 'webkit'>)
              .map((browser) => ({browser, name: browser})),
          },
        },
      },
      {
        extends: true,
        test: {
          name: 'node',
          include: ['tools/**/*.test.ts'],
          environment: 'node',
        },
      },
    ],
  },
  publicDir: false,
  define: vueDefines,
  plugins: sharedPlugins(),
});
