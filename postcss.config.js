const importer = require('postcss-import')
// const importer = require('postcss-easy-import')
const customProperties = require('postcss-custom-properties')
const colorFunctions = require('postcss-color-function')
const fontMagician = require('postcss-font-magician')
const nesting = require('postcss-nesting')
const nested = require('postcss-nested')
const cssnext = require('postcss-cssnext')
// const browserReporter = require('postcss-browser-reporter')
// const reporter = require('postcss-reporter')

function postcssConfig (webpack) {
  return [
    importer({
      path: [
        './public',
        './node_modules'
      ],
      addDependencyTo: webpack
    }),
    customProperties,
    colorFunctions,
    fontMagician,
    nesting,
    nested,
    cssnext
  ]
}

module.exports = postcssConfig
