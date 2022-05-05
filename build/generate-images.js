import imageminZopfli from 'imagemin-zopfli';
import {optimize} from 'svgo';
import {fabric} from 'fabric';
import fs from 'fs';
import {resolve, dirname} from 'path';
import {fileURLToPath} from 'url';

const {readFile, writeFile} = fs.promises;
const __dirname = dirname(fileURLToPath(import.meta.url));
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

async function generate(svg, outputFile, {size, bg}) {
  if (outputFile.endsWith('.svg')) {
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
  const svg = await readFile(logoFile, 'utf8');

  await Promise.all([
    generate(svg, resolve(__dirname, '../public/img/logo.svg'), {size: 32}),
    generate(svg, resolve(__dirname, '../public/img/logo.png'), {size: 512}),
    generate(svg, resolve(__dirname, '../public/img/favicon.png'), {size: 180}),
    generate(svg, resolve(__dirname, '../public/img/avatar_default.png'), {size: 200}),
    generate(svg, resolve(__dirname, '../public/img/apple-touch-icon.png'), {size: 180, bg: true}),
    gitea && generate(svg, resolve(__dirname, '../public/img/gitea.svg'), {size: 32}),
  ]);
}

main().then(exit).catch(exit);

