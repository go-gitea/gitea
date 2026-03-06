#!/usr/bin/env node
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {readFileSync, readdirSync} from 'node:fs';
import {join} from 'node:path';
import {argv, cwd, exit, platform} from 'node:process';

const pwd = cwd();
const args = argv.slice(2);

let hash = '';
try {
  const h = createHash('sha1');
  const cIndex = args.indexOf('-c');
  if (cIndex !== -1 && args[cIndex + 1]) {
    h.update(readFileSync(join(pwd, args[cIndex + 1])));
  } else {
    const configRe = /^eslint\.config\.\w+$/;
    for (const file of readdirSync(pwd).sort()) {
      if (configRe.test(file)) {
        h.update(readFileSync(join(pwd, file)));
      }
    }
  }
  try {
    h.update(readFileSync(join(pwd, 'pnpm-lock.yaml')));
  } catch {}
  hash = h.digest('hex').slice(0, 12);
} catch {}

try {
  execFileSync('pnpm', ['exec', 'eslint',
    '--cache',
    '--cache-location', `node_modules/.cache/eslint/${hash}/`,
    '--cache-strategy', 'content',
    ...args,
  ], {
    stdio: 'inherit',
    shell: platform === 'win32',
  });
} catch (err: any) {
  exit(err?.status ?? 1);
}
