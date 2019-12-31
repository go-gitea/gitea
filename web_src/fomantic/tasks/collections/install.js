/*******************************
 *   Define Install Sub-Tasks
 *******************************/

/*
  Lets you serve files to a local documentation instance
  https://github.com/Semantic-Org/Semantic-UI-Docs/
*/
module.exports = function (gulp) {

  var
    // docs tasks
    install      = require('./../install'),
    checkInstall = require('./../check-install')
  ;

  gulp.task('install', install);
  gulp.task('install').description = 'Runs set-up';

  gulp.task('check-install', checkInstall);
  gulp.task('check-install').description = 'Displays current version of Semantic';

};
