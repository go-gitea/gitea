#!/usr/bin/env node
import imageminZopfli from 'imagemin-zopfli';
import {optimize} from 'svgo';
import {fabric} from 'fabric';
import {readFile, writeFile} from 'fs/promises';

function exit(err) {
  if (err) console.error(err);
  process.exit(err ? 1 : 0);
}

function loadSvg(svg) {
  return new Promise((resolve) => {
    fabric.loadSVGFromString(svg, (objects, options) => {
      resolve({objects, options});
    });
  });
}

async function generate(svg, outputFile, {size, bg}) {
  if (String(outputFile).endsWith('.svg')) {
    const {data} = optimize(svg, {
      plugins: [
        'preset-default',
        'removeDimensions',
        {
          name: 'addAttributesToSVGElement',
          params: {attributes: [{width: size}, {height: size}]}
        },
      ],
    });
    await writeFile(outputFile, data);
    return;
  }

  const {objects, options} = await loadSvg(svg);
  const canvas = new fabric.Canvas();
  canvas.setDimensions({width: size, height: size});
  const ctx = canvas.getContext('2d');
  ctx.scale(options.width ? (size / options.width) : 1, options.height ? (size / options.height) : 1);

  if (bg) {
    canvas.add(new fabric.Rect({
      left: 0,
      top: 0,
      height: size * (1 / (size / options.height)),
      width: size * (1 / (size / options.width)),
      fill: 'white',
    }));
  }

  canvas.add(fabric.util.groupSVGElements(objects, options));
  canvas.renderAll();

  let png = Buffer.from([]);
  for await (const chunk of canvas.createPNGStream()) {
    png = Buffer.concat([png, chunk]);
  }

  png = await imageminZopfli({more: true})(png);
  await writeFile(outputFile, png);
}

async function main() {
  const gitea = process.argv.slice(2).includes('gitea');
  const logoSvg = await readFile(new URL('../assets/logo.svg', import.meta.url), 'utf8');
  const faviconSvg = await readFile(new URL('../assets/favicon.svg', import.meta.url), 'utf8');

  await Promise.all([
    generate(logoSvg, new URL('../public/img/logo.svg', import.meta.url), {size: 32}),
    generate(logoSvg, new URL('../public/img/logo.png', import.meta.url), {size: 512}),
    generate(faviconSvg, new URL('../public/img/favicon.svg', import.meta.url), {size: 32}),
    generate(faviconSvg, new URL('../public/img/favicon.png', import.meta.url), {size: 180}),
    generate(logoSvg, new URL('../public/img/avatar_default.png', import.meta.url), {size: 200}),
    generate(logoSvg, new URL('../public/img/apple-touch-icon.png', import.meta.url), {size: 180, bg: true}),
    gitea && generate(logoSvg, new URL('../public/img/gitea.svg', import.meta.url), {size: 32}),
  ]);
}

main().then(exit).catch(exit);
