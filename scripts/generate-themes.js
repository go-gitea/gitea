const remapCss = require('remap-css');
const fastGlob = require('fast-glob');
const { readFile, writeFile } = require('fs').promises;
const { basename, resolve } = require('path');

const glob = (pattern) => fastGlob(pattern, { cwd: resolve(__dirname, '..') });
const themesSources = resolve(__dirname, '../web_src/less/themes/theme-*.js');
const cssSources = resolve(__dirname, '../public/**/*.css');
const cssSourcesIgnore = [/^theme-/, /^swagger/];

const remapOpts = {
  comments: true,
  stylistic: true,
  indentDeclaration: 4,
};

function filterCssSources(path) {
  return !cssSourcesIgnore.some((re) => re.test(basename(path)));
}

async function makeTheme(jsFile) {
  const mappings = require(`${jsFile}`);
  const outputFile = jsFile.replace(/\.js$/, '.gen.css');

  const publicFiles = (await glob(cssSources)).filter(filterCssSources);
  const cssContents = await Promise.all(publicFiles.map((file) => readFile(file, 'utf8')));
  const generatedCss = await remapCss(cssContents.map((css) => ({ css })), mappings, remapOpts);

  await writeFile(outputFile, generatedCss);
}

function exit(err) {
  if (err) console.error(err);
  process.exit(err ? 1 : 0);
}

async function main() {
  const jsFiles = await glob(themesSources);
  await Promise.all(jsFiles.map(makeTheme));
}

main().then(exit).catch(exit);
