#!/usr/bin/env node
import {readdirSync, readFileSync} from 'node:fs';
import {parse, relative} from 'node:path';
import {fileURLToPath} from 'node:url';
import {exit} from 'node:process';
import fastGlob from 'fast-glob';

const knownSvgs = new Set();
for (const file of readdirSync(new URL('../public/assets/img/svg', import.meta.url))) {
  knownSvgs.add(parse(file).name);
}

const rootPath = fileURLToPath(new URL('..', import.meta.url));
let hadErrors = false;

for (const file of fastGlob.sync(fileURLToPath(new URL('../templates/**/*.tmpl', import.meta.url)))) {
  const content = readFileSync(file, 'utf8');
  for (const [_, name] of content.matchAll(/svg ["'`]([^"'`]+)["'`]/g)) {
    if (!knownSvgs.has(name)) {
      console.info(`SVG "${name}" not found, used in ${relative(rootPath, file)}`);
      hadErrors = true;
    }
  }
}

exit(hadErrors ? 1 : 0);
