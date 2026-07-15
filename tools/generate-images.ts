#!/usr/bin/env node
import {initWasm, Resvg} from '@resvg/resvg-wasm';
import {optimize} from 'svgo';
import {mkdir, readFile, writeFile} from 'node:fs/promises';
import {argv, exit} from 'node:process';

// mail icons are embedded as inline attachments because mail clients don't render svg.
// colors are fixed mid-tones that work on both light and dark backgrounds.
// keep the octicon choices in sync with web_src/js/modules/action-status-icon.ts.
async function generateMailIcon(octicon: string, name: string, color: string) {
  const url = new URL(`../node_modules/@primer/octicons/build/svg/${octicon}-16.svg`, import.meta.url);
  const svg = (await readFile(url, 'utf8')).replace('<svg ', `<svg fill="${color}" `);
  await generate(svg, `../services/mailer/icons/${name}.png`, {size: 32});
}

async function generate(svg: string, path: string, {size, bg}: {size: number, bg?: boolean}) {
  const outputFile = new URL(path, import.meta.url);

  if (outputFile.href.endsWith('.svg')) {
    const {data} = optimize(svg, {
      plugins: [
        'preset-default',
        'removeDimensions',
        {
          name: 'addAttributesToSVGElement',
          params: {
            attributes: [{width: String(size)}, {height: String(size)}],
          },
        },
      ],
    });
    await writeFile(outputFile, data);
    return;
  }

  const resvgJS = new Resvg(svg, {
    fitTo: {
      mode: 'width',
      value: size,
    },
    ...(bg && {background: 'white'}),
  });
  const renderedImage = resvgJS.render();
  const pngBytes = renderedImage.asPng();
  await writeFile(outputFile, Buffer.from(pngBytes));
}

async function main() {
  const gitea = argv.slice(2).includes('gitea');
  const logoSvg = await readFile(new URL('../assets/logo.svg', import.meta.url), 'utf8');
  const faviconSvg = await readFile(new URL('../assets/favicon.svg', import.meta.url), 'utf8');
  await initWasm(await readFile(new URL(import.meta.resolve('@resvg/resvg-wasm/index_bg.wasm'))));
  await mkdir(new URL('../services/mailer/icons/', import.meta.url), {recursive: true});

  await Promise.all([
    generateMailIcon('check-circle-fill', 'status-success', '#2da44e'),
    generateMailIcon('x-circle-fill', 'status-failure', '#e5534b'),
    generateMailIcon('stop', 'status-cancelled', '#808080'),
    generateMailIcon('skip', 'status-skipped', '#808080'),
    generate(logoSvg, '../public/assets/img/logo.svg', {size: 32}),
    generate(logoSvg, '../public/assets/img/logo.png', {size: 512}),
    generate(faviconSvg, '../public/assets/img/favicon.svg', {size: 32}),
    generate(faviconSvg, '../public/assets/img/favicon.png', {size: 180}),
    generate(logoSvg, '../public/assets/img/avatar_default.png', {size: 200}),
    generate(logoSvg, '../public/assets/img/apple-touch-icon.png', {size: 180, bg: true}),
    gitea && generate(logoSvg, '../public/assets/img/gitea.svg', {size: 32}),
  ]);
}

try {
  await main();
} catch (err) {
  console.error(err);
  exit(1);
}
