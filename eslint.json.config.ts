import {defineConfig, globalIgnores} from 'eslint/config';
import json from '@eslint/json';

export default defineConfig([
  globalIgnores([
    '**/.venv',
    '**/node_modules',
    '**/public',
  ]),
  {
    plugins: {
      json,
    },
  },
  {
    files: ['**/*.json'],
    language: 'json/json',
    rules: {
      'json/no-duplicate-keys': [2],
    },
  },
  {
    files: [
      'tsconfig.json',
      '.devcontainer/*.json',
      '.vscode/*.json',
      'contrib/ide/vscode/*.json',
    ],
    language: 'json/jsonc',
    languageOptions: {
      allowTrailingCommas: true,
    },
    rules: {
      'json/no-duplicate-keys': [2],
    },
  },
]);
