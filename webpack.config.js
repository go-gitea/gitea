const fastGlob = require('fast-glob');
const wrapAnsi = require('wrap-ansi');
const AddAssetPlugin = require('add-asset-webpack-plugin');
const CssMinimizerPlugin = require('css-minimizer-webpack-plugin');
const FixStyleOnlyEntriesPlugin = require('webpack-fix-style-only-entries');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const MonacoWebpackPlugin = require('monaco-editor-webpack-plugin');
const TerserPlugin = require('terser-webpack-plugin');
const VueLoaderPlugin = require('vue-loader/lib/plugin');
const {statSync} = require('fs');
const {resolve, parse} = require('path');
const {LicenseWebpackPlugin} = require('license-webpack-plugin');
const {SourceMapDevToolPlugin} = require('webpack');

const glob = (pattern) => fastGlob.sync(pattern, {cwd: __dirname, absolute: true});

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

  if (cssFile.includes('font-awesome')) {
    if (/(eot|ttf|otf|woff|svg)$/.test(importedFile)) return false;
  }

  return true;
};

module.exports = {
  mode: isProduction ? 'production' : 'development',
  entry: {
    index: [
      resolve(__dirname, 'web_src/js/jquery.js'),
      resolve(__dirname, 'web_src/fomantic/build/semantic.js'),
      resolve(__dirname, 'web_src/js/index.js'),
      resolve(__dirname, 'web_src/fomantic/build/semantic.css'),
      resolve(__dirname, 'web_src/less/index.less'),
    ],
    swagger: [
      resolve(__dirname, 'web_src/js/standalone/swagger.js'),
      resolve(__dirname, 'web_src/less/standalone/swagger.less'),
    ],
    serviceworker: [
      resolve(__dirname, 'web_src/js/serviceworker.js'),
    ],
    'eventsource.sharedworker': [
      resolve(__dirname, 'web_src/js/features/eventsource.sharedworker.js'),
    ],
    'easymde': [
      resolve(__dirname, 'web_src/js/easymde.js'),
      resolve(__dirname, 'node_modules/easymde/dist/easymde.min.css'),
    ],
    ...themes,
  },
  devtool: false,
  output: {
    path: resolve(__dirname, 'public'),
    filename: ({chunk}) => {
      // serviceworker can only manage assets below it's script's directory so
      // we have to put it in / instead of /js/
      return chunk.name === 'serviceworker' ? '[name].js' : 'js/[name].js';
    },
    chunkFilename: 'js/[name].js',
  },
  optimization: {
    minimize: isProduction,
    minimizer: [
      new TerserPlugin({
        sourceMap: true,
        extractComments: false,
        terserOptions: {
          output: {
            comments: false,
          },
        },
      }),
      new CssMinimizerPlugin({
        sourceMap: true,
        minimizerOptions: {
          preset: [
            'default',
            {
              discardComments: {
                removeAll: true,
              },
            },
          ],
        },
      }),
    ],
    splitChunks: {
      chunks: 'async',
      name: (_, chunks) => chunks.map((item) => item.name).join('-'),
    },
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
            loader: 'babel-loader',
            options: {
              sourceMaps: true,
              cacheDirectory: true,
              cacheCompression: false,
              cacheIdentifier: [
                resolve(__dirname, 'package.json'),
                resolve(__dirname, 'package-lock.json'),
                resolve(__dirname, 'webpack.config.js'),
              ].map((path) => statSync(path).mtime.getTime()).join(':'),
              presets: [
                [
                  '@babel/preset-env',
                  {
                    useBuiltIns: 'usage',
                    corejs: 3,
                  },
                ],
              ],
              plugins: [
                [
                  '@babel/plugin-transform-runtime',
                  {
                    regenerator: true,
                  }
                ],
              ],
              generatorOpts: {
                compact: false,
              },
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
              url: filterCssImport,
              import: filterCssImport,
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
              url: filterCssImport,
              import: filterCssImport,
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
        include: resolve(__dirname, 'public/img/svg'),
        use: [
          {
            loader: 'raw-loader',
          },
        ],
      },
      {
        test: /\.(ttf|woff2?)$/,
        use: [
          {
            loader: 'file-loader',
            options: {
              name: '[name].[ext]',
              outputPath: 'fonts/',
              publicPath: (url) => `../fonts/${url}`, // required to remove css/ path segment
            },
          },
        ],
      },
      {
        test: /\.png$/i,
        use: [
          {
            loader: 'file-loader',
            options: {
              name: '[name].[ext]',
              outputPath: 'img/webpack/',
              publicPath: (url) => `../img/webpack/${url}`, // required to remove css/ path segment
            },
          },
        ],
      },
    ],
  },
  plugins: [
    new VueLoaderPlugin(),
    // avoid generating useless js output files for css--only chunks
    new FixStyleOnlyEntriesPlugin({
      extensions: ['less', 'scss', 'css'],
      silent: true,
    }),
    new MiniCssExtractPlugin({
      filename: 'css/[name].css',
      chunkFilename: 'css/[name].css',
    }),
    new SourceMapDevToolPlugin({
      filename: '[file].map',
      include: [
        'js/index.js',
        'css/index.css',
      ],
    }),
    new MonacoWebpackPlugin({
      filename: 'js/monaco-[name].worker.js',
    }),
    isProduction ? new LicenseWebpackPlugin({
      outputFilename: 'js/licenses.txt',
      perChunkOutput: false,
      addBanner: false,
      skipChildCompilers: true,
      modulesDirectories: [
        resolve(__dirname, 'node_modules'),
      ],
      additionalModules: [
        '@primer/octicons',
      ].map((name) => ({name, directory: resolve(__dirname, `node_modules/${name}`)})),
      renderLicenses: (modules) => {
        const line = '-'.repeat(80);
        return modules.map((module) => {
          const {name, version} = module.packageJson;
          const {licenseId, licenseText} = module;
          const body = wrapAnsi(licenseText || '', 80);
          return `${line}\n${name}@${version} - ${licenseId}\n${line}\n${body}`;
        }).join('\n');
      },
      stats: {
        warnings: false,
        errors: true,
      },
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
    children: false,
    excludeAssets: [
      // exclude monaco's language chunks in stats output for brevity
      // https://github.com/microsoft/monaco-editor-webpack-plugin/issues/113
      /^js\/[0-9]+\.js$/,
    ],
  },
};
