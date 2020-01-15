const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');
const { SourceMapDevToolPlugin } = require('webpack');

module.exports = {
  mode: 'production',
  entry: {
    index: ['./web_src/js/index'],
    swagger: ['./web_src/js/swagger'],
  },
  devtool: false,
  output: {
    path: path.resolve(__dirname, 'public/js'),
    filename: '[name].js',
    chunkFilename: '[name].js',
  },
  optimization: {
    minimize: true,
    minimizer: [new TerserPlugin({
      sourceMap: true,
      extractComments: false,
      terserOptions: {
        output: {
          comments: false,
        },
      },
    })],
  },
  module: {
    rules: [
      {
        test: /\.js$/,
        exclude: /node_modules/,
        use: {
          loader: 'babel-loader',
          options: {
            presets: [
              [
                '@babel/preset-env',
                {
                  useBuiltIns: 'usage',
                  corejs: 3,
                }
              ]
            ],
            plugins: [
              [
                '@babel/plugin-transform-runtime',
                {
                  regenerator: true,
                }
              ]
            ],
          }
        }
      },
      {
        test: /\.css$/i,
        use: ['style-loader', 'css-loader'],
      },
    ]
  },
  plugins: [
    new SourceMapDevToolPlugin({
      filename: '[name].js.map',
      exclude: [
        'swagger.js',
      ],
    }),
  ],
  performance: {
    assetFilter: (filename) => {
      return !filename.endsWith('.map') && filename !== 'swagger.js';
    }
  },
};
