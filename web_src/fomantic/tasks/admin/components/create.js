/*******************************
     Create Component Repos
*******************************/

/*
 This will create individual component repositories for each SUI component

  * copy component files from release
  * create commonjs files as index.js for NPM release
  * create release notes that filter only items related to component
  * custom package.json file from template
  * create bower.json from template
  * create README from template
  * create meteor.js file
*/

var
  gulp            = require('gulp'),

  // node dependencies
  fs              = require('fs'),
  path            = require('path'),

  // admin dependencies
  concatFileNames = require('gulp-concat-filenames'),
  flatten         = require('gulp-flatten'),
  jsonEditor      = require('gulp-json-editor'),
  plumber         = require('gulp-plumber'),
  rename          = require('gulp-rename'),
  replace         = require('gulp-replace'),
  tap             = require('gulp-tap'),

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

  for(index in release.components) {

    var
      component = release.components[index]
    ;

    // streams... designed to save time and make coding fun...
    (function(component) {

      var
        outputDirectory      = path.join(release.outputRoot, component),
        isJavascript         = fs.existsSync(output.compressed + component + '.js'),
        isCSS                = fs.existsSync(output.compressed + component + '.css'),
        capitalizedComponent = component.charAt(0).toUpperCase() + component.slice(1),
        packageName          = release.packageRoot + component,
        repoName             = release.componentRepoRoot + capitalizedComponent,
        gitURL               = 'https://github.com/' + release.org + '/' + repoName + '.git',
        repoURL              = 'https://github.com/' + release.org + '/' + repoName + '/',
        concatSettings = {
          newline : '',
          root    : outputDirectory,
          prepend : "    '",
          append  : "',"
        },
        regExp               = {
          match            : {
            // templated values
            name      : '{component}',
            titleName : '{Component}',
            version   : '{version}',
            files     : '{files}',
            // release notes
            spacedVersions    : /(###.*\n)\n+(?=###)/gm,
            spacedLists       : /(^- .*\n)\n+(?=^-)/gm,
            trim              : /^\s+|\s+$/g,
            unrelatedNotes    : new RegExp('^((?!(^.*(' + component + ').*$|###.*)).)*$', 'gmi'),
            whitespace        : /\n\s*\n\s*\n/gm,
            // npm
            componentExport   : /(.*)\$\.fn\.\w+\s*=\s*function\(([^\)]*)\)\s*{/g,
            componentReference: '$.fn.' + component,
            settingsExport    : /\$\.fn\.\w+\.settings\s*=/g,
            settingsReference : /\$\.fn\.\w+\.settings/g,
            trailingComma     : /,(?=[^,]*$)/,
            jQuery            : /jQuery/g,
          },
          replace : {
            // readme
            name              : component,
            titleName         : capitalizedComponent,
            // release notes
            spacedVersions    : '',
            spacedLists       : '$1',
            trim              : '',
            unrelatedNotes    : '',
            whitespace        : '\n\n',
            // npm
            componentExport   :  'var _module = module;\n$1module.exports = function($2) {',
            componentReference:  '_module.exports',
            settingsExport    :  'module.exports.settings =',
            settingsReference :  '_module.exports.settings',
            jQuery            :  'require("jquery")'
          }
        },
        task = {
          all      : component + ' creating',
          repo     : component + ' create repo',
          bower    : component + ' create bower.json',
          readme   : component + ' create README',
          npm      : component + ' create NPM Module',
          notes    : component + ' create release notes',
          composer : component + ' create composer.json',
          package  : component + ' create package.json',
          meteor   : component + ' create meteor package.js',
        },
        // paths to includable assets
        manifest = {
          assets    : outputDirectory + '/assets/**/' + component + '?(s).*',
          component : outputDirectory + '/' + component + '+(.js|.css)'
        }
      ;

      // copy dist files into output folder adjusting asset paths
      function copyDist() {
        return gulp.src(release.source + component + '.*')
          .pipe(plumber())
          .pipe(flatten())
          .pipe(replace(release.paths.source, release.paths.output))
          .pipe(gulp.dest(outputDirectory))
        ;
      }

      // create npm module
      function createNpmModule() {
        return gulp.src(release.source + component + '!(*.min|*.map).js')
          .pipe(plumber())
          .pipe(flatten())
          .pipe(replace(regExp.match.componentExport, regExp.replace.componentExport))
          .pipe(replace(regExp.match.componentReference, regExp.replace.componentReference))
          .pipe(replace(regExp.match.settingsExport, regExp.replace.settingsExport))
          .pipe(replace(regExp.match.settingsReference, regExp.replace.settingsReference))
          .pipe(replace(regExp.match.jQuery, regExp.replace.jQuery))
          .pipe(rename('index.js'))
          .pipe(gulp.dest(outputDirectory))
        ;
      }

      // create readme
      function createReadme() {
        return gulp.src(release.templates.readme)
          .pipe(plumber())
          .pipe(flatten())
          .pipe(replace(regExp.match.name, regExp.replace.name))
          .pipe(replace(regExp.match.titleName, regExp.replace.titleName))
          .pipe(gulp.dest(outputDirectory))
        ;
      }

      // extend bower.json
      function extendBower() {
        return gulp.src(release.templates.bower)
          .pipe(plumber())
          .pipe(flatten())
          .pipe(jsonEditor(function(bower) {
            bower.name = packageName;
            bower.description = capitalizedComponent + ' - Semantic UI';
            if(isJavascript) {
              if(isCSS) {
                bower.main = [
                  component + '.js',
                  component + '.css'
                ];
              }
              else {
                bower.main = [
                  component + '.js'
                ];
              }
              bower.dependencies = {
                jquery: '>=1.8'
              };
            }
            else {
              bower.main = [
                component + '.css'
              ];
            }
            return bower;
          }))
          .pipe(gulp.dest(outputDirectory))
        ;
      }

      // extend package.json
      function extendPackage() {
        return gulp.src(release.templates.package)
          .pipe(plumber())
          .pipe(flatten())
          .pipe(jsonEditor(function(npm) {
            if(isJavascript) {
              npm.dependencies = {
                jquery: 'x.x.x'
              };
              npm.main = 'index.js';
            }
            npm.name = packageName;
            if(version) {
              npm.version = version;
            }
            npm.title       = 'Semantic UI - ' + capitalizedComponent;
            npm.description = 'Single component release of ' + component;
            npm.repository  = {
              type : 'git',
              url  : gitURL
            };
            return npm;
          }))
          .pipe(gulp.dest(outputDirectory))
        ;
      }

      // extend composer.json
      function extendComposer(){
        return gulp.src(release.templates.composer)
          .pipe(plumber())
          .pipe(flatten())
          .pipe(jsonEditor(function(composer) {
            if(isJavascript) {
              composer.dependencies = {
                jquery: 'x.x.x'
              };
              composer.main = component + '.js';
            }
            composer.name = 'semantic/' + component;
            if(version) {
              composer.version = version;
            }
            composer.description = 'Single component release of ' + component;
            return composer;
          }))
          .pipe(gulp.dest(outputDirectory))
        ;
      }

      // create release notes
      function createReleaseNotes() {
        return gulp.src(release.templates.notes)
          .pipe(plumber())
          .pipe(flatten())
          // Remove release notes for lines not mentioning component
          .pipe(replace(regExp.match.unrelatedNotes, regExp.replace.unrelatedNotes))
          .pipe(replace(regExp.match.whitespace, regExp.replace.whitespace))
          .pipe(replace(regExp.match.spacedVersions, regExp.replace.spacedVersions))
          .pipe(replace(regExp.match.spacedLists, regExp.replace.spacedLists))
          .pipe(replace(regExp.match.trim, regExp.replace.trim))
          .pipe(gulp.dest(outputDirectory))
        ;
      }

      // Creates meteor package.js
      function createMeteorPackage() {
        var
          filenames = ''
        ;
        return gulp.src(manifest.component)
          .pipe(concatFileNames('empty.txt', concatSettings))
          .pipe(tap(function(file) {
            filenames += file.contents;
          }))
          .on('end', function() {
            gulp.src(manifest.assets)
              .pipe(concatFileNames('empty.txt', concatSettings))
              .pipe(tap(function(file) {
                filenames += file.contents;
              }))
              .on('end', function() {
                // remove trailing slash
                filenames = filenames.replace(regExp.match.trailingComma, '').trim();
                gulp.src(release.templates.meteor.component)
                  .pipe(plumber())
                  .pipe(flatten())
                  .pipe(replace(regExp.match.name, regExp.replace.name))
                  .pipe(replace(regExp.match.titleName, regExp.replace.titleName))
                  .pipe(replace(regExp.match.version, version))
                  .pipe(replace(regExp.match.files, filenames))
                  .pipe(rename(release.files.meteor))
                  .pipe(gulp.dest(outputDirectory))
                ;
              })
            ;
          })
        ;
      }

      tasks.push(gulp.series(
          copyDist,
          createNpmModule,
          extendBower,
          createReadme,
          extendPackage,
          extendComposer,
          createReleaseNotes,
          createMeteorPackage
      ));
    })(component);
  }

  gulp.series(...tasks)(callback);
};
