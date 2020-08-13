#!/usr/bin/env node
'use strict';

const imageminZopfli = require('imagemin-zopfli');
const {fabric} = require('fabric');
const {DOMParser, XMLSerializer} = require('xmldom');
const {readFile, writeFile} = require('fs').promises;
const {resolve} = require('path');
const Svgo = require('svgo');

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

async function generate(svg, outputFile, {size, bg, removeDetail} = {}) {
  const parser = new DOMParser();
  const serializer = new XMLSerializer();
  const document = parser.parseFromString(svg);

  if (removeDetail) {
    for (const el of Array.from(document.getElementsByTagName('g') || [])) {
      for (const attribute of Array.from(el.attributes || [])) {
        if (attribute.name === 'class' && attribute.value === 'detail-remove') {
          el.parentNode.removeChild(el);
        }
      }
    }
  }

  svg = serializer.serializeToString(document);

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
  const svg = await readFile(resolve(__dirname, '../assets/logo.svg'), 'utf8');
  await generateSvgFavicon(svg, resolve(__dirname, '../public/img/favicon.svg'));
  await generate(svg, resolve(__dirname, '../public/img/gitea-lg.png'), {size: 880});
  await generate(svg, resolve(__dirname, '../public/img/gitea-512.png'), {size: 512});
  await generate(svg, resolve(__dirname, '../public/img/gitea-192.png'), {size: 192});
  await generate(svg, resolve(__dirname, '../public/img/gitea-sm.png'), {size: 120});
  await generate(svg, resolve(__dirname, '../public/img/avatar_default.png'), {size: 200});
  await generate(svg, resolve(__dirname, '../public/img/favicon.png'), {size: 180, removeDetail: true});
  await generate(svg, resolve(__dirname, '../public/img/apple-touch-icon.png'), {size: 180, bg: true});
}

main().then(exit).catch(exit);

