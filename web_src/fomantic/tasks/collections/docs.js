/*******************************
 *     Define Docs Sub-Tasks
 *******************************/

/*
  Lets you serve files to a local documentation instance
  https://github.com/Semantic-Org/Semantic-UI-Docs/
*/
module.exports = function (gulp) {

  var
    // docs tasks
    serveDocs = require('./../docs/serve'),
    buildDocs = require('./../docs/build')
  ;

  gulp.task('serve-docs', serveDocs);
  gulp.task('serve-docs').description = 'Serve file changes to SUI Docs';

  gulp.task('build-docs', buildDocs);
  gulp.task('build-docs').description = 'Build all files and add to SUI Docs';

};
