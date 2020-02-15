const cssnano = require('cssnano');
const fastGlob = require('fast-glob');
const FixStyleOnlyEntriesPlugin = require('webpack-fix-style-only-entries');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const OptimizeCSSAssetsPlugin = require('optimize-css-assets-webpack-plugin');
const PostCSSPresetEnv = require('postcss-preset-env');
const PostCSSSafeParser = require('postcss-safe-parser');
const SpriteLoaderPlugin = require('svg-sprite-loader/plugin');
const TerserPlugin = require('terser-webpack-plugin');
const VueLoaderPlugin = require('vue-loader/lib/plugin');
const { statSync } = require('fs');
const { resolve, parse } = require('path');
const { SourceMapDevToolPlugin } = require('webpack');

const glob = (pattern) => fastGlob.sync(pattern, { cwd: __dirname, absolute: true });

const themes = {};
for (const path of glob('web_src/less/themes/*.less')) {
  themes[parse(path).name] = [path];
}

module.exports = {
  mode: 'production',
  entry: {
    index: [
      resolve(__dirname, 'web_src/js/index.js'),
      resolve(__dirname, 'web_src/less/index.less'),
    ],
    swagger: [
      resolve(__dirname, 'web_src/js/standalone/swagger.js'),
    ],
    jquery: [
      resolve(__dirname, 'web_src/js/jquery.js'),
    ],
    icons: glob('node_modules/@primer/octicons/build/svg/**/*.svg'),
    ...themes,
  },
  devtool: false,
  output: {
    path: resolve(__dirname, 'public'),
    filename: 'js/[name].js',
    chunkFilename: 'js/[name].js',
  },
  optimization: {
    minimize: true,
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
      new OptimizeCSSAssetsPlugin({
        cssProcessor: cssnano,
        cssProcessorOptions: {
          parser: PostCSSSafeParser,
        },
        cssProcessorPluginOptions: {
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
    }
  },
  module: {
    rules: [
      {
        test: /\.vue$/,
        exclude: /node_modules/,
        loader: 'vue-loader',
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
            },
          },
        ],
      },
      {
        test: /\.(less|css)$/i,
        use: [
          {
            loader: MiniCssExtractPlugin.loader,
          },
          {
            loader: 'css-loader',
            options: {
              importLoaders: 2,
              url: false,
            }
          },
          {
            loader: 'postcss-loader',
            options: {
              plugins: () => [
                PostCSSPresetEnv(),
              ],
            },
          },
          {
            loader: 'less-loader',
          },
        ],
      },
      {
        test: /\.svg$/,
        use: [
          {
            loader: 'svg-sprite-loader',
            options: {
              extract: true,
              spriteFilename: 'img/svg/icons.svg',
              symbolId: (path) => {
                const { name } = parse(path);
                if (/@primer[/\\]octicons/.test(path)) {
                  return `octicon-${name}`;
                }
                return name;
              },
            },
          },
          {
            loader: 'svgo-loader',
          },
        ],
      },
    ],
  },
  plugins: [
    new VueLoaderPlugin(),
    // avoid generating useless js output files for css- and svg-only chunks
    new FixStyleOnlyEntriesPlugin({
      extensions: ['less', 'scss', 'css', 'svg'],
      silent: true,
    }),
    new MiniCssExtractPlugin({
      filename: 'css/[name].css',
      chunkFilename: 'css/[name].css',
    }),
    new SourceMapDevToolPlugin({
      filename: 'js/[name].js.map',
      include: [
        'js/index.js',
      ],
    }),
    new SpriteLoaderPlugin({
      plainSprite: true,
    }),
  ],
  performance: {
    maxEntrypointSize: 512000,
    maxAssetSize: 512000,
    assetFilter: (filename) => {
      if (filename.endsWith('.map')) return false;
      if (['js/swagger.js', 'js/highlight.js'].includes(filename)) return false;
      return true;
    },
  },
  resolve: {
    symlinks: false,
  },
};
