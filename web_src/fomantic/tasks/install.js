/*******************************
 *         Install Task
 *******************************/

/*
   Install tasks

   For more notes

   * Runs automatically after npm update (hooks)
   * (NPM) Install - Will ask for where to put semantic (outside pm folder)
   * (NPM) Upgrade - Will look for semantic install, copy over files and update if new version
   * Standard installer runs asking for paths to site files etc

*/

var
  gulp           = require('gulp'),

  // node dependencies
  console        = require('better-console'),
  extend         = require('extend'),
  fs             = require('fs'),
  mkdirp         = require('mkdirp'),
  path           = require('path'),

  // gulp dependencies
  chmod          = require('gulp-chmod'),
  del            = require('del'),
  jsonEditor     = require('gulp-json-editor'),
  plumber        = require('gulp-plumber'),
  inquirer       = require('inquirer'),
  rename         = require('gulp-rename'),
  replace        = require('gulp-replace'),
  requireDotFile = require('require-dot-file'),
  wrench         = require('wrench-sui'),

  // install config
  install        = require('./config/project/install'),

  // user config
  config         = require('./config/user'),

  // release config (name/title/etc)
  release        = require('./config/project/release'),

  // shorthand
  questions      = install.questions,
  files          = install.files,
  folders        = install.folders,
  regExp         = install.regExp,
  settings       = install.settings,
  source         = install.source
;

// Export install task
module.exports = function (callback) {

  var
    currentConfig = requireDotFile('semantic.json'),
    manager       = install.getPackageManager(),
    rootQuestions = questions.root,
    installFolder = false,
    answers
  ;

  console.clear();

  /* Test NPM install
  manager = {
    name : 'NPM',
    root : path.normalize(__dirname + '/../')
  };
  */


  /* Don't do end user config if SUI is a sub-module */
  if (install.isSubModule()) {
    console.info('SUI is a sub-module, skipping end-user install');
    return;
  }

  /*-----------------
      Update SUI
  -----------------*/

// run update scripts if semantic.json exists
  if (currentConfig && manager.name === 'NPM') {

    var
      updateFolder = path.join(manager.root, currentConfig.base),
      updatePaths  = {
        config      : path.join(manager.root, files.config),
        tasks       : path.join(updateFolder, folders.tasks),
        themeImport : path.join(updateFolder, folders.themeImport),
        definition  : path.join(currentConfig.paths.source.definitions),
        site        : path.join(currentConfig.paths.source.site),
        theme       : path.join(currentConfig.paths.source.themes),
        defaultTheme: path.join(currentConfig.paths.source.themes, folders.defaultTheme)
      }
    ;

    // duck-type if there is a project installed
    if (fs.existsSync(updatePaths.definition)) {

      // perform update if new version
      if (currentConfig.version !== release.version) {
        console.log('Updating Semantic UI from ' + currentConfig.version + ' to ' + release.version);

        console.info('Updating ui definitions...');
        wrench.copyDirSyncRecursive(source.definitions, updatePaths.definition, settings.wrench.overwrite);

        console.info('Updating default theme...');
        wrench.copyDirSyncRecursive(source.themes, updatePaths.theme, settings.wrench.merge);
        wrench.copyDirSyncRecursive(source.defaultTheme, updatePaths.defaultTheme, settings.wrench.overwrite);

        console.info('Updating tasks...');
        wrench.copyDirSyncRecursive(source.tasks, updatePaths.tasks, settings.wrench.overwrite);

        console.info('Updating gulpfile.js');
        gulp.src(source.userGulpFile)
          .pipe(plumber())
          .pipe(gulp.dest(updateFolder))
        ;

        // copy theme import
        console.info('Updating theme import file');
        gulp.src(source.themeImport)
          .pipe(plumber())
          .pipe(gulp.dest(updatePaths.themeImport))
        ;

        console.info('Adding new site theme files...');
        wrench.copyDirSyncRecursive(source.site, updatePaths.site, settings.wrench.merge);

        console.info('Updating version...');

        // update version number in semantic.json
        gulp.src(updatePaths.config)
          .pipe(plumber())
          .pipe(rename(settings.rename.json)) // preserve file extension
          .pipe(jsonEditor({
            version: release.version
          }))
          .pipe(gulp.dest(manager.root))
        ;

        console.info('Update complete! Run "\x1b[92mgulp build\x1b[0m" to rebuild dist/ files.');

        callback();
        return;
      } else {
        console.log('Current version of Semantic UI already installed');
        callback();
        return;
      }

    } else {
      console.error('Cannot locate files to update at path: ', updatePaths.definition);
      console.log('Running installer');
    }

  }

  /*--------------
   Determine Root
  ---------------*/

// PM that supports Build Tools (NPM Only Now)
  if (manager.name === 'NPM') {
    rootQuestions[0].message = rootQuestions[0].message
      .replace('{packageMessage}', 'We detected you are using ' + manager.name + ' Nice!')
      .replace('{root}', manager.root)
    ;
    // set default path to detected PM root
    rootQuestions[0].default = manager.root;
    rootQuestions[1].default = manager.root;

    // insert PM questions after "Install Type" question
    Array.prototype.splice.apply(questions.setup, [2, 0].concat(rootQuestions));

    // omit cleanup questions for managed install
    questions.cleanup = [];
  }


  /*--------------
     Create SUI
  ---------------*/

  gulp.task('run setup', function (callback) {

    // If auto-install is switched on, we skip the configuration section and simply reuse the configuration from semantic.json
    if (install.shouldAutoInstall()) {
      answers = {
        overwrite   : 'yes',
        install     : 'auto',
        useRoot     : true,
        semanticRoot: currentConfig.base
      };
      callback();
    } else {
      return inquirer.prompt(questions.setup)
        .then((setupAnswers) => {
          // hoist
          answers = setupAnswers;
        });
    }
  });

  gulp.task('create install files', function (callback) {

    /*--------------
     Exit Conditions
    ---------------*/

    // if config exists and user specifies not to proceed
    if (answers.overwrite !== undefined && answers.overwrite == 'no') {
      callback();
      return;
    }
    console.clear();
    if (install.shouldAutoInstall()) {
      console.log('Auto-Installing (Without User Interaction)');
    } else {
      console.log('Installing');
    }
    console.log('------------------------------');


    /*--------------
          Paths
    ---------------*/

    var
      installPaths = {
        config           : files.config,
        configFolder     : folders.config,
        site             : answers.site || folders.site,
        themeConfig      : files.themeConfig,
        themeConfigFolder: folders.themeConfig
      }
    ;

    /*--------------
      NPM Install
    ---------------*/

    // Check if PM install
    if (manager && (answers.useRoot || answers.customRoot)) {

      // Set root to custom root path if set
      if (answers.customRoot) {
        if (answers.customRoot === '') {
          console.log('Unable to proceed, invalid project root');
          callback();
          return;
        }
        manager.root = answers.customRoot;
      }

      // special install paths only for PM install
      installPaths = extend(false, {}, installPaths, {
        definition  : folders.definitions,
        lessImport  : folders.lessImport,
        tasks       : folders.tasks,
        theme       : folders.themes,
        defaultTheme: path.join(folders.themes, folders.defaultTheme),
        themeImport : folders.themeImport
      });

      // add project root to semantic root
      installFolder = path.join(manager.root, answers.semanticRoot);

      // add install folder to all output paths
      for (var destination in installPaths) {
        if (installPaths.hasOwnProperty(destination)) {
          // config goes in project root, rest in install folder
          installPaths[destination] = (destination == 'config' || destination == 'configFolder')
            ? path.normalize(path.join(manager.root, installPaths[destination]))
            : path.normalize(path.join(installFolder, installPaths[destination]))
          ;
        }
      }

      // create project folders
      try {
        mkdirp.sync(installFolder);
        mkdirp.sync(installPaths.definition);
        mkdirp.sync(installPaths.theme);
        mkdirp.sync(installPaths.tasks);
      } catch (error) {
        console.error('NPM does not have permissions to create folders at your specified path. Adjust your folders permissions and run "npm install" again');
      }

      console.log('Installing to \x1b[92m' + answers.semanticRoot + '\x1b[0m');

      console.info('Copying UI definitions');
      wrench.copyDirSyncRecursive(source.definitions, installPaths.definition, settings.wrench.overwrite);

      console.info('Copying UI themes');
      wrench.copyDirSyncRecursive(source.themes, installPaths.theme, settings.wrench.merge);
      wrench.copyDirSyncRecursive(source.defaultTheme, installPaths.defaultTheme, settings.wrench.overwrite);

      console.info('Copying gulp tasks');
      wrench.copyDirSyncRecursive(source.tasks, installPaths.tasks, settings.wrench.overwrite);

      // copy theme import
      console.info('Adding theme files');
      gulp.src(source.themeImport)
        .pipe(plumber())
        .pipe(gulp.dest(installPaths.themeImport))
      ;
      gulp.src(source.lessImport)
        .pipe(plumber())
        .pipe(gulp.dest(installPaths.lessImport))
      ;

      // create gulp file
      console.info('Creating gulpfile.js');
      gulp.src(source.userGulpFile)
        .pipe(plumber())
        .pipe(gulp.dest(installFolder))
      ;

    }


    /*--------------
       Site Theme
    ---------------*/

    // Copy _site templates folder to destination
    if (fs.existsSync(installPaths.site)) {
      console.info('Site folder exists, merging files (no overwrite)', installPaths.site);
    } else {
      console.info('Creating site theme folder', installPaths.site);
    }
    wrench.copyDirSyncRecursive(source.site, installPaths.site, settings.wrench.merge);

    /*--------------
      Theme Config
    ---------------*/

    gulp.task('create theme.config', function () {
      var
        // determine path to site theme folder from theme config
        // force CSS path variable to use forward slashes for paths
        pathToSite   = path.relative(path.resolve(installPaths.themeConfigFolder), path.resolve(installPaths.site)).replace(/\\/g, '/'),
        siteVariable = "@siteFolder   : '" + pathToSite + "/';"
      ;

      // rewrite site variable in theme.less
      console.info('Adjusting @siteFolder to: ', pathToSite + '/');

      if (fs.existsSync(installPaths.themeConfig)) {
        console.info('Modifying src/theme.config (LESS config)', installPaths.themeConfig);
        return gulp.src(installPaths.themeConfig)
          .pipe(plumber())
          .pipe(replace(regExp.siteVariable, siteVariable))
          .pipe(gulp.dest(installPaths.themeConfigFolder))
          ;
      } else {
        console.info('Creating src/theme.config (LESS config)', installPaths.themeConfig);
        return gulp.src(source.themeConfig)
          .pipe(plumber())
          .pipe(rename({extname: ''}))
          .pipe(replace(regExp.siteVariable, siteVariable))
          .pipe(gulp.dest(installPaths.themeConfigFolder))
          ;
      }
    });

    /*--------------
      Semantic.json
    ---------------*/

    gulp.task('create semantic.json', function () {

      var
        jsonConfig = install.createJSON(answers)
      ;

      // adjust variables in theme.less
      if (fs.existsSync(installPaths.config)) {
        console.info('Extending config file (semantic.json)', installPaths.config);
        return gulp.src(installPaths.config)
          .pipe(plumber())
          .pipe(rename(settings.rename.json)) // preserve file extension
          .pipe(jsonEditor(jsonConfig))
          .pipe(gulp.dest(installPaths.configFolder))
          ;
      } else {
        console.info('Creating config file (semantic.json)', installPaths.config);
        return gulp.src(source.config)
          .pipe(plumber())
          .pipe(rename({extname: ''})) // remove .template from ext
          .pipe(jsonEditor(jsonConfig, {end_with_newline: true}))
          .pipe(gulp.dest(installPaths.configFolder))
          ;
      }

    });

    gulp.series('create theme.config', 'create semantic.json')(callback);
  });

  gulp.task('clean up install', function (callback) {

    // Completion Message
    if (installFolder && !install.shouldAutoInstall()) {
      console.log('\n Setup Complete! \n Installing Peer Dependencies. \x1b[0;31mPlease refrain from ctrl + c\x1b[0m... \n After completion navigate to \x1b[92m' + answers.semanticRoot + '\x1b[0m and run "\x1b[92mgulp build\x1b[0m" to build');
      callback();
    } else {
      console.log('');
      console.log('');

      // If auto-install is switched on, we skip the configuration section and simply build the dependencies
      if (install.shouldAutoInstall()) {
        gulp.series('build')(callback);
      } else {
        // We don't return the inquirer promise on purpose because we handle the callback ourselves
        inquirer.prompt(questions.cleanup)
          .then((answers) => {
            if (answers.cleanup === 'yes') {
              del(install.setupFiles);
            }
            if (answers.build === 'yes') {
              gulp.series('build')(callback);
            } else {
              callback();
            }
          });
      }
    }
  });

  gulp.series('run setup', 'create install files', 'clean up install')(callback);
};
