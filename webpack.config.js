const ClosureCompilerPlugin = require('webpack-closure-compiler')
const ExtractTextPlugin = require('extract-text-webpack-plugin')
const webpack = require('webpack')
const postcssConfig = require('./postcss.config')
module.exports = {
  entry: {
    index: './public/js/index',
    clipboard: './public/js/clipboard',
    datetimepicker: './public/js/datetimepicker',
    dropzone: './public/js/dropzone',
    emojify: './public/js/emojify',
    simpleMDE: './public/js/simpleMDE',
    minicolors: './public/js/minicolors'
  },
  output: {
    filename: '[name].js',
    chunkFilename: '[id].js',
    path: __dirname + '/public/assets'
  },
  module: {
    loaders: [
      {
        test: /\.js$/,
        exclude: /node_modules/,
        loader: ['babel-loader?cacheDirectory']
      },
      {
        test: /\.css$/,
        exclude: /node_modules/,
        loader: ExtractTextPlugin.extract({
          fallbackLoader: 'style-loader',
          loader: 'css-loader?modules&importLoaders=1!postcss-loader'
        })
      },
      {
        test: /\.less$/,
        loader: ExtractTextPlugin.extract({
          fallbackLoader: 'style-loader',
          loader: 'css-loader?modules&importLoaders=1!less-loader!postcss-loader'
        })
      }
    ]
  },
  // postcss: postcssConfig,
  plugins: [
    new ExtractTextPlugin(__dirname + '/public/assets/[name].css'),
    new webpack.ProvidePlugin({
      '$': 'jquery',
      'jQuery': 'jquery',
      'window.jQuery': 'jquery'
    }),
    new ClosureCompilerPlugin({
      compiler: {
        language_in: 'ECMASCRIPT6',
        language_out: 'ECMASCRIPT5',
        compilation_level: 'ADVANCED'
      },
      concurrency: 3
    }),
    new webpack.optimize.AggressiveMergingPlugin(),
    new webpack.optimize.OccurrenceOrderPlugin() /*,
    new webpack.NoErrorsPlugin()*/
  ]
}
