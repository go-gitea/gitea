#!/usr/bin/env node
import fastGlob from 'fast-glob';
import {optimize} from 'svgo';
import {parse} from 'node:path';
import {readFile, writeFile, mkdir, copyFile, rm} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {execSync} from 'node:child_process';

const glob = (pattern) => fastGlob.sync(pattern, {
  cwd: fileURLToPath(new URL('..', import.meta.url)),
  absolute: true,
});

const removeUnwantedSvgs = () => {
  // remove folder from icons as we have a custom, colorful material folder in web_src/svg
  const removeFolder = rm('node_modules/material-icon-theme/icons/folder.svg', {force: true});

  // remove all icons of open folders as we don't use them anywhere
  const removeOpenFolders = glob('node_modules/material-icon-theme/icons/folder*-open.svg').map((file) => rm(file, {force: true}));

  return Promise.all([removeFolder, ...removeOpenFolders]);
};

// inspired by https://github.com/Claudiohbsantos/github-material-icons-extension/blob/ff97e50980/scripts/build-dependencies.js
const generateIconMap = async () => {
  // build icon map
  execSync('npm run generateJson', {cwd: 'node_modules/material-icon-theme'});

  // copy icon map to assets
  const src = fileURLToPath(new URL('../node_modules/material-icon-theme/dist/material-icons.json', import.meta.url));
  const dest = fileURLToPath(new URL('../assets/material-icons.json', import.meta.url));
  await copyFile(src, dest);
};

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

  await writeFile(fileURLToPath(new URL(`../public/img/svg/${name}.svg`, import.meta.url)), data);
}

function processFiles(pattern, opts) {
  return glob(pattern).map((file) => processFile(file, opts));
}

async function main() {
  await removeUnwantedSvgs();
  try {
    await mkdir(fileURLToPath(new URL('../public/img/svg', import.meta.url)), {recursive: true});
  } catch {}

  await Promise.all([
    generateIconMap,
    ...processFiles('node_modules/material-icon-theme/icons/*.svg', {prefix: 'material'}),
    ...processFiles('node_modules/@primer/octicons/build/svg/*-16.svg', {prefix: 'octicon'}),
    ...processFiles('web_src/svg/*.svg'),
    ...processFiles('public/img/gitea.svg', {fullName: 'gitea-gitea'}),
  ]);
}

main().then(exit).catch(exit);
