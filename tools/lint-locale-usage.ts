#!/usr/bin/env node
import {readFileSync, globSync} from 'node:fs';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {exit} from 'node:process';

const localeUrl = new URL('../options/locale/locale_en-US.json', import.meta.url);
const localeKeys = new Set<string>(Object.keys(JSON.parse(readFileSync(localeUrl, 'utf8'))));
const unusedKeys = new Set(localeKeys);

// keys used as-is, in Go/TS string literals or template actions
const stringLiteral = /"((?:[^"\n\\]|\\.)*)"|`([^`\n]*)`|'((?:[^'\n\\]|\\.)*)'/g;
// keys assembled at runtime, like `"repo.signing.wont_sign." + reason` or `printf "admin.dashboard.%s" .Name`
const keyPrefix = /"([a-z][\w.-]*[._-])(?:%[sdv]|"\s*\+)/g;
const templateAction = /\{\{(.*?)\}\}/gs;
// call sites spelling out their key, the only ones that can be checked for existence
const trCalls = [
  /\bTr(?:String|HTMLEscapeArgs)?\(\s*(?:ctx\s*,\s*)?"([^"\n\\]*)"\s*[,)]/g,
  /\bTrN\([^,\n]*,\s*"([^"\n\\]*)"\s*,\s*"([^"\n\\]*)"\s*[,)]/g,
  /\.Tr\s+"([^"\n]*)"/g,
  /\.TrN\s+(?:\([^)\n]*\)|\S+)\s+"([^"\n]*)"\s+"([^"\n]*)"/g,
];

const rootPath = fileURLToPath(new URL('..', import.meta.url));
const files = globSync(['**/*.go', '**/*.ts', '**/*.vue', 'templates/**/*.tmpl'], {
  cwd: rootPath,
  exclude: ['**/node_modules/**', '**/*_test.go', '**/*.test.ts'], // test fixtures use made-up keys
});

const prefixes = new Set<string>();
const missing = new Map<string, string>();

function collect(text: string, file: string): void {
  for (const match of text.matchAll(stringLiteral)) {
    const value = match[1] ?? match[2] ?? match[3];
    unusedKeys.delete(value);
    if (match[2]) collect(match[2], file); // Go struct tags hold their key in a `locale:"..."` raw string
  }
  for (const [, prefix] of text.matchAll(keyPrefix)) {
    if (prefix.includes('.')) prefixes.add(prefix); // a prefix without a section would match far too much
  }
  for (const regex of trCalls) {
    for (const [, ...keys] of text.matchAll(regex)) {
      for (const key of keys) {
        if (!localeKeys.has(key)) missing.set(key, file);
      }
    }
  }
}

for (const file of files) {
  const content = readFileSync(join(rootPath, file), 'utf8');
  if (file.endsWith('.tmpl')) {
    // only template actions hold keys, scanning the whole file would let attribute quotes swallow them
    for (const [, action] of content.matchAll(templateAction)) collect(action, file);
  } else {
    collect(content, file);
  }
}

for (const key of unusedKeys) {
  for (const prefix of prefixes) {
    if (key.startsWith(prefix)) {
      unusedKeys.delete(key);
      break;
    }
  }
}

for (const key of unusedKeys) {
  console.info(`locale key "${key}" is not used anywhere`);
}
for (const [key, file] of missing) {
  console.info(`locale key "${key}" used in ${file} does not exist`);
}

exit(unusedKeys.size + missing.size ? 1 : 0);
