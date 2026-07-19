import markdown from '@eslint/markdown';
import unicorn from 'eslint-plugin-unicorn';
import {defineConfig, globalIgnores} from 'eslint/config';

export default defineConfig([
  globalIgnores([
    'public',
    'custom/public',
    'data',
    '.venv',
  ]),
  {
    files: ['**/*.md'],
    plugins: {markdown, unicorn},
    language: 'markdown/gfm',
    rules: {
      'unicorn/no-missing-local-resource': [2],
    },
  },
]);
