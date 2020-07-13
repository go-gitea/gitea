#!/usr/bin/env node
'use strict';

const fastGlob = require('fast-glob');
const Svgo = require('svgo');
const {resolve, parse} = require('path');
const {readFile, writeFile, mkdir} = require('fs').promises;

const glob = (pattern) => fastGlob.sync(pattern, {cwd: resolve(__dirname), absolute: true});
const outputDir = resolve(__dirname, '../public/img/svg');

function exit(err) {
  if (err) console.error(err);
  process.exit(err ? 1 : 0);
}

async function processFile(file, {prefix = ''} = {}) {
  const name = `${prefix}${parse(file).name}`;

  const svgo = new Svgo({
    plugins: [
      {removeXMLNS: true},
      {removeDimensions: true},
      {
        addClassesToSVGElement: {
          classNames: [
            'svg',
            name,
          ],
        },
      },
      {
        addAttributesToSVGElement: {
          attributes: [
            {'width': '16'},
            {'height': '16'},
            {'aria-hidden': 'true'},
          ],
        },
      },
    ],
  });

  const {data} = await svgo.optimize(await readFile(file, 'utf8'));
  await writeFile(resolve(outputDir, `${name}.svg`), data);
}

async function main() {
  try {
    await mkdir(outputDir);
  } catch {}

  for (const file of glob('../node_modules/@primer/octicons/build/svg/*.svg')) {
    await processFile(file, {prefix: 'octicon-'});
  }

  for (const file of glob('../web_src/svg/*.svg')) {
    await processFile(file);
  }
}

main().then(exit).catch(exit);

