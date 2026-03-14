import {build, defineConfig} from 'vite';
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
import type {InlineConfig, Manifest, Plugin, Rolldown} from 'vite';

const isProduction = env.NODE_ENV !== 'development';

// ENABLE_SOURCEMAP accepts the following values:
// true - all enabled, the default in development
// reduced - minimal sourcemaps, the default in production
// false - all disabled
let sourceMaps: string | undefined;
if ('ENABLE_SOURCEMAP' in env) {
  sourceMaps = ['true', 'false'].includes(env.ENABLE_SOURCEMAP || '') ? env.ENABLE_SOURCEMAP : 'reduced';
} else {
  sourceMaps = isProduction ? 'reduced' : 'true';
}

const outDir = fileURLToPath(new URL('public/assets', import.meta.url));

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

const commonRolldownOptions: Rolldown.RolldownOptions = {
  checks: {
    eval: false, // htmx needs eval
    pluginTimings: false,
  },
};

function commonViteOpts<T extends InlineConfig>({build, ...other}: T): T {
  const {rolldownOptions, ...otherBuild} = build || {};
  return {
    configFile: false,
    root: import.meta.dirname,
    publicDir: false,
    build: {
      outDir,
      emptyOutDir: false,
      sourcemap: sourceMaps !== 'false',
      target: 'es2020',
      minify: isProduction,
      cssMinify: 'esbuild',
      reportCompressedSize: false,
      rolldownOptions: {
        ...commonRolldownOptions,
        ...rolldownOptions,
      },
      ...otherBuild,
    },
    ...other,
  } as InlineConfig & T;
}

// Build index.js as a blocking IIFE bundle, matching the pre-Vite webpack behavior.
function iifeIndexPlugin(): Plugin {
  return {
    name: 'iife-index',
    async closeBundle() {
      // Clean up old hashed files before rebuilding
      for (const file of globSync('js/index.*.js*', {cwd: outDir})) unlinkSync(join(outDir, file));
      for (const file of globSync('js/webcomponents.*.js*', {cwd: outDir})) unlinkSync(join(outDir, file));

      const result = await build(commonViteOpts({
        build: {
          lib: {
            entry: fileURLToPath(new URL('web_src/js/index.ts', import.meta.url)),
            formats: ['iife'],
            name: 'gitea',
          },
          rolldownOptions: {
            output: {
              entryFileNames: 'js/index.[hash:8].js',
            },
          },
        },
        define: {
          'process.env.NODE_ENV': JSON.stringify(isProduction ? 'production' : 'development'),
        },
        plugins: [
          stringPlugin(),
          sourceMaps === 'reduced' && reducedSourcemapPlugin(),
        ],
      }));

      // Append IIFE index entry to the main Vite manifest
      const manifestPath = join(outDir, '.vite', 'manifest.json');
      let manifest: Manifest = {};
      try { manifest = JSON.parse(readFileSync(manifestPath, 'utf8')) } catch {}
      for (const buildOutput of (Array.isArray(result) ? result : [result])) {
        if (!('output' in buildOutput)) continue;
        const entry = buildOutput.output.find((o) => o.fileName.startsWith('js/index.'));
        if (entry) {
          manifest['web_src/js/index.ts'] = {
            file: entry.fileName,
            name: 'index',
            isEntry: true,
          };
          delete manifest['web_src/js/webcomponents/index.ts'];
          writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
          break;
        }
      }
    },
  };
}

// In 'reduced' mode, exclude node_modules from sourcemaps
function reducedSourcemapPlugin(runCloseBundle = false): Plugin {
  return {
    name: 'reduced-sourcemap',
    enforce: 'post',
    transform(code, id) {
      if (id.includes('node_modules')) {
        return {code, map: {mappings: ''}};
      }
      return null;
    },
    // Delete map files with no own code, strip node_modules sourcesContent from the rest.
    // Rolldown ignores generateBundle mutations, so we must rewrite files in closeBundle.
    ...(runCloseBundle && {closeBundle() {
      for (const file of globSync('**/*.map', {cwd: outDir})) {
        const mapPath = join(outDir, file);
        const map = JSON.parse(readFileSync(mapPath, 'utf8'));
        const hasOwnCode = map.sources?.some((s: string) => !s.includes('node_modules'));
        if (!hasOwnCode) {
          unlinkSync(mapPath);
          continue;
        }
        if (!map.sourcesContent?.length) continue;
        map.sourcesContent = map.sourcesContent.map((content: string, i: number) =>
          map.sources[i]?.includes('node_modules') ? '' : content,
        );
        writeFileSync(mapPath, JSON.stringify(map));
      }
    }}),
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
        'index-domready': fileURLToPath(new URL('web_src/js/index-domready.ts', import.meta.url)),
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
    plugins: () => [
      sourceMaps === 'reduced' && reducedSourcemapPlugin(),
    ],
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
    iifeIndexPlugin(),
    filterCssUrlPlugin(),
    stringPlugin(),
    sourceMaps === 'reduced' && reducedSourcemapPlugin(true),
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
}));
