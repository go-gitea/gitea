import {defineConfig} from 'vitest/dist/config.js';
import {readFile} from 'node:fs/promises';
import {dataToEsm} from '@rollup/pluginutils';
import {extname} from 'node:path';
import vue from '@vitejs/plugin-vue';

function stringPlugin() {
  return {
    name: 'string-plugin',
    enforce: 'pre',
    async load(id) {
      const path = id.split('?')[0];
      if (extname(path) !== '.svg') return null;
      return dataToEsm(await readFile(path, 'utf8'));
    }
  };
}

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
