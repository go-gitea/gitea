import {defineConfig, globalIgnores} from 'eslint/config';
import json from '@eslint/json';

export default defineConfig([
  globalIgnores([
    '**/.venv',
    '**/node_modules',
    '**/public',
  ]),
  {
    files: ['**/*.json'],
    plugins: {json},
    language: 'json/json',
    extends: ['json/recommended'],
  },
  {
    files: [
      'tsconfig.json',
      '.devcontainer/*.json',
      '.vscode/*.json',
      'contrib/ide/vscode/*.json',
    ],
    plugins: {json},
    language: 'json/jsonc',
    languageOptions: {
      allowTrailingCommas: true,
    },
    extends: ['json/recommended'],
  },
]);
