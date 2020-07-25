#!/usr/bin/env node
'use strict';

const imageminZopfli = require('imagemin-zopfli'); // eslint-disable-line import/no-unresolved
const svg2img = require('svg2img'); // eslint-disable-line import/no-unresolved
const {DOMParser, XMLSerializer} = require('xmldom'); // eslint-disable-line import/no-unresolved
const {promisify} = require('util');
const {readFile, writeFile} = require('fs').promises;
const {resolve} = require('path');

function exit(err) {
  if (err) console.error(err);
  process.exit(err ? 1 : 0);
}

async function generate(svg, outputFile, {size, bg, removeDetail} = {}) {
  const parser = new DOMParser();
  const serializer = new XMLSerializer();
  const document = parser.parseFromString(svg);

  if (bg) {
    const rect = parser.parseFromString('<rect width="100%" height="100%" fill="white"/>');
    document.documentElement.insertBefore(rect, document.documentElement.firstChild);
  }

  if (removeDetail) {
    for (const el of Array.from(document.getElementsByTagName('g') || [])) {
      for (const attribute of Array.from(el.attributes || [])) {
        if (attribute.name === 'class' && attribute.value === 'detail-remove') {
          el.parentNode.removeChild(el);
        }
      }
    }
  }

  const processedSvg = serializer.serializeToString(document);
  const png = await promisify(svg2img)(processedSvg, {
    width: size,
    height: size,
    preserveAspectRatio: 'xMidYMid meet',
  });

  const optimizedPng = await imageminZopfli({more: true})(png);
  await writeFile(outputFile, optimizedPng);
}

async function main() {
  const svg = await readFile(resolve(__dirname, '../assets/logo.svg'), 'utf8');
  await generate(svg, resolve(__dirname, '../public/img/gitea-lg.png'), {size: 880});
  await generate(svg, resolve(__dirname, '../public/img/gitea-512.png'), {size: 512});
  await generate(svg, resolve(__dirname, '../public/img/gitea-192.png'), {size: 192});
  await generate(svg, resolve(__dirname, '../public/img/gitea-sm.png'), {size: 120});
  await generate(svg, resolve(__dirname, '../public/img/avatar_default.png'), {size: 200});
  await generate(svg, resolve(__dirname, '../public/img/favicon.png'), {size: 180, removeDetail: true});
  await generate(svg, resolve(__dirname, '../public/img/apple-touch-icon.png'), {size: 180, bg: true});
}

main().then(exit).catch(exit);

