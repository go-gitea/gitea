/*******************************
 *          Watch Task
 *******************************/

var
  gulp = require('gulp')
;

// RTL watch are now handled by the default watch process
module.exports = function (callback) {
  gulp.series(require('../watch'))(callback);
};