import fastGlob from 'fast-glob';
import wrapAnsi from 'wrap-ansi';
import AddAssetPlugin from 'add-asset-webpack-plugin';
import LicenseCheckerWebpackPlugin from 'license-checker-webpack-plugin';
import MiniCssExtractPlugin from 'mini-css-extract-plugin';
import MonacoWebpackPlugin from 'monaco-editor-webpack-plugin';
import VueLoader from 'vue-loader';
import EsBuildLoader from 'esbuild-loader';
import {parse, dirname} from 'path';
import webpack from 'webpack';
import {fileURLToPath} from 'url';

const {VueLoaderPlugin} = VueLoader;
const {ESBuildMinifyPlugin} = EsBuildLoader;
const {SourceMapDevToolPlugin} = webpack;
const glob = (pattern) => fastGlob.sync(pattern, {
  cwd: dirname(fileURLToPath(new URL(import.meta.url))),
  absolute: true,
});

const themes = {};
for (const path of glob('web_src/less/themes/*.less')) {
  themes[parse(path).name] = [path];
}

const isProduction = process.env.NODE_ENV !== 'development';

const filterCssImport = (url, ...args) => {
  const cssFile = args[1] || args[0]; // resourcePath is 2nd argument for url and 3rd for import
  const importedFile = url.replace(/[?#].+/, '').toLowerCase();

  if (cssFile.includes('fomantic')) {
    if (/brand-icons/.test(importedFile)) return false;
    if (/(eot|ttf|otf|woff|svg)$/.test(importedFile)) return false;
  }

  if (cssFile.includes('font-awesome') && /(eot|ttf|otf|woff|svg)$/.test(importedFile)) {
    return false;
  }

  return true;
};

export default {
  mode: isProduction ? 'production' : 'development',
  entry: {
    index: [
      fileURLToPath(new URL('web_src/js/jquery.js', import.meta.url)),
      fileURLToPath(new URL('web_src/fomantic/build/semantic.js', import.meta.url)),
      fileURLToPath(new URL('web_src/js/index.js', import.meta.url)),
      fileURLToPath(new URL('node_modules/easymde/dist/easymde.min.css', import.meta.url)),
      fileURLToPath(new URL('web_src/fomantic/build/semantic.css', import.meta.url)),
      fileURLToPath(new URL('web_src/less/misc.css', import.meta.url)),
      fileURLToPath(new URL('web_src/less/index.less', import.meta.url)),
    ],
    swagger: [
      fileURLToPath(new URL('web_src/js/standalone/swagger.js', import.meta.url)),
      fileURLToPath(new URL('web_src/less/standalone/swagger.less', import.meta.url)),
    ],
    serviceworker: [
      fileURLToPath(new URL('web_src/js/serviceworker.js', import.meta.url)),
    ],
    'eventsource.sharedworker': [
      fileURLToPath(new URL('web_src/js/features/eventsource.sharedworker.js', import.meta.url)),
    ],
    ...themes,
  },
  devtool: false,
  output: {
    path: fileURLToPath(new URL('public', import.meta.url)),
    filename: ({chunk}) => {
      // serviceworker can only manage assets below it's script's directory so
      // we have to put it in / instead of /js/
      return chunk.name === 'serviceworker' ? '[name].js' : 'js/[name].js';
    },
    chunkFilename: ({chunk}) => {
      const language = (/monaco.*languages?_.+?_(.+?)_/.exec(chunk.id) || [])[1];
      return `js/${language ? `monaco-language-${language.toLowerCase()}` : `[name]`}.[contenthash:8].js`;
    },
  },
  optimization: {
    minimize: isProduction,
    minimizer: [
      new ESBuildMinifyPlugin({
        target: 'es2015',
        minify: true,
        css: true,
        legalComments: 'none',
      }),
    ],
    splitChunks: {
      chunks: 'async',
      name: (_, chunks) => chunks.map((item) => item.name).join('-'),
    },
    moduleIds: 'named',
    chunkIds: 'named',
  },
  module: {
    rules: [
      {
        test: /\.vue$/,
        exclude: /node_modules/,
        loader: 'vue-loader',
      },
      {
        test: /\.worker\.js$/,
        exclude: /monaco/,
        use: [
          {
            loader: 'worker-loader',
            options: {
              inline: 'no-fallback',
            },
          },
        ],
      },
      {
        test: /\.js$/,
        exclude: /node_modules/,
        use: [
          {
            loader: 'esbuild-loader',
            options: {
              target: 'es2015'
            },
          },
        ],
      },
      {
        test: /.css$/i,
        use: [
          {
            loader: MiniCssExtractPlugin.loader,
          },
          {
            loader: 'css-loader',
            options: {
              sourceMap: true,
              url: {filter: filterCssImport},
              import: {filter: filterCssImport},
            },
          },
        ],
      },
      {
        test: /.less$/i,
        use: [
          {
            loader: MiniCssExtractPlugin.loader,
          },
          {
            loader: 'css-loader',
            options: {
              sourceMap: true,
              importLoaders: 1,
              url: {filter: filterCssImport},
              import: {filter: filterCssImport},
            },
          },
          {
            loader: 'less-loader',
            options: {
              sourceMap: true,
            },
          },
        ],
      },
      {
        test: /\.svg$/,
        include: fileURLToPath(new URL('public/img/svg', import.meta.url)),
        type: 'asset/source',
      },
      {
        test: /\.(ttf|woff2?)$/,
        type: 'asset/resource',
        generator: {
          filename: 'fonts/[name].[contenthash:8][ext]',
        }
      },
      {
        test: /\.png$/i,
        type: 'asset/resource',
        generator: {
          filename: 'img/webpack/[name].[contenthash:8][ext]',
        }
      },
    ],
  },
  plugins: [
    new VueLoaderPlugin(),
    new MiniCssExtractPlugin({
      filename: 'css/[name].css',
      chunkFilename: 'css/[name].[contenthash:8].css',
    }),
    new SourceMapDevToolPlugin({
      filename: '[file].[contenthash:8].map',
      include: [
        'js/index.js',
        'css/index.css',
      ],
    }),
    new MonacoWebpackPlugin({
      filename: 'js/monaco-[name].[contenthash:8].worker.js',
    }),
    isProduction ? new LicenseCheckerWebpackPlugin({
      outputFilename: 'js/licenses.txt',
      outputWriter: ({dependencies}) => {
        const line = '-'.repeat(80);
        return dependencies.map((module) => {
          const {name, version, licenseName, licenseText} = module;
          const body = wrapAnsi(licenseText || '', 80);
          return `${line}\n${name}@${version} - ${licenseName}\n${line}\n${body}`;
        }).join('\n');
      },
      override: {
        'jquery.are-you-sure@*': {licenseName: 'MIT'},
      },
      allow: '(Apache-2.0 OR BSD-2-Clause OR BSD-3-Clause OR MIT OR ISC)',
      ignore: [
        'font-awesome',
      ],
    }) : new AddAssetPlugin('js/licenses.txt', `Licenses are disabled during development`),
  ],
  performance: {
    hints: false,
    maxEntrypointSize: Infinity,
    maxAssetSize: Infinity,
  },
  resolve: {
    symlinks: false,
    alias: {
      vue$: 'vue/dist/vue.esm.js', // needed because vue's default export is the runtime only
    },
  },
  watchOptions: {
    ignored: [
      'node_modules/**',
    ],
  },
  stats: {
    assetsSort: 'name',
    assetsSpace: Infinity,
    cached: false,
    cachedModules: false,
    children: false,
    chunkModules: false,
    chunkOrigins: false,
    chunksSort: 'name',
    colors: true,
    entrypoints: false,
    excludeAssets: [
      /^js\/monaco-language-.+\.js$/,
      !isProduction && /^js\/licenses.txt$/,
    ].filter((item) => !!item),
    groupAssetsByChunk: false,
    groupAssetsByEmitStatus: false,
    groupAssetsByInfo: false,
    groupModulesByAttributes: false,
    modules: false,
    reasons: false,
    runtimeModules: false,
  },
};
