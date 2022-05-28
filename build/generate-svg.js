#!/usr/bin/env node
import fastGlob from 'fast-glob';
import {optimize} from 'svgo';
import {parse} from 'path';
import {readFile, writeFile, mkdir} from 'fs/promises';
import {fileURLToPath} from 'url';

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

  const {data} = optimize(await readFile(file, 'utf8'), {
    plugins: [
      {name: 'preset-default'},
      {name: 'removeXMLNS'},
      {name: 'removeDimensions'},
      {name: 'prefixIds', params: {prefix: () => name}},
      {name: 'addClassesToSVGElement', params: {classNames: ['svg', name]}},
      {name: 'addAttributesToSVGElement', params: {attributes: [{'width': '16'}, {'height': '16'}, {'aria-hidden': 'true'}]}},
    ],
  });

  await writeFile(fileURLToPath(new URL(`../public/img/svg/${name}.svg`, import.meta.url)), data);
}

function processFiles(pattern, opts) {
  return glob(pattern).map((file) => processFile(file, opts));
}

async function main() {
  try {
    await mkdir(fileURLToPath(new URL('../public/img/svg', import.meta.url)), {recursive: true});
  } catch {}

  await Promise.all([
    ...processFiles('node_modules/@primer/octicons/build/svg/*-16.svg', {prefix: 'octicon'}),
    ...processFiles('web_src/svg/*.svg'),
    ...processFiles('public/img/gitea.svg', {fullName: 'gitea-gitea'}),
  ]);
}

main().then(exit).catch(exit);
