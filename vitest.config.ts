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
  // vitest would otherwise resolve the node build, which ships the runtime compiler
  resolve: {alias: {vue: 'vue/dist/vue.runtime.esm-bundler.js'}},
  define: vueDefines,
  plugins: sharedPlugins(),
});
