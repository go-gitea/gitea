/*******************************
 *          Build Task
 *******************************/

const
  gulp         = require('gulp'),

  // node dependencies
  console      = require('better-console'),

  // gulp dependencies
  autoprefixer = require('gulp-autoprefixer'),
  chmod        = require('gulp-chmod'),
  concatCSS    = require('gulp-concat-css'),
  dedupe       = require('gulp-dedupe'),
  flatten      = require('gulp-flatten'),
  gulpif       = require('gulp-if'),
  header       = require('gulp-header'),
  less         = require('gulp-less'),
  minifyCSS    = require('gulp-clean-css'),
  normalize    = require('normalize-path'),
  plumber      = require('gulp-plumber'),
  print        = require('gulp-print').default,
  rename       = require('gulp-rename'),
  replace      = require('gulp-replace'),
  replaceExt   = require('replace-ext'),
  rtlcss       = require('gulp-rtlcss'),

  // config
  config       = require('./../config/user'),
  docsConfig   = require('./../config/docs'),
  tasks        = require('../config/tasks'),
  install      = require('../config/project/install'),

  // shorthand
  globs        = config.globs,
  assets       = config.paths.assets,

  banner       = tasks.banner,
  filenames    = tasks.filenames,
  comments     = tasks.regExp.comments,
  log          = tasks.log,
  settings     = tasks.settings
;

/**
 * Builds the css
 * @param src
 * @param type
 * @param compress
 * @param config
 * @param opts
 * @return {*}
 */
function build(src, type, compress, config, opts) {
  let fileExtension;
  if (type === 'rtl' && compress) {
    fileExtension = settings.rename.rtlMinCSS;
  } else if (type === 'rtl') {
    fileExtension = settings.rename.rtlCSS;
  } else if (compress) {
    fileExtension = settings.rename.minCSS;
  }

  return gulp.src(src, opts)
    .pipe(plumber(settings.plumber.less))
    .pipe(less(settings.less))
    .pipe(autoprefixer(settings.prefix))
    .pipe(gulpif(type === 'rtl', rtlcss()))
    .pipe(replace(comments.variables.in, comments.variables.out))
    .pipe(replace(comments.license.in, comments.license.out))
    .pipe(replace(comments.large.in, comments.large.out))
    .pipe(replace(comments.small.in, comments.small.out))
    .pipe(replace(comments.tiny.in, comments.tiny.out))
    .pipe(flatten())
    .pipe(replace(config.paths.assets.source,
      compress ? config.paths.assets.compressed : config.paths.assets.uncompressed))
    .pipe(gulpif(compress, minifyCSS(settings.minify)))
    .pipe(gulpif(fileExtension, rename(fileExtension)))
    .pipe(gulpif(config.hasPermissions, chmod(config.parsedPermissions)))
    .pipe(gulp.dest(compress ? config.paths.output.compressed : config.paths.output.uncompressed))
    .pipe(print(log.created))
    ;
}

/**
 * Packages the css files in dist
 * @param {string} type - type of the css processing (none, rtl, docs)
 * @param {boolean} compress - should the output be compressed
 */
function pack(type, compress) {
  const output       = type === 'docs' ? docsConfig.paths.output : config.paths.output;
  const ignoredGlobs = type === 'rtl' ? globs.ignoredRTL + '.rtl.css' : globs.ignored + '.css';

  let concatenatedCSS;
  if (type === 'rtl') {
    concatenatedCSS = compress ? filenames.concatenatedMinifiedRTLCSS : filenames.concatenatedRTLCSS;
  } else {
    concatenatedCSS = compress ? filenames.concatenatedMinifiedCSS : filenames.concatenatedCSS;
  }

  return gulp.src(output.uncompressed + '/**/' + globs.components + ignoredGlobs)
    .pipe(plumber())
    .pipe(dedupe())
    .pipe(replace(assets.uncompressed, assets.packaged))
    .pipe(concatCSS(concatenatedCSS, settings.concatCSS))
    .pipe(gulpif(config.hasPermissions, chmod(config.parsedPermissions)))
    .pipe(gulpif(compress, minifyCSS(settings.concatMinify)))
    .pipe(header(banner, settings.header))
    .pipe(gulp.dest(output.packaged))
    .pipe(print(log.created))
    ;
}

function buildCSS(src, type, config, opts, callback) {
  if (!install.isSetup()) {
    console.error('Cannot build CSS files. Run "gulp install" to set-up Semantic');
    callback();
    return;
  }

  if (callback === undefined) {
    callback = opts;
    opts     = config;
    config   = type;
    type     = src;
    src      = config.paths.source.definitions + '/**/' + config.globs.components + '.less';
  }

  const buildUncompressed       = () => build(src, type, false, config, opts);
  buildUncompressed.displayName = 'Building uncompressed CSS';

  const buildCompressed       = () => build(src, type, true, config, opts);
  buildCompressed.displayName = 'Building compressed CSS';

  const packUncompressed       = () => pack(type, false);
  packUncompressed.displayName = 'Packing uncompressed CSS';

  const packCompressed       = () => pack(type, true);
  packCompressed.displayName = 'Packing compressed CSS';

  gulp.parallel(
    gulp.series(buildUncompressed, packUncompressed),
    gulp.series(buildCompressed, packCompressed)
  )(callback);
}

function rtlAndNormal(src, callback) {
  if (callback === undefined) {
    callback = src;
    src      = config.paths.source.definitions + '/**/' + config.globs.components + '.less';
  }

  const rtl       = (callback) => buildCSS(src, 'rtl', config, {}, callback);
  rtl.displayName = "CSS Right-To-Left";
  const css       = (callback) => buildCSS(src, 'default', config, {}, callback);
  css.displayName = "CSS";

  if (config.rtl === true || config.rtl === 'Yes') {
    rtl(callback);
  } else if (config.rtl === 'both') {
    gulp.series(rtl, css)(callback);
  } else {
    css(callback);
  }
}

function docs(src, callback) {
  if (callback === undefined) {
    callback = src;
    src      = config.paths.source.definitions + '/**/' + config.globs.components + '.less';
  }

  const func       = (callback) => buildCSS(src, 'docs', config, {}, callback);
  func.displayName = "CSS Docs";

  func(callback);
}

// Default tasks
module.exports = rtlAndNormal;

// We keep the changed files in an array to call build with all of them at the same time
let timeout, files = [];

/**
 * Watch changes in CSS files and call the correct build pipe
 * @param type
 * @param config
 */
module.exports.watch = function (type, config) {
  const method = type === 'docs' ? docs : rtlAndNormal;

  // Watch theme.config file
  gulp.watch([
    normalize(config.paths.source.config),
    normalize(config.paths.source.site + '/**/site.variables'),
    normalize(config.paths.source.themes + '/**/site.variables')
  ])
    .on('all', function () {
      // Clear timeout and reset files
      timeout && clearTimeout(timeout);
      files = [];
      return gulp.series(method)();
    });

  // Watch any less / overrides / variables files
  gulp.watch([
    normalize(config.paths.source.definitions + '/**/*.less'),
    normalize(config.paths.source.site + '/**/*.{overrides,variables}'),
    normalize(config.paths.source.themes + '/**/*.{overrides,variables}')
  ])
    .on('all', function (event, path) {
      // We don't handle deleted files yet
      if (event === 'unlink' || event === 'unlinkDir') {
        return;
      }

      // Clear timeout
      timeout && clearTimeout(timeout);

      // Determine which LESS file has to be recompiled
      let lessPath;
      if(path.indexOf('site.variables') !== -1)  {
        return;
      } else if (path.indexOf(config.paths.source.themes) !== -1) {
        console.log('Change detected in packaged theme');
        lessPath = replaceExt(path, '.less');
        lessPath = lessPath.replace(tasks.regExp.theme, config.paths.source.definitions);
      } else if (path.indexOf(config.paths.source.site) !== -1) {
        console.log('Change detected in site theme');
        lessPath = replaceExt(path, '.less');
        lessPath = lessPath.replace(config.paths.source.site, config.paths.source.definitions);
      } else {
        console.log('Change detected in definition');
        lessPath = path;
      }

      // Add file to internal changed files array
      if (!files.includes(lessPath)) {
        files.push(lessPath);
      }

      // Update timeout
      timeout = setTimeout(() => {
        // Copy files to build in another array
        const buildFiles = [...files];
        // Call method
        gulp.series((callback) => method(buildFiles, callback))();
        // Reset internal changed files array
        files = [];
      }, 1000);
    });
};

// Expose build css method
module.exports.buildCSS = buildCSS;