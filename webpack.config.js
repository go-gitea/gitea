const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');
const { SourceMapDevToolPlugin } = require('webpack');
const VueLoaderPlugin = require('vue-loader/lib/plugin');
const cssnano = require('cssnano');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const OptimizeCSSAssetsPlugin = require('optimize-css-assets-webpack-plugin');
const PostCSSSafeParser = require('postcss-safe-parser');
const PostCSSPresetEnv = require('postcss-preset-env');

module.exports = {
  mode: 'production',
  entry: {
    index: ['./web_src/js/index'],
    swagger: ['./web_src/js/swagger'],
    jquery: ['./web_src/js/jquery'],
  },
  devtool: false,
  output: {
    path: path.resolve(__dirname, 'public'),
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
        test: /\.css$/i,
        use: [
          {
            loader: MiniCssExtractPlugin.loader,
          },
          {
            loader: 'css-loader',
            options: {
              importLoaders: 1,
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
        ],
      },
    ],
  },
  plugins: [
    new VueLoaderPlugin(),
    new MiniCssExtractPlugin({
      filename: 'css/[name].css',
      chunkFilename: 'css/[name].css',
    }),
    new SourceMapDevToolPlugin({
      filename: 'js/[name].js.map',
      exclude: [
        'js/gitgraph.js',
        'js/jquery.js',
        'js/swagger.js',
      ],
    }),
  ],
  performance: {
    maxEntrypointSize: 512000,
    maxAssetSize: 512000,
    assetFilter: (filename) => {
      return !filename.endsWith('.map') && filename !== 'js/swagger.js';
    },
  },
  resolve: {
    symlinks: false,
  },
};
