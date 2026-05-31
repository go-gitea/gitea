#!/usr/bin/env node
import {initWasm, Resvg} from '@resvg/resvg-wasm';
import {optimize} from 'svgo';
import {readFile, writeFile} from 'node:fs/promises';
import {argv, exit} from 'node:process';

async function generate(svg: string, path: string, {size, bg}: {size: number, bg?: boolean}) {
  const outputFile = new URL(path, import.meta.url);

  if (String(outputFile).endsWith('.svg')) {
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

function faviconWithStatusDot(svg: string, color: string) {
  // eslint-disable-next-line github/unescaped-html-literal
  return svg.replace('</svg>', `<circle cx="500" cy="500" r="110" fill="${color}" stroke="#fff" stroke-width="40"/></svg>`);
}

async function main() {
  const gitea = argv.slice(2).includes('gitea');
  const logoSvg = await readFile(new URL('../assets/logo.svg', import.meta.url), 'utf8');
  const faviconSvg = await readFile(new URL('../assets/favicon.svg', import.meta.url), 'utf8');
  const faviconSuccessSvg = faviconWithStatusDot(faviconSvg, '#1f883d');
  const faviconPendingSvg = faviconWithStatusDot(faviconSvg, '#bf8700');
  const faviconFailureSvg = faviconWithStatusDot(faviconSvg, '#cf222e');
  await initWasm(await readFile(new URL(import.meta.resolve('@resvg/resvg-wasm/index_bg.wasm'))));

  await Promise.all([
    generate(logoSvg, '../public/assets/img/logo.svg', {size: 32}),
    generate(logoSvg, '../public/assets/img/logo.png', {size: 512}),
    generate(faviconSvg, '../public/assets/img/favicon.svg', {size: 32}),
    generate(faviconSvg, '../public/assets/img/favicon.png', {size: 180}),
    generate(faviconSuccessSvg, '../public/assets/img/favicon-success.svg', {size: 32}),
    generate(faviconSuccessSvg, '../public/assets/img/favicon-success.png', {size: 180}),
    generate(faviconPendingSvg, '../public/assets/img/favicon-pending.svg', {size: 32}),
    generate(faviconPendingSvg, '../public/assets/img/favicon-pending.png', {size: 180}),
    generate(faviconFailureSvg, '../public/assets/img/favicon-failure.svg', {size: 32}),
    generate(faviconFailureSvg, '../public/assets/img/favicon-failure.png', {size: 180}),
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
