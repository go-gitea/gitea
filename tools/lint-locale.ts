#!/usr/bin/env node
import {readFileSync, globSync} from 'node:fs';
import {extname, join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {exit} from 'node:process';

const localeUrl = new URL('../options/locale/locale_en-US.json', import.meta.url);
const localeKeys = new Set(Object.keys(JSON.parse(readFileSync(localeUrl, 'utf8'))));
const unusedKeys = new Set(localeKeys);

// keys travel through variables and struct tags too, so any literal counts as a usage
const stringLiteral = /"((?:[^"\n\\]|\\.)*)"|`([^`\n]*)`|'((?:[^'\n\\]|\\.)*)'/g;
// keys assembled at runtime, like `"repo.signing.wont_sign." + reason` or `printf "admin.dashboard.%s" .Name`
const keyPrefix = /"([a-z][\w.-]*[._-])(?:%[sd]|"\s*\+)/g;
const templateAction = /\{\{(.*?)\}\}/gs;
// calls spelling out their key, `Tr("key")` in Go and `Tr "key"` in templates, the only ones checkable for existence
const trCalls = [
  /\bTr(?:String)?[( ]\s*"((?:[^"\n\\]|\\.)*)"(?!\s*\+)/g, // a trailing `+` means the key is only a prefix
  /\bTrN[( ]\s*(?:\([^)\n]*\)|[^,\s]+)[,\s]\s*"([^"\n\\]*)"[,\s]\s*"([^"\n\\]*)"/g,
];

const rootPath = fileURLToPath(new URL('..', import.meta.url));
const files = globSync(['**/*.go', 'templates/**/*.tmpl'], {
  cwd: rootPath,
  exclude: ['**/node_modules/**', '**/.venv/**', '**/*_test.go'], // test fixtures use made-up keys
});

const prefixes = new Set<string>();
const missingKeys = new Map<string, string>();

function collect(text: string, file: string): void {
  for (const match of text.matchAll(stringLiteral)) {
    const value = match[1] ?? match[2] ?? match[3];
    unusedKeys.delete(value);
    if (match[2]) collect(match[2], file); // Go struct tags hold their key in a `locale:"..."` raw string
  }
  for (const [_match, prefix] of text.matchAll(keyPrefix)) {
    if (prefix.includes('.')) prefixes.add(prefix); // a prefix without a section would match far too much
  }
  for (const regex of trCalls) {
    for (const [_match, ...keys] of text.matchAll(regex)) {
      for (const key of keys) {
        if (!localeKeys.has(key)) missingKeys.set(key, file);
      }
    }
  }
}

for (const file of files) {
  const content = readFileSync(join(rootPath, file), 'utf8');
  if (extname(file) === '.tmpl') {
    // only template actions hold keys, scanning the whole file would let attribute quotes swallow them
    for (const [_match, action] of content.matchAll(templateAction)) collect(action, file);
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
for (const [key, file] of missingKeys) {
  console.info(`locale key "${key}" used in ${file} does not exist`);
}

exit(unusedKeys.size + missingKeys.size ? 1 : 0);
