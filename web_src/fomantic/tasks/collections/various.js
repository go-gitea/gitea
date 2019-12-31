/*******************************
 *   Define Various Sub-Tasks
 *******************************/

/*
  Lets you serve files to a local documentation instance
  https://github.com/Semantic-Org/Semantic-UI-Docs/
*/
module.exports = function (gulp) {

  var
    clean   = require('./../clean'),
    version = require('./../version')
  ;

  gulp.task('clean', clean);
  gulp.task('clean').description = 'Clean dist folder';

  gulp.task('version', version);
  gulp.task('version').description = 'Clean dist folder';

};
