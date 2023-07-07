import {defineConfig} from 'vitest/dist/config.js';
import vue from '@vitejs/plugin-vue';
import {stringPlugin} from 'vite-string-plugin';

export default defineConfig({
  test: {
    include: ['web_src/**/*.test.js'],
    setupFiles: ['./web_src/js/test/setup.js'],
    environment: 'jsdom',
    testTimeout: 20000,
    open: false,
    allowOnly: true,
    passWithNoTests: true,
    watch: false,
    outputDiffLines: Infinity,
  },
  plugins: [
    stringPlugin(),
    vue(),
  ],
});
