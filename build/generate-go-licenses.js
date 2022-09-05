#!/usr/bin/env node
import fastGlob from 'fast-glob';
import {fileURLToPath} from 'url';
import {readFileSync, writeFileSync} from 'fs';
import wrapAnsi from 'wrap-ansi';
import {join, dirname} from 'path';

const base = process.argv[2];
const out = process.argv[3];

function exit(err) {
  if (err) console.error(err);
  process.exit(err ? 1 : 0);
}

async function main() {
  const data = fastGlob.sync('**/*', {
    cwd: fileURLToPath(new URL(`../${base}`, import.meta.url)),
  }).filter((path) => {
    return /\/((UN)?LICEN(S|C)E|COPYING|NOTICE)/i.test(path);
  }).sort().map((path) => {
    return {
      name: dirname(path),
      body: wrapAnsi(readFileSync(join(base, path), 'utf8') || '', 80)
    };
  });
  writeFileSync(out, JSON.stringify(data, null, 2));
}

main().then(exit).catch(exit);
