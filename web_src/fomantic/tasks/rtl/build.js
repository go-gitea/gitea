/*******************************
 *         Build Task
 *******************************/

var
  gulp = require('gulp')
;

// RTL builds are now handled by the default build process
module.exports = function (callback) {
  gulp.series(require('../build'))(callback);
};
