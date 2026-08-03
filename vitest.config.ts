import {defineConfig} from 'vitest/config';
import {sharedPlugins, vueDefines} from './tools/shared.ts';

export default defineConfig({
  test: {
    include: [
      'web_src/**/*.test.ts',
      'tools/eslint-rules/**/*.test.ts',
    ],
    setupFiles: ['web_src/js/vitest.setup.ts'],
    environment: 'happy-dom',
    testTimeout: 20000,
    open: false,
    allowOnly: true,
    passWithNoTests: true,
    globals: true,
    watch: false,
    isolate: false,
    sequence: {
      concurrent: true,
    },
  },
  define: vueDefines,
  plugins: sharedPlugins(),
});
