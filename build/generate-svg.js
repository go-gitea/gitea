#!/usr/bin/env node
import fastGlob from 'fast-glob';
import {optimize} from 'svgo';
import {parse} from 'node:path';
import {readFile, writeFile, mkdir} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';

const glob = (pattern) => fastGlob.sync(pattern, {
  cwd: fileURLToPath(new URL('..', import.meta.url)),
  absolute: true,
});

function exit(err) {
  if (err) console.error(err);
  process.exit(err ? 1 : 0);
}

async function processFile(file, {prefix, fullName} = {}) {
  let name;
  if (fullName) {
    name = fullName;
  } else {
    name = parse(file).name;
    if (prefix) name = `${prefix}-${name}`;
    if (prefix === 'octicon') name = name.replace(/-[0-9]+$/, ''); // chop of '-16' on octicons
  }

  // Set the `xmlns` attribute so that the files are displayable in standalone documents
  // The svg backend module will strip the attribute during startup for inline display
  const {data} = optimize(await readFile(file, 'utf8'), {
    plugins: [
      {name: 'preset-default'},
      {name: 'removeDimensions'},
      {name: 'prefixIds', params: {prefix: () => name}},
      {name: 'addClassesToSVGElement', params: {classNames: ['svg', name]}},
      {
        name: 'addAttributesToSVGElement', params: {
          attributes: [
            {'xmlns': 'http://www.w3.org/2000/svg'},
            {'width': '16'}, {'height': '16'}, {'aria-hidden': 'true'},
          ]
        }
      },
    ],
  });

  await writeFile(fileURLToPath(new URL(`../public/assets/img/svg/${name}.svg`, import.meta.url)), data);
}

function processFiles(pattern, opts) {
  return glob(pattern).map((file) => processFile(file, opts));
}

async function main() {
  try {
    await mkdir(fileURLToPath(new URL('../public/assets/img/svg', import.meta.url)), {recursive: true});
  } catch {}

  await Promise.all([
    ...processFiles('node_modules/@primer/octicons/build/svg/*-16.svg', {prefix: 'octicon'}),
    ...processFiles('web_src/svg/*.svg'),
    ...processFiles('public/assets/img/gitea.svg', {fullName: 'gitea-gitea'}),
  ]);
}

main().then(exit).catch(exit);
