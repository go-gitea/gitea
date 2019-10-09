const path = require('path')

module.exports = {
    entry: path.join(__dirname, './web_src/js/index.js'),
    output: {
        path: path.join(__dirname, "./public/js"),
        filename: 'index.js'
    },
  };
  