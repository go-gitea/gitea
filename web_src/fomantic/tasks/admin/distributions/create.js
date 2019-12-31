/*******************************
     Create Distributions
*******************************/

/*
 This will create individual distribution repositories for each SUI distribution

  * copy distribution files to release
  * update package.json file
*/

var
  gulp            = require('gulp'),

  // node dependencies
  fs              = require('fs'),
  path            = require('path'),
  mergeStream     = require('merge-stream'),

  // admin dependencies
  flatten         = require('gulp-flatten'),
  jsonEditor      = require('gulp-json-editor'),
  plumber         = require('gulp-plumber'),
  rename          = require('gulp-rename'),
  replace         = require('gulp-replace'),

  // config
  config          = require('../../config/user'),
  release         = require('../../config/admin/release'),
  project         = require('../../config/project/release'),

  // shorthand
  version         = project.version,
  output          = config.paths.output

;


module.exports = function(callback) {
  var
    stream,
    index,
    tasks = []
  ;

  for(index in release.distributions) {

    var
      distribution = release.distributions[index]
    ;

    // streams... designed to save time and make coding fun...
    (function(distribution) {

      var
        distLowerCase   = distribution.toLowerCase(),
        outputDirectory = path.join(release.outputRoot, distLowerCase),
        packageFile     = path.join(outputDirectory, release.files.npm),
        repoName        = release.distRepoRoot + distribution,
        regExp          = {
          match : {
            files   : '{files}',
            version : '{version}'
          }
        },
        task = {
          all     : distribution + ' copying files',
          repo    : distribution + ' create repo',
          meteor  : distribution + ' create meteor package.js',
          package : distribution + ' create package.json'
        },
        gatherFiles,
        createList
      ;

      // get files for meteor
      gatherFiles = function(dir) {
        var
          dir   = dir || path.resolve('.'),
          list  = fs.readdirSync(dir),
          omitted = [
            '.git',
            'node_modules',
            'package.js',
            'LICENSE',
            'README.md',
            'package.json',
            'bower.json',
            '.gitignore'
          ],
          files = []
        ;
        list.forEach(function(file) {
          var
            isOmitted = (omitted.indexOf(file) > -1),
            filePath  = path.join(dir, file),
            stat      = fs.statSync(filePath)
          ;
          if(!isOmitted) {
            if(stat && stat.isDirectory()) {
              files = files.concat(gatherFiles(filePath));
            }
            else {
              files.push(filePath.replace(outputDirectory + path.sep, ''));
            }
          }
        });
        return files;
      };

      // spaces out list correctly
      createList = function(files) {
        var filenames = '';
        for(var file in files) {
          if(file == (files.length - 1) ) {
            filenames += "'" + files[file] + "'";
          }
          else {
            filenames += "'" + files[file] + "',\n    ";
          }
        }
        return filenames;
      };

      tasks.push(function() {
          var
              files     = gatherFiles(outputDirectory),
              filenames = createList(files)
          ;
          gulp.src(release.templates.meteor[distLowerCase])
              .pipe(plumber())
              .pipe(flatten())
              .pipe(replace(regExp.match.version, version))
              .pipe(replace(regExp.match.files, filenames))
              .pipe(rename(release.files.meteor))
              .pipe(gulp.dest(outputDirectory))
          ;
      });

      if(distribution == 'CSS') {
        tasks.push(function() {
          var
            themes,
            components,
            releases
          ;
          themes = gulp.src('dist/themes/default/**/*', { base: 'dist/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          components = gulp.src('dist/components/*', { base: 'dist/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          releases = gulp.src('dist/*', { base: 'dist/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          return mergeStream(themes, components, releases);
        });
      }
      else if(distribution == 'LESS') {
        tasks.push(function() {
          var
            definitions,
            themeImport,
            themeConfig,
            siteTheme,
            themes
          ;
          definitions = gulp.src('src/definitions/**/*', { base: 'src/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          themeImport = gulp.src('src/semantic.less', { base: 'src/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          themeImport = gulp.src('src/theme.less', { base: 'src/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          themeConfig = gulp.src('src/theme.config.example', { base: 'src/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          siteTheme = gulp.src('src/_site/**/*', { base: 'src/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          themes = gulp.src('src/themes/**/*', { base: 'src/' })
            .pipe(gulp.dest(outputDirectory))
          ;
          return mergeStream(definitions, themeImport, themeConfig, siteTheme, themes);
        });
      }

      // extend package.json
      tasks.push(function() {
        return gulp.src(packageFile)
          .pipe(plumber())
          .pipe(jsonEditor(function(package) {
            if(version) {
              package.version = version;
            }
            return package;
          }))
          .pipe(gulp.dest(outputDirectory))
        ;
      });

    })(distribution);
  }

  gulp.series(...tasks)(callback);
};