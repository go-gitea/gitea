import {defineConfig, loadEnv} from 'vite';
import {fileURLToPath} from 'node:url';
import {parse, dirname, extname} from 'node:path';
import {rmSync, mkdirSync, readFileSync} from 'node:fs';
import {stringPlugin} from 'vite-string-plugin';
import wrapAnsi from 'wrap-ansi';
import fastGlob from 'fast-glob';
import vue from '@vitejs/plugin-vue';
import licensePlugin from 'rollup-plugin-license';

const glob = (pattern) => fastGlob.sync(pattern, {
  cwd: dirname(fileURLToPath(new URL(import.meta.url))),
  absolute: true,
});

function formatLicenseText(licenseText) {
  return wrapAnsi(licenseText || '', 80).trim();
}

function cleanOutDirPlugin() {
  return {
    name: 'vite-clean-out-dir-plugin',
    buildStart: () => {
      rmSync(new URL('public/js', import.meta.url), {recursive: true, force: true});
      rmSync(new URL('public/css', import.meta.url), {recursive: true, force: true});
      rmSync(new URL('public/fonts', import.meta.url), {recursive: true, force: true});
      mkdirSync(new URL('public/js', import.meta.url), {recursive: true});
      mkdirSync(new URL('public/css', import.meta.url), {recursive: true});
      mkdirSync(new URL('public/fonts', import.meta.url), {recursive: true});
    }
  };
}

export default defineConfig(({mode}) => {
  const envDir = fileURLToPath(new URL(dirname(import.meta.url)));
  const env = loadEnv(mode, envDir);
  const isProduction = mode !== 'development';

  const themes = {};
  for (const path of glob('web_src/css/themes/*.css')) {
    themes[parse(path).name] = path;
  }

  let sourceMapEnabled;
  if ('VITE_ENABLE_SOURCEMAP' in env) {
    sourceMapEnabled = env.VITE_ENABLE_SOURCEMAP === 'true';
  } else {
    sourceMapEnabled = !isProduction;
  }

  return {
    root: fileURLToPath(new URL('.', import.meta.url)),
    base: '/',
    publicDir: false,
    logLevel: 'info',
    clearScreen: false,
    open: false,
    envDir,
    build: {
      outDir: fileURLToPath(new URL('public', import.meta.url)),
      emptyOutDir: false,
      rollupOptions: {
        input: {
          index: fileURLToPath(new URL('web_src/js/entry/index.js', import.meta.url)),
          webcomponents: fileURLToPath(new URL('web_src/js/entry/webcomponents.js', import.meta.url)),
          swagger: fileURLToPath(new URL('web_src/js/entry/swagger.js', import.meta.url)),
          'eventsource.sharedworker': fileURLToPath(new URL('web_src/js/entry/eventsource.sharedworker.js', import.meta.url)),
          ...(!isProduction && {
            devtest: fileURLToPath(new URL('web_src/js/entry/devtest.js', import.meta.url)),
          }),
          ...themes,
        },
        output: {
          entryFileNames: 'js/[name].js',
          chunkFileNames: 'js/[name].[hash:8].js',
          assetFileNames: ({name}) => {
            name = name.split('?')[0];
            if (name === 'index.css') return `css/${name}`;
            if (name.startsWith('theme')) return `css/${name}`;
            if (/\.js$/i.test(name)) return `css/[name].[hash:8].js`;
            if (/\.css$/i.test(name)) return `css/[name].[hash:8].css`;
            if (/\.(ttf|woff2?)$/i.test(name)) return `fonts/[name].[hash:8]${extname(name)}`;
            if (/\.png$/i.test(name)) return `img/bundled/[name].[hash:8]${extname(name)}`;
            if (name === 'editor.main') return 'js/[name].[hash:8].js';
            throw new Error(`Unable to match asset ${name} to path, please add it in vite.config.js`);
          },
        },
      },
      minify: false,
      target: 'modules',
      chunkSizeWarningLimit: Infinity,
      assetsInlineLimit: 32768,
      reportCompressedSize: false,
      sourcemap: sourceMapEnabled,
    },
    css: {
      transformer: 'lightningcss',
    },
    esbuild: {
      legalComments: 'none',
    },
    plugins: [
      cleanOutDirPlugin(),
      stringPlugin(),
      vue(),
      licensePlugin({
        thirdParty: {
          output: {
            file: fileURLToPath(new URL('public/js/licenses.txt', import.meta.url)),
            template(dependencies) {
              const line = '-'.repeat(80);
              const goJson = readFileSync('assets/go-licenses.json', 'utf8');
              const goModules = JSON.parse(goJson).map(({name, licenseText}) => {
                return {name, body: formatLicenseText(licenseText)};
              });
              const jsModules = dependencies.map(({name, version, licenseName, licenseText}) => {
                return {name, version, licenseName, body: formatLicenseText(licenseText)};
              });

              const modules = [...goModules, ...jsModules].sort((a, b) => a.name.localeCompare(b.name));
              return modules.map(({name, version, body}) => {
                return `${line}\n${name}@${version}\n${line}\n${body}`;
              }).join('\n');
            },
          },
        },
      }),
    ],
  };
});
