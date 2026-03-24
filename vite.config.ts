import {build, defineConfig} from 'vite';
import vuePlugin from '@vitejs/plugin-vue';
import {stringPlugin} from 'vite-string-plugin';
import {readFileSync, writeFileSync, unlinkSync, globSync} from 'node:fs';
import {join, parse} from 'node:path';
import {env} from 'node:process';
import tailwindcss from 'tailwindcss';
import tailwindConfig from './tailwind.config.ts';
import wrapAnsi from 'wrap-ansi';
import licensePlugin from 'rollup-plugin-license';
import type {InlineConfig, Plugin, Rolldown} from 'vite';

const isProduction = env.NODE_ENV !== 'development';
const enableSourcemap = env.ENABLE_SOURCEMAP ? env.ENABLE_SOURCEMAP === 'true' : !isProduction;
const outDir = join(import.meta.dirname, 'public/assets');

const themes: Record<string, string> = {};
for (const path of globSync('web_src/css/themes/*.css', {cwd: import.meta.dirname})) {
  themes[parse(path).name] = join(import.meta.dirname, path);
}

const webComponents = new Set([
  // our own, in web_src/js/webcomponents
  'overflow-menu',
  'origin-url',
  'relative-time',
  // from dependencies
  'markdown-toolbar',
  'text-expander',
]);

function formatLicenseText(licenseText: string) {
  return wrapAnsi(licenseText || '', 80).trim();
}

const commonRolldownOptions: Rolldown.RolldownOptions = {
  checks: {
    eval: false, // htmx needs eval
    pluginTimings: false,
  },
};

function commonViteOpts({build, ...other}: InlineConfig): InlineConfig {
  const {rolldownOptions, ...otherBuild} = build || {};
  return {
    configFile: false,
    root: import.meta.dirname,
    publicDir: false,
    build: {
      outDir,
      emptyOutDir: false,
      sourcemap: enableSourcemap,
      target: 'es2020',
      minify: isProduction ? 'oxc' : false,
      cssMinify: isProduction ? 'esbuild' : false,
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

// Build iife.js as a blocking IIFE bundle to avoid pop-in effects
function iifePlugin(): Plugin {
  return {
    name: 'iife',
    async closeBundle() {
      // Clean up old hashed files before rebuilding
      for (const file of globSync('js/iife.*.js*', {cwd: outDir})) unlinkSync(join(outDir, file));

      const result = await build(commonViteOpts({
        build: {
          lib: {
            entry: join(import.meta.dirname, 'web_src/js/iife.ts'),
            formats: ['iife'],
            name: 'iife',
          },
          rolldownOptions: {
            output: {
              entryFileNames: 'js/iife.[hash:8].js',
            },
          },
        },
        define: {
          // needed for tippy.js
          'process.env.NODE_ENV': JSON.stringify(isProduction ? 'production' : 'development'),
        },
        plugins: [
          stringPlugin(),
        ],
      }));

      // Append IIFE entry to the main Vite manifest
      const manifestPath = join(outDir, '.vite', 'manifest.json');
      const buildOutput = (Array.isArray(result) ? result[0] : result) as Rolldown.RolldownOutput;
      const entry = buildOutput.output.find((o) => o.fileName.startsWith('js/iife.'));
      if (!entry) throw new Error('IIFE build produced no output');
      writeFileSync(manifestPath, JSON.stringify({
        ...JSON.parse(readFileSync(manifestPath, 'utf8')),
        'web_src/js/iife.ts': {file: entry.fileName, name: 'iife', isEntry: true},
      }, null, 2));
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

export default defineConfig(commonViteOpts({
  base: './',
  build: {
    modulePreload: false,
    manifest: true,
    chunkSizeWarningLimit: Infinity,
    rolldownOptions: {
      input: {
        index: join(import.meta.dirname, 'web_src/js/index.ts'),
        swagger: join(import.meta.dirname, 'web_src/js/standalone/swagger.ts'),
        'external-render-iframe': join(import.meta.dirname, 'web_src/js/standalone/external-render-iframe.ts'),
        sharedworker: join(import.meta.dirname, 'web_src/js/features/sharedworker.ts'),
        ...(!isProduction && {
          devtest: join(import.meta.dirname, 'web_src/js/standalone/devtest.ts'),
        }),
        ...themes,
      },
      output: {
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
    iifePlugin(),
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
      thirdParty: {
        output: {
          file: join(import.meta.dirname, 'public/assets/licenses.txt'),
          template(deps) {
            const line = '-'.repeat(80);
            const goJson = readFileSync(join(import.meta.dirname, 'assets/go-licenses.json'), 'utf8');
            const goModules = JSON.parse(goJson).map(({name, licenseText}: {name: string, licenseText: string}) => {
              return {name, body: formatLicenseText(licenseText)};
            });
            const jsModules = deps.map((dep) => {
              return {name: dep.name, version: dep.version, body: formatLicenseText(dep.licenseText ?? '')};
            });
            const modules = [...goModules, ...jsModules].sort((a, b) => a.name.localeCompare(b.name));
            return modules.map(({name, version, body}: {name: string, version?: string, body: string}) => {
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
}));
