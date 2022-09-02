#!/usr/bin/env node
import fastGlob from 'fast-glob';
import {fileURLToPath} from 'url';
import {readFileSync, writeFileSync} from 'fs';
import wrapAnsi from 'wrap-ansi';
import {join, dirname} from 'path';

const base = process.argv[2];
const out = process.argv[3];

const glob = (pattern) => fastGlob.sync(pattern, {
  cwd: fileURLToPath(new URL(`../${base}`, import.meta.url)),
});

function exit(err) {
  if (err) console.error(err);
  process.exit(err ? 1 : 0);
}

async function main() {
  const line = '-'.repeat(80);
  const str = glob(['**/*']).sort().filter((path) => {
    return /\/license/i.test(path);
  }).map((path) => {
    const body = wrapAnsi(readFileSync(join(base, path), 'utf8') || '', 80);
    const name = dirname(path);
    return `${line}\n${name}\n${line}\n${body}`;
  }).join('\n');
  writeFileSync(out, str);
}

main().then(exit).catch(exit);
