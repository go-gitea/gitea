const fastGlob = require('fast-glob');
const wrapAnsi = require('wrap-ansi');
const CssNanoPlugin = require('cssnano-webpack-plugin');
const FixStyleOnlyEntriesPlugin = require('webpack-fix-style-only-entries');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const MonacoWebpackPlugin = require('monaco-editor-webpack-plugin');
const PostCSSPresetEnv = require('postcss-preset-env');
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

const filterCssImport = (parsedImport, cssFile) => {
  const url = parsedImport && parsedImport.url ? parsedImport.url : parsedImport;
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
    ],
    serviceworker: [
      resolve(__dirname, 'web_src/js/serviceworker.js'),
    ],
    'eventsource.sharedworker': [
      resolve(__dirname, 'web_src/js/features/eventsource.sharedworker.js'),
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
      new CssNanoPlugin({
        sourceMap: true,
        cssnanoOptions: {
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
      cacheGroups: {
        // this bundles all monaco's languages into one file instead of emitting 1-65.js files
        monaco: {
          test: /monaco-editor/,
          name: 'monaco',
          chunks: 'async',
        },
      },
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
              name: '[name].js',
              inline: true,
              fallback: false,
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
              cacheDirectory: true,
              cacheCompression: false,
              cacheIdentifier: [
                resolve(__dirname, 'package.json'),
                resolve(__dirname, 'package-lock.json'),
                resolve(__dirname, 'webpack.config.js'),
              ].map((path) => statSync(path).mtime.getTime()).join(':'),
              sourceMaps: true,
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
                '@babel/plugin-proposal-object-rest-spread',
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
              importLoaders: 1,
              url: filterCssImport,
              import: filterCssImport,
              sourceMap: true,
            },
          },
          {
            loader: 'postcss-loader',
            options: {
              plugins: () => [
                PostCSSPresetEnv(),
              ],
              sourceMap: true,
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
              importLoaders: 2,
              url: filterCssImport,
              import: filterCssImport,
              sourceMap: true,
            },
          },
          {
            loader: 'postcss-loader',
            options: {
              plugins: () => [
                PostCSSPresetEnv(),
              ],
              sourceMap: true,
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
              publicPath: (url) => `../fonts/${url}`, // seems required for monaco's font
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
    new LicenseWebpackPlugin({
      outputFilename: 'js/licenses.txt',
      perChunkOutput: false,
      addBanner: false,
      skipChildCompilers: true,
      modulesDirectories: [
        resolve(__dirname, 'node_modules'),
      ],
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
    }),
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
  },
};
