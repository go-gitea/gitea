const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');

module.exports = {
  mode: 'production',
  entry: {
    index: './web_src/js/index.js',
  },
  devtool: 'source-map',
  output: {
    path: path.resolve(__dirname, 'public/js'),
    filename: "[name].js"
  },
  optimization: {
    minimize: true,
    minimizer: [new TerserPlugin({ 
        sourceMap: true
     })],
  },
};
