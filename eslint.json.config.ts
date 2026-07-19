import json from '@eslint/json';
import {defineConfig, globalIgnores} from 'eslint/config';

export default defineConfig([
  globalIgnores([
    'public',
    'custom/public',
    'data',
    '.venv',
  ]),
  {
    files: ['**/*.json'],
    plugins: {json},
    language: 'json/json',
    extends: ['json/recommended'],
  },
  {
    files: ['**/*.json5'],
    plugins: {json},
    language: 'json/json5',
    extends: ['json/recommended'],
  },
  {
    files: [
      'tsconfig.json',
      '.devcontainer/*.json',
      '.vscode/*.json',
      'contrib/development/vscode/*.json',
    ],
    plugins: {json},
    language: 'json/jsonc',
    languageOptions: {
      allowTrailingCommas: true,
    },
    extends: ['json/recommended'],
  },
]);
