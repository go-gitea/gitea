/*******************************
 Serve Docs
 *******************************/
var
  gulp        = require('gulp'),

  // node dependencies
  console     = require('better-console'),

  // gulp dependencies
  print       = require('gulp-print').default,

  // user config
  config      = require('../config/docs'),

  // task config
  tasks       = require('../config/tasks'),
  configSetup = require('../config/project/config'),

  // shorthand
  log         = tasks.log,

  css         = require('../build/css'),
  js          = require('../build/javascript'),
  assets      = require('../build/assets')
;


module.exports = function () {

  // use a different config
  config = configSetup.addDerivedValues(config);

  console.clear();
  console.log('Watching source files for changes');

  /*--------------
     Copy Source
  ---------------*/

  gulp
    .watch(['src/**/*.*'])
    .on('all', function (event, path) {
      // We don't handle deleted files yet
      if (event === 'unlink' || event === 'unlinkDir') {
        return;
      }
      return gulp.src(path, {
        base: 'src/'
      })
        .pipe(gulp.dest(config.paths.output.less))
        .pipe(print(log.created))
        ;
    })
  ;

  /*--------------
    Copy Examples
  ---------------*/

  gulp
    .watch(['examples/**/*.*'])
    .on('all', function (event, path) {
      // We don't handle deleted files yet
      if (event === 'unlink' || event === 'unlinkDir') {
        return;
      }
      return gulp.src(path, {
        base: 'examples/'
      })
        .pipe(gulp.dest(config.paths.output.examples))
        .pipe(print(log.created))
        ;
    })
  ;

  /*--------------
      Watch CSS
  ---------------*/

  css.watch('docs', config);

  /*--------------
      Watch JS
  ---------------*/

  js.watch('docs', config);

  /*--------------
    Watch Assets
  ---------------*/

  assets.watch('docs', config);

};
