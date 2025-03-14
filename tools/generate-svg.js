#!/usr/bin/env node
import fastGlob from 'fast-glob';
import {optimize} from 'svgo';
import {parse} from 'node:path';
import {readFile, writeFile, mkdir} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {exit} from 'node:process';
import * as fs from 'node:fs';

const glob = (pattern) => fastGlob.sync(pattern, {
  cwd: fileURLToPath(new URL('..', import.meta.url)),
  absolute: true,
});

async function processAssetsSvgFile(file, {prefix, fullName} = {}) {
  let name = fullName;
  if (!name) {
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
          ],
        },
      },
    ],
  });
  await writeFile(fileURLToPath(new URL(`../public/assets/img/svg/${name}.svg`, import.meta.url)), data);
}

function processAssetsSvgFiles(pattern, opts) {
  return glob(pattern).map((file) => processAssetsSvgFile(file, opts));
}

async function processMaterialFileIcons() {
  const files = glob('node_modules/material-icon-theme/icons/*.svg');
  const svgSymbols = {};
  for (const file of files) {
    // remove all unnecessary attributes, only keep "viewBox"
    const {data} = optimize(await readFile(file, 'utf8'), {
      plugins: [
        {name: 'preset-default'},
        {name: 'removeDimensions'},
        {name: 'removeXMLNS'},
        {name: 'removeAttrs', params: {attrs: 'xml:space', elemSeparator: ','}},
      ],
    });
    const svgName = parse(file).name;
    // intentionally use single quote here to avoid escaping
    svgSymbols[svgName] = data.replace(/"/g, `'`);
  }
  fs.writeFileSync(fileURLToPath(new URL(`../options/fileicon/material-icon-svgs.json`, import.meta.url)), JSON.stringify(svgSymbols, null, 2));

  const vscodeExtensionsJson = await readFile(fileURLToPath(new URL(`generate-svg-vscode-extensions.json`, import.meta.url)));
  const vscodeExtensions = JSON.parse(vscodeExtensionsJson);
  const iconRulesJson = await readFile(fileURLToPath(new URL(`../node_modules/material-icon-theme/dist/material-icons.json`, import.meta.url)));
  const iconRules = JSON.parse(iconRulesJson);
  // The rules are from VSCode material-icon-theme, we need to adjust them to our needs
  // 1. We only use lowercase filenames to match (it should be good enough for most cases and more efficient)
  // 2. We do not have a "Language ID" system:
  //    * https://code.visualstudio.com/docs/languages/identifiers#_known-language-identifiers
  //    * https://github.com/microsoft/vscode/tree/1.98.0/extensions
  delete iconRules.iconDefinitions;
  for (const [k, v] of Object.entries(iconRules.fileNames)) iconRules.fileNames[k.toLowerCase()] = v;
  for (const [k, v] of Object.entries(iconRules.folderNames)) iconRules.folderNames[k.toLowerCase()] = v;
  for (const [k, v] of Object.entries(iconRules.fileExtensions)) iconRules.fileExtensions[k.toLowerCase()] = v;
  // Use VSCode's "Language ID" mapping from its extensions
  for (const [_, langIdExtMap] of Object.entries(vscodeExtensions)) {
    for (const [langId, names] of Object.entries(langIdExtMap)) {
      for (const name of names) {
        const nameLower = name.toLowerCase();
        if (nameLower[0] === '.') {
          iconRules.fileExtensions[nameLower.substring(1)] ??= langId;
        } else {
          iconRules.fileNames[nameLower] ??= langId;
        }
      }
    }
  }
  const iconRulesPretty = JSON.stringify(iconRules, null, 2);
  fs.writeFileSync(fileURLToPath(new URL(`../options/fileicon/material-icon-rules.json`, import.meta.url)), iconRulesPretty);
}

async function main() {
  await mkdir(fileURLToPath(new URL('../public/assets/img/svg', import.meta.url)), {recursive: true});
  await Promise.all([
    ...processAssetsSvgFiles('node_modules/@primer/octicons/build/svg/*-16.svg', {prefix: 'octicon'}),
    ...processAssetsSvgFiles('web_src/svg/*.svg'),
    ...processAssetsSvgFiles('public/assets/img/gitea.svg', {fullName: 'gitea-gitea'}),
    processMaterialFileIcons(),
  ]);
}

try {
  await main();
} catch (err) {
  console.error(err);
  exit(1);
}
