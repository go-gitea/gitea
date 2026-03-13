import {build, defineConfig, type Plugin} from 'vite';
import vuePlugin from '@vitejs/plugin-vue';
import {stringPlugin} from 'vite-string-plugin';
import {readFileSync, writeFileSync, unlinkSync, globSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {join, parse} from 'node:path';
import {env} from 'node:process';
import tailwindcss from 'tailwindcss';
import tailwindConfig from './tailwind.config.ts';
import wrapAnsi from 'wrap-ansi';
import licensePlugin from 'rollup-plugin-license';

const isProduction = env.NODE_ENV !== 'development';

// ENABLE_SOURCEMAP accepts 'true', 'false', or 'reduced'.
// Vite does not support partial sourcemaps, so 'reduced' is treated as 'true'.
let enableSourcemap: boolean;
if ('ENABLE_SOURCEMAP' in env) {
  enableSourcemap = env.ENABLE_SOURCEMAP !== 'false';
} else {
  enableSourcemap = !isProduction;
}

const outDir = fileURLToPath(new URL('public/assets', import.meta.url));
const buildTarget = 'es2020';

const themes: Record<string, string> = {};
for (const path of globSync('web_src/css/themes/*.css', {cwd: import.meta.dirname})) {
  themes[parse(path).name] = fileURLToPath(new URL(path, import.meta.url));
}

const webComponents = new Set([
  // our own, in web_src/js/webcomponents
  'overflow-menu',
  'origin-url',
  // from dependencies
  'markdown-toolbar',
  'relative-time',
  'text-expander',
]);

const formatLicenseText = (licenseText: string) => wrapAnsi(licenseText || '', 80).trim();

// Build web components as a separate IIFE bundle that loads as a blocking script
// to prevent flash of unstyled content. This runs as part of the main build.
function webcomponentsPlugin(): Plugin {
  return {
    name: 'webcomponents-iife',
    async closeBundle() {
      // Clean up old hashed webcomponents files before rebuilding
      for (const file of globSync('js/webcomponents.*.js*', {cwd: outDir})) {
        unlinkSync(join(outDir, file));
      }

      const result = await build({
        configFile: false,
        root: import.meta.dirname,
        publicDir: false,
        build: {
          outDir,
          emptyOutDir: false,
          sourcemap: enableSourcemap,
          target: buildTarget,
          minify: isProduction,
          reportCompressedSize: false,
          lib: {
            entry: fileURLToPath(new URL('web_src/js/webcomponents/index.ts', import.meta.url)),
            formats: ['iife'],
            name: 'webcomponents',
          },
          rolldownOptions: {
            output: {
              entryFileNames: 'js/webcomponents.[hash:8].js',
            },
          },
        },
        define: {
          'process.env.NODE_ENV': JSON.stringify(isProduction ? 'production' : 'development'), // for tippy.js
        },
        plugins: [
          stringPlugin(),
        ],
      });

      // Append webcomponents entry to the main Vite manifest
      for (const buildOutput of (Array.isArray(result) ? result : [result])) {
        if (!('output' in buildOutput)) continue;
        const entry = buildOutput.output.find((o: {fileName: string}) => o.fileName.startsWith('js/webcomponents.'));
        if (entry) {
          const manifestPath = join(outDir, '.vite', 'manifest.json');
          const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
          manifest['web_src/js/webcomponents/index.ts'] = {
            file: entry.fileName,
            name: 'webcomponents',
            isEntry: true,
          };
          writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
          break;
        }
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

export default defineConfig({
  root: import.meta.dirname,
  base: './',
  publicDir: false,
  build: {
    outDir,
    emptyOutDir: false,
    modulePreload: false,
    sourcemap: enableSourcemap,
    target: buildTarget,
    minify: isProduction,
    manifest: true,
    chunkSizeWarningLimit: Infinity,
    reportCompressedSize: false,
    rolldownOptions: {
      checks: {
        eval: false, // htmx needs eval
        pluginTimings: false,
      },
      input: {
        index: fileURLToPath(new URL('web_src/js/index.ts', import.meta.url)),
        swagger: fileURLToPath(new URL('web_src/js/standalone/swagger.ts', import.meta.url)),
        'external-render-iframe': fileURLToPath(new URL('web_src/js/standalone/external-render-iframe.ts', import.meta.url)),
        sharedworker: fileURLToPath(new URL('web_src/js/features/sharedworker.ts', import.meta.url)),
        ...(!isProduction && {
          devtest: fileURLToPath(new URL('web_src/js/standalone/devtest.ts', import.meta.url)),
        }),
        ...themes,
      },
      output: {
        entryFileNames: 'js/[name].[hash:8].js',
        chunkFileNames: 'js/[name].[hash:8].js',
        assetFileNames: (info: {name?: string}) => {
          const name = (info.name ?? '').split('?')[0];
          if (/\.css$/i.test(name)) {
            return 'css/[name].[hash:8].css';
          }
          if (/\.(ttf|woff2?)$/i.test(name)) return 'fonts/[name].[hash:8].[ext]';
          return '[name].[hash:8].[ext]';
        },
      },
    },
  },
  worker: {
    rolldownOptions: {
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
  experimental: {
    renderBuiltUrl(filename, {hostType}) {
      if (hostType === 'css') {
        return `../${filename}`; // CSS files are in css/, assets are siblings, so go up one level
      }
      return {relative: true};
    },
  },
  define: {
    __VUE_OPTIONS_API__: true,
    __VUE_PROD_DEVTOOLS__: false,
    __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: false,
  },
  plugins: [
    webcomponentsPlugin(),
    filterCssUrlPlugin(),
    stringPlugin(),
    vuePlugin({
      template: {
        compilerOptions: {
          isCustomElement: (tag: string) => webComponents.has(tag),
        },
      },
    }),
    isProduction ? licensePlugin({
      thirdParty: {
        output: {
          file: fileURLToPath(new URL('public/assets/licenses.txt', import.meta.url)),
          template(dependencies) {
            const line = '-'.repeat(80);
            const goJson = readFileSync('assets/go-licenses.json', 'utf8');
            const goModules = JSON.parse(goJson).map(({name, licenseText}: Record<string, string>) => {
              return {name, body: formatLicenseText(licenseText)};
            });
            const jsModules = dependencies.map((dep) => {
              return {name: dep.name, version: dep.version, body: formatLicenseText(dep.licenseText ?? '')};
            });

            const modules = [...goModules, ...jsModules].sort((a, b) => a.name.localeCompare(b.name));
            return modules.map(({name, version, body}: Record<string, string>) => {
              const title = version ? `${name}@${version}` : name;
              return `${line}\n${title}\n${line}\n${body}`;
            }).join('\n');
          },
        },
        allow(dependency) {
          if (dependency.name === 'khroma') return true; // MIT: https://github.com/fabiospampinato/khroma/pull/33
          return /(Apache-2\.0|0BSD|BSD-2-Clause|BSD-3-Clause|MIT|ISC|CPAL-1\.0|Unlicense|EPL-1\.0|EPL-2\.0)/.test(dependency.license ?? '');
        },
      },
    }) : {
      name: 'dev-licenses-stub',
      closeBundle() {
        writeFileSync(join(outDir, 'licenses.txt'), 'Licenses are disabled during development');
      },
    },
  ],
});
