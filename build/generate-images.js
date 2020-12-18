#!/usr/bin/env node
'use strict';

const imageminZopfli = require('imagemin-zopfli');
const {fabric} = require('fabric');
const {readFile, writeFile} = require('fs').promises;
const {resolve} = require('path');
const Svgo = require('svgo');

const logoFile = resolve(__dirname, '../assets/logo.svg');

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

async function generateSvgFavicon(svg, outputFile) {
  const svgo = new Svgo({
    plugins: [
      {removeDimensions: true},
      {
        addAttributesToSVGElement: {
          attributes: [
            {'width': '32'},
            {'height': '32'},
          ],
        },
      },
    ],
  });

  const {data} = await svgo.optimize(svg);
  await writeFile(outputFile, data);
}

async function generateSvg(svg, outputFile) {
  const svgo = new Svgo();
  const {data} = await svgo.optimize(svg);
  await writeFile(outputFile, data);
}

async function generate(svg, outputFile, {size, bg}) {
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

  const svg = await readFile(logoFile, 'utf8');
  await Promise.all([
    generateSvgFavicon(svg, resolve(__dirname, '../public/img/favicon.svg')),
    generateSvg(svg, resolve(__dirname, '../public/img/logo.svg')),
    generate(svg, resolve(__dirname, '../public/img/logo-lg.png'), {size: 880}),
    generate(svg, resolve(__dirname, '../public/img/logo-512.png'), {size: 512}),
    generate(svg, resolve(__dirname, '../public/img/logo-192.png'), {size: 192}),
    generate(svg, resolve(__dirname, '../public/img/logo-sm.png'), {size: 120}),
    generate(svg, resolve(__dirname, '../public/img/avatar_default.png'), {size: 200}),
    generate(svg, resolve(__dirname, '../public/img/favicon.png'), {size: 180}),
    generate(svg, resolve(__dirname, '../public/img/apple-touch-icon.png'), {size: 180, bg: true}),
  ]);
  if (gitea) {
    await Promise.all([
      generateSvg(svg, resolve(__dirname, '../public/img/gitea.svg')),
      generate(svg, resolve(__dirname, '../public/img/gitea-192.png'), {size: 192}),
    ]);
  }
}

main().then(exit).catch(exit);

