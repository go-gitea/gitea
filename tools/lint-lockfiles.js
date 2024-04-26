#!/usr/bin/env node
import {readFileSync} from 'node:fs';
import {exit} from 'node:process';
import {relative} from 'node:path';
import {fileURLToPath} from 'node:url';

const files = [
  '../package-lock.json',
  '../web_src/fomantic/package-lock.json',
];

const rootPath = fileURLToPath(new URL('..', import.meta.url));
let hadErrors = false;

// This checks that all "resolved" URLs in package-lock.json point to the official npm registry.
// If a user is using a npm proxy (private or public), they would write that proxy's URL into
// the file which we do not want because it could cause issues during installation.
for (const file of files.map((file) => fileURLToPath(new URL(file, import.meta.url)))) {
  const data = JSON.parse(readFileSync(file));
  for (const [pkg, {resolved}] of Object.entries(data.packages)) {
    if (resolved && !resolved.startsWith('https://registry.npmjs.org/')) {
      console.info(`${relative(rootPath, file)}: Expected "resolved" on package ${pkg} to start with "https://registry.npmjs.org/"`);
      hadErrors = true;
    }
  }
}

exit(hadErrors ? 1 : 0);
