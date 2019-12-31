/*******************************
 *         Build Task
 *******************************/

var
  // dependencies
  gulp     = require('gulp'),

  // config
  install  = require('./config/project/install')
;

module.exports = function (callback) {

  console.info('Building Semantic');

  if (!install.isSetup()) {
    console.error('Cannot find semantic.json. Run "gulp install" to set-up Semantic');
    return 1;
  }

  gulp.series('build-css', 'build-javascript', 'build-assets')(callback);
};
