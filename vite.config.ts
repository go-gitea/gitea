import {build, defineConfig} from 'vite';
import vuePlugin from '@vitejs/plugin-vue';
import {stringPlugin} from 'vite-string-plugin';
import {licensePlugin, wrap} from 'rolldown-license-plugin';
import {readFileSync, writeFileSync, mkdirSync, unlinkSync, globSync} from 'node:fs';
import path, {basename, join, parse} from 'node:path';
import {env} from 'node:process';
import tailwindcss from 'tailwindcss';
import tailwindConfig from './tailwind.config.ts';
import type {InlineConfig, Plugin, Rolldown} from 'vite';
import {camelize} from 'vue';

const isProduction = env.NODE_ENV !== 'development';

// ENABLE_SOURCEMAP accepts the following values:
// true - all sourcemaps enabled, the default in development
// reduced - sourcemaps only for index.js, the default in production
// false - all sourcemaps disabled
let enableSourcemap: string;
if ('ENABLE_SOURCEMAP' in env) {
  enableSourcemap = ['true', 'false'].includes(env.ENABLE_SOURCEMAP!) ? env.ENABLE_SOURCEMAP! : 'reduced';
} else {
  enableSourcemap = isProduction ? 'reduced' : 'true';
}
const outDir = join(import.meta.dirname, 'public/assets');

const themes: Record<string, string> = {};
for (const path of globSync('web_src/css/themes/*.css', {cwd: import.meta.dirname})) {
  themes[parse(path).name] = join(import.meta.dirname, path);
}

const webComponents = new Set([
  // our own, in web_src/js/webcomponents
  'overflow-menu',
  'relative-time',
  // from dependencies
  'markdown-toolbar',
  'text-expander',
]);

const commonRolldownOptions: Rolldown.RolldownOptions = {
  checks: {
    pluginTimings: false,
  },
};

function commonViteOpts({build, ...other}: InlineConfig): InlineConfig {
  const {rolldownOptions, ...otherBuild} = build || {};
  return {
    base: './', // make all asset URLs relative, so it works in subdirectory deployments
    configFile: false,
    root: import.meta.dirname,
    publicDir: false,
    build: {
      outDir,
      emptyOutDir: false,
      sourcemap: enableSourcemap !== 'false',
      target: 'es2020',
      minify: isProduction ? 'oxc' : false,
      cssMinify: isProduction ? 'esbuild' : false,
      chunkSizeWarningLimit: Infinity,
      assetsInlineLimit: 32768,
      reportCompressedSize: false,
      rolldownOptions: {
        ...commonRolldownOptions,
        ...rolldownOptions,
      },
      ...otherBuild,
    },
    ...other,
  };
}

function iifeBuildOpts({sourceFileName, write}: {sourceFileName: string, write?: boolean}) {
  const sourceBaseName = basename(sourceFileName, '.ts');
  // HINT: VITE-OUTPUT-DIR: all outputted JS files are in "js" directory
  const entryFileName = `js/${sourceBaseName}.[hash:8].js`;
  return commonViteOpts({
    build: {
      lib: {entry: join(import.meta.dirname, 'web_src/js', sourceFileName), name: camelize(sourceBaseName), formats: ['iife']},
      rolldownOptions: {output: {entryFileNames: entryFileName}},
      ...(write === false && {write: false}),
    },
    plugins: [stringPlugin()],
  });
}

// Build iife.js as a blocking IIFE bundle. In dev mode, serves it from memory
// and rebuilds on file changes. In prod mode, writes to disk during closeBundle.
function iifePlugin(sourceFileName: string): Plugin {
  let iifeCode = '', iifeMap = '';
  const iifeModules = new Set<string>();
  let isBuilding = false;

  const sourceBaseName = path.basename(sourceFileName, '.ts');
  return {
    name: `iife:${sourceFileName}`, // plugin name
    async configureServer(server) {
      const buildAndCache = async () => {
        const result = await build(iifeBuildOpts({sourceFileName, write: false}));
        const output = (Array.isArray(result) ? result[0] : result) as Rolldown.RolldownOutput;
        const chunk = output.output[0];
        iifeCode = chunk.code.replace(/\/\/# sourceMappingURL=.*/, `//# sourceMappingURL=${sourceBaseName}.js.map`);
        const mapAsset = output.output.find((o) => o.fileName.endsWith('.map'));
        iifeMap = mapAsset && 'source' in mapAsset ? String(mapAsset.source) : '';
        iifeModules.clear();
        for (const id of Object.keys(chunk.modules)) iifeModules.add(id);
      };
      await buildAndCache();

      let needsRebuild = false;
      server.watcher.on('change', async (path) => {
        if (!iifeModules.has(path)) return;
        needsRebuild = true;
        if (isBuilding) return;
        isBuilding = true;
        try {
          do {
            needsRebuild = false;
            await buildAndCache();
          } while (needsRebuild);
          server.ws.send({type: 'full-reload'});
        } finally {
          isBuilding = false;
        }
      });

      server.middlewares.use((req, res, next) => {
        // on the dev server, an "iife" file is a virtual file in memory, serve it directly
        const pathname = req.url!.split('?')[0];
        if (pathname === '/web_src/js/__vite_dev_server_check') {
          res.end('ok');
        } else if (pathname === `/web_src/js/${sourceFileName}`) {
          res.setHeader('Content-Type', 'application/javascript');
          res.setHeader('Cache-Control', 'no-store');
          res.end(iifeCode);
        } else if (pathname === `/web_src/js/${sourceBaseName}.js.map`) {
          res.setHeader('Content-Type', 'application/json');
          res.setHeader('Cache-Control', 'no-store');
          res.end(iifeMap);
        } else {
          next();
        }
      });
    },
    async closeBundle() {
      for (const file of globSync(`js/${sourceBaseName}.*.js*`, {cwd: outDir})) unlinkSync(join(outDir, file));

      const result = await build(iifeBuildOpts({sourceFileName}));
      const buildOutput = (Array.isArray(result) ? result[0] : result) as Rolldown.RolldownOutput;
      const entry = buildOutput.output.find((o) => o.fileName.startsWith(`js/${sourceBaseName}.`));
      if (!entry) throw new Error('IIFE build produced no output');

      const manifestPath = join(outDir, '.vite', 'manifest.json');
      const manifestData = JSON.parse(readFileSync(manifestPath, 'utf8'));
      manifestData[`web_src/js/${sourceFileName}`] = {file: entry.fileName, name: sourceBaseName, isEntry: true};
      writeFileSync(manifestPath, JSON.stringify(manifestData, null, 2));
    },
  };
}

// In reduced sourcemap mode, only keep sourcemaps for main files
function reducedSourcemapPlugin(): Plugin {
  const standalonePrefixes = [
    'js/index.',
    'js/iife.',
    'js/swagger.',
    'js/external-render-helper.',
    'js/eventsource.sharedworker.',
  ];
  return {
    name: 'reduced-sourcemap',
    apply: 'build',
    closeBundle() {
      if (enableSourcemap !== 'reduced') return;
      for (const file of globSync('{js,css}/*.map', {cwd: outDir})) {
        if (standalonePrefixes.some((prefix) => file.startsWith(prefix))) continue;
        unlinkSync(join(outDir, file));
      }
    },
  };
}

// Filter out legacy font formats from CSS, keeping only woff2
function filterCssUrlPlugin(): Plugin {
  return {
    name: 'filter-css-url',
    enforce: 'pre',
    transform(code, id) {
      if (!id.endsWith('.css') || !id.includes('katex')) return null;
      return code.replace(/,\s*url\([^)]*\.(?:woff|ttf)\)\s*format\("[^"]*"\)/gi, '');
    },
  };
}

const viteDevServerPort = Number(env.FRONTEND_DEV_SERVER_PORT) || 3001;
const viteDevPortFilePath = join(outDir, '.vite', 'dev-port');

// Write the Vite dev server's actual port to a file so the Go server can discover it for proxying.
function viteDevServerPortPlugin(): Plugin {
  return {
    name: 'vite-dev-server-port',
    apply: 'serve',
    configureServer(server) {
      server.httpServer!.once('listening', () => {
        const addr = server.httpServer!.address();
        if (typeof addr === 'object' && addr) {
          mkdirSync(path.dirname(viteDevPortFilePath), {recursive: true});
          writeFileSync(viteDevPortFilePath, String(addr.port));
        }
      });
    },
  };
}

export default defineConfig(commonViteOpts({
  appType: 'custom', // Go serves all HTML, disable Vite's HTML handling
  clearScreen: false,
  server: {
    port: viteDevServerPort,
    open: false,
    host: '0.0.0.0',
    strictPort: false,
    cors: true,
    fs: {
      // VITE-DEV-SERVER-SECURITY: the dev server will be exposed to public by Gitea's web server, so we need to strictly limit the access
      // Otherwise `/@fs/*` will be able to access any file (including app.ini which contains INTERNAL_TOKEN)
      strict: true,
      allow: [
        'assets',
        'node_modules',
        'public',
        'web_src',
        // do not add any other directories here, unless you are absolutely sure it's safe to expose them to the public
      ],
    },
    headers: {
      'Cache-Control': 'no-store', // prevent browser disk cache
    },
    warmup: {
      clientFiles: [
        // warmup the important entry points
        'web_src/js/index.ts',
        'web_src/css/index.css',
        'web_src/css/themes/*.css',
      ],
    },
  },
  build: {
    modulePreload: false,
    manifest: true,
    rolldownOptions: {
      input: {
        index: join(import.meta.dirname, 'web_src/js/index.ts'),
        swagger: join(import.meta.dirname, 'web_src/js/swagger.ts'),
        'eventsource.sharedworker': join(import.meta.dirname, 'web_src/js/eventsource.sharedworker.ts'),
        devtest: join(import.meta.dirname, 'web_src/css/devtest.css'),
        ...themes,
      },
      output: {
        // HINT: VITE-OUTPUT-DIR: all outputted JS files are in "js" directory
        // So standalone/iife source files should also be in "js" directory,
        // to keep consistent between production and dev server, avoid unexpected behaviors.
        entryFileNames: 'js/[name].[hash:8].js',
        chunkFileNames: 'js/[name].[hash:8].js',
        assetFileNames: ({names}) => {
          const name = names[0];
          if (name.endsWith('.css')) return 'css/[name].[hash:8].css';
          if (/\.(ttf|woff2?)$/.test(name)) return 'fonts/[name].[hash:8].[ext]';
          return '[name].[hash:8].[ext]';
        },
      },
    },
  },
  worker: {
    rolldownOptions: {
      ...commonRolldownOptions,
      output: {
        entryFileNames: 'js/[name].[hash:8].js',
      },
    },
  },
  css: {
    transformer: 'postcss',
    postcss: {
      plugins: [
        tailwindcss(tailwindConfig),
      ],
    },
  },
  define: {
    __VUE_OPTIONS_API__: true,
    __VUE_PROD_DEVTOOLS__: false,
    __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: false,
  },
  plugins: [
    iifePlugin('iife.ts'),
    iifePlugin('external-render-helper.ts'),
    viteDevServerPortPlugin(),
    reducedSourcemapPlugin(),
    filterCssUrlPlugin(),
    stringPlugin(),
    vuePlugin({
      template: {
        compilerOptions: {
          isCustomElement: (tag) => webComponents.has(tag),
        },
      },
    }),
    isProduction ? licensePlugin({
      done(deps, context) {
        const line = '-'.repeat(80);
        const goLicenses = JSON.parse(readFileSync(join(import.meta.dirname, 'assets/go-licenses.json'), 'utf8'));
        const combined: Record<string, string> = {};
        for (const {name, licenseText} of goLicenses) {
          combined[name] = wrap(licenseText || '', 80).trim();
        }
        for (const {name, version, licenseText} of deps) {
          combined[`${name}@${version}`] = wrap(licenseText, 80).trim();
        }
        const content = Object.entries(combined)
          .sort(([a], [b]) => a.localeCompare(b))
          .map(([title, body]) => `${line}\n${title}\n${line}\n${body}`).join('\n');
        context.emitFile({type: 'asset', fileName: 'licenses.txt', source: content});
      },
      match: /^((UN)?LICEN(S|C)E|COPYING).*$/i, // also defined in build/generate-go-licenses.go
      allow(dep) {
        if (dep.name === 'khroma') return true; // MIT: https://github.com/fabiospampinato/khroma/pull/33
        return /(Apache-2\.0|0BSD|BSD-2-Clause|BSD-3-Clause|MIT|ISC|CPAL-1\.0|Unlicense|EPL-1\.0|EPL-2\.0)/.test(dep.license);
      },
    }) : {
      name: 'dev-licenses-stub',
      configureServer() {
        writeFileSync(join(outDir, 'licenses.txt'), 'Licenses are disabled during development');
      },
    },
  ],
}));
