/*******************************
            Set-up
*******************************/

var
  fs             = require('fs'),
  path           = require('path'),
  defaults       = require('../defaults'),
  release        = require('./release'),

  requireDotFile = require('require-dot-file')
;

/*******************************
          When to Ask
*******************************/

/* Preconditions for install questions */

var when = {

  // path
  changeRoot: function(questions) {
    return (questions.useRoot !== undefined && questions.useRoot !== true);
  },

  // permissions
  changePermissions: function(questions) {
    return (questions.changePermissions && questions.changePermissions === true);
  },

  // install
  hasConfig: function() {
    return requireDotFile('semantic.json', process.cwd());
  },

  allowOverwrite: function(questions) {
    return (questions.overwrite === undefined || questions.overwrite == 'yes');
  },
  notAuto: function(questions) {
    return (questions.install !== 'auto' && (questions.overwrite === undefined || questions.overwrite == 'yes'));
  },
  custom: function(questions) {
    return (questions.install === 'custom' && (questions.overwrite === undefined || questions.overwrite == 'yes'));
  },
  express: function(questions) {
    return (questions.install === 'express' && (questions.overwrite === undefined || questions.overwrite == 'yes'));
  },

  // customize
  customize: function(questions) {
    return (questions.customize === true);
  },
  primaryColor: function(questions) {
    return (questions.primaryColor);
  },
  secondaryColor: function(questions) {
    return (questions.secondaryColor);
  }
};

/*******************************
        Response Filters
*******************************/

/* Filters to user input from install questions */

var filter = {
  removeTrailingSlash: function(path) {
    return path.replace(/(\/$|\\$)+/mg, '');
  }
};

/*******************************
          Configuration
*******************************/

module.exports = {

  // check whether install is setup
  isSetup: function() {
    return when.hasConfig();
  },

  // detect whether there is a semantic.json configuration and that the auto-install option is set to true
  shouldAutoInstall: function() {
    var
      config = when.hasConfig()
    ;
    return config['autoInstall'];
  },

  // checks if files are in a PM directory
  getPackageManager: function(directory) {
    var
      // returns last matching result (avoid sub-module detection)
      walk = function(directory) {
        var
          pathArray     = directory.split(path.sep),
          folder        = pathArray[pathArray.length - 1],
          nextDirectory = path.join(directory, path.sep, '..')
        ;
        if( folder == 'bower_components') {
          return {
            name: 'Bower',
            root: nextDirectory
          };
        }
        else if(folder == 'node_modules') {
         return {
            name: 'NPM',
            root: nextDirectory
          };
        }
        else if(folder == 'composer') {
         return {
            name: 'Composer',
            root: nextDirectory
          };
        }
        if(path.resolve(directory) == path.resolve(nextDirectory)) {
          return false;
        }
        // recurse downward
        return walk(nextDirectory);
      }
    ;
    // start walk from current directory if none specified
    directory = directory || (__dirname + path.sep);
    return walk(directory);
  },

  // checks if files is PMed submodule
  isSubModule: function(directory) {
    var
      moduleFolders = 0,
      walk = function(directory) {
        var
          pathArray     = directory.split(path.sep),
          folder        = pathArray[pathArray.length - 2],
          nextDirectory = path.join(directory, path.sep, '..')
        ;
        if( folder == 'bower_components') {
          moduleFolders++;
        }
        else if(folder == 'node_modules') {
          moduleFolders++;
        }
        else if(folder == 'composer') {
          moduleFolders++;
        }
        if(path.resolve(directory) == path.resolve(nextDirectory)) {
          return (moduleFolders > 1);
        }
        // recurse downward
        return walk(nextDirectory);
      }
    ;
    // start walk from current directory if none specified
    directory = directory || (__dirname + path.sep);
    return walk(directory);
  },


  createJSON: function(answers) {
    var
      json = {
        paths: {
          source: {},
          output: {}
        }
      }
    ;

    // add components
    if(answers.components) {
      json.components = answers.components;
    }

    // add rtl choice
    if(answers.rtl) {
      json.rtl = answers.rtl;
    }

    // add permissions
    if(answers.permission) {
      json.permission = answers.permission;
    }

    // add path to semantic
    if(answers.semanticRoot) {
      json.base = path.normalize(answers.semanticRoot);
    }

    // record version number to avoid re-installing on same version
    json.version = release.version;

    // add dist folder paths
    if(answers.dist) {
      answers.dist = path.normalize(answers.dist);

      json.paths.output = {
        packaged     : path.normalize(answers.dist + '/'),
        uncompressed : path.normalize(answers.dist + '/components/'),
        compressed   : path.normalize(answers.dist + '/components/'),
        themes       : path.normalize(answers.dist + '/themes/')
      };
    }

    // add site path
    if(answers.site) {
      json.paths.source.site = path.normalize(answers.site + '/');
    }
    if(answers.packaged) {
      json.paths.output.packaged = path.normalize(answers.packaged + '/');
    }
    if(answers.compressed) {
      json.paths.output.compressed = path.normalize(answers.compressed + '/');
    }
    if(answers.uncompressed) {
      json.paths.output.uncompressed = path.normalize(answers.uncompressed + '/');
    }
    return json;
  },

  // files cleaned up after install
  setupFiles: [
    './src/theme.config.example',
    './semantic.json.example',
    './src/_site'
  ],

  regExp: {
    // used to match siteFolder variable in theme.less
    siteVariable: /@siteFolder .*\'(.*)/mg
  },

  // source paths (when installing)
  source: {
    config       : './semantic.json.example',
    definitions  : './src/definitions',
    gulpFile     : './gulpfile.js',
    lessImport   : './src/semantic.less',
    site         : './src/_site',
    tasks        : './tasks',
    themeConfig  : './src/theme.config.example',
    themeImport  : './src/theme.less',
    themes       : './src/themes',
    defaultTheme : './src/themes/default',
    userGulpFile : './tasks/config/npm/gulpfile.js'
  },

  // expected final filenames
  files: {
    config      : 'semantic.json',
    lessImport  : 'src/semantic.less',
    site        : 'src/site',
    themeConfig : 'src/theme.config',
    themeImport : 'src/theme.less'
  },

  // folder paths to files relative to root
  folders: {
    config       : './',
    definitions  : 'src/definitions/',
    lessImport   : 'src/',
    modules      : 'node_modules/',
    site         : 'src/site/',
    tasks        : 'tasks/',
    themeConfig  : 'src/',
    themeImport  : 'src/',
    themes       : 'src/themes/',

    defaultTheme : 'default/' // only path that is relative to another directory and not root
  },

  // questions asked during install
  questions: {

    root: [
      {
        type    : 'list',
        name    : 'useRoot',
        message :
          '{packageMessage} Is this your project folder? {root}',
        choices: [
          {
            name  : 'Yes',
            value : true
          },
          {
            name  : 'No, let me specify',
            value : false
          }
        ]
      },
      {
        type    : 'input',
        name    : 'customRoot',
        message : 'Please enter the absolute path to your project root',
        default : '/my/project/path',
        when    : when.changeRoot
      },
      {
        type    : 'input',
        name    : 'semanticRoot',
        message : 'Where should we put Semantic UI inside your project?',
        default : 'semantic/'
      }
    ],

    setup: [
      {
        type: 'list',
        name: 'overwrite',
        message: 'It looks like you have a semantic.json file already.',
        when: when.hasConfig,
        choices: [
          {
            name: 'Yes, extend my current settings.',
            value: 'yes'
          },
          {
            name: 'Skip install',
            value: 'no'
          }
        ]
      },
      {
        type: 'list',
        name: 'install',
        message: 'Set-up Semantic UI',
        when: when.allowOverwrite,
        choices: [
          {
            name: 'Automatic (Use default locations and all components)',
            value: 'auto'
          },
          {
            name: 'Express (Set components and output folder)',
            value: 'express'
          },
          {
            name: 'Custom (Customize all src/dist values)',
            value: 'custom'
          }
        ]
      },
      {
        type: 'checkbox',
        name: 'components',
        message: 'What components should we include in the package?',

        // duplicated manually from tasks/defaults.js with additional property
        choices: [
          { name: "reset", checked: true },
          { name: "site", checked: true },
          { name: "button", checked: true },
          { name: "container", checked: true },
          { name: "divider", checked: true },
          { name: "emoji", checked: true },
          { name: "flag", checked: true },
          { name: "header", checked: true },
          { name: "icon", checked: true },
          { name: "image", checked: true },
          { name: "input", checked: true },
          { name: "label", checked: true },
          { name: "list", checked: true },
          { name: "loader", checked: true },
          { name: "rail", checked: true },
          { name: "reveal", checked: true },
          { name: "segment", checked: true },
          { name: "step", checked: true },
          { name: "breadcrumb", checked: true },
          { name: "form", checked: true },
          { name: "grid", checked: true },
          { name: "menu", checked: true },
          { name: "message", checked: true },
          { name: "table", checked: true },
          { name: "ad", checked: true },
          { name: "card", checked: true },
          { name: "comment", checked: true },
          { name: "feed", checked: true },
          { name: "item", checked: true },
          { name: "statistic", checked: true },
          { name: "accordion", checked: true },
          { name: "calendar", checked: true },
          { name: "checkbox", checked: true },
          { name: "dimmer", checked: true },
          { name: "dropdown", checked: true },
          { name: "embed", checked: true },
          { name: "modal", checked: true },
          { name: "nag", checked: true },
          { name: "placeholder", checked: true },
          { name: "popup", checked: true },
          { name: "progress", checked: true },
          { name: "slider", checked: true },
          { name: "rating", checked: true },
          { name: "search", checked: true },
          { name: "shape", checked: true },
          { name: "sidebar", checked: true },
          { name: "sticky", checked: true },
          { name: "tab", checked: true },
          { name: "text", checked: true },
          { name: "toast", checked: true },
          { name: "transition", checked: true },
          { name: "api", checked: true },
          { name: "form", checked: true },
          { name: "state", checked: true },
          { name: "visibility", checked: true }
        ],
        when: when.notAuto
      },
      {
        type: 'list',
        name: 'changePermissions',
        when: when.notAuto,
        message: 'Should we set permissions on outputted files?',
        choices: [
          {
            name: 'No',
            value: false
          },
          {
            name: 'Yes',
            value: true
          }
        ]
      },
      {
        type: 'input',
        name: 'permission',
        message: 'What octal file permission should outputted files receive?',
        default: defaults.permission,
        when: when.changePermissions
      },
      {
        type: 'list',
        name: 'rtl',
        message: 'Do you use a RTL (Right-To-Left) language?',
        when: when.notAuto,
        choices: [
          {
            name: 'No',
            value: false
          },
          {
            name: 'Yes',
            value: true
          },
          {
            name: 'Build Both',
            value: 'both'
          }
        ]
      },
      {
        type: 'input',
        name: 'dist',
        message: 'Where should we output Semantic UI?',
        default: defaults.paths.output.packaged,
        filter: filter.removeTrailingSlash,
        when: when.express
      },
      {
        type: 'input',
        name: 'site',
        message: 'Where should we put your site folder?',
        default: defaults.paths.source.site,
        filter: filter.removeTrailingSlash,
        when: when.custom
      },
      {
        type: 'input',
        name: 'packaged',
        message: 'Where should we output a packaged version?',
        default: defaults.paths.output.packaged,
        filter: filter.removeTrailingSlash,
        when: when.custom
      },
      {
        type: 'input',
        name: 'compressed',
        message: 'Where should we output compressed components?',
        default: defaults.paths.output.compressed,
        filter: filter.removeTrailingSlash,
        when: when.custom
      },
      {
        type: 'input',
        name: 'uncompressed',
        message: 'Where should we output uncompressed components?',
        default: defaults.paths.output.uncompressed,
        filter: filter.removeTrailingSlash,
        when: when.custom
      }
    ],


    cleanup: [
      {
        type: 'list',
        name: 'cleanup',
        message: 'Should we remove set-up files?',
        choices: [
          {
            name: 'Yes (re-install will require redownloading semantic).',
            value: 'yes'
          },
          {
            name: 'No Thanks',
            value: 'no'
          }
        ]
      },
      {
        type: 'list',
        name: 'build',
        message: 'Do you want to build Semantic now?',
        choices: [
          {
            name: 'Yes',
            value: 'yes'
          },
          {
            name: 'No',
            value: 'no'
          }
        ]
      },
    ],
    site: [
      {
        type: 'list',
        name: 'customize',
        message: 'You have not yet customized your site, can we help you do that?',
        choices: [
          {
            name: 'Yes, ask me a few questions',
            value: true
          },
          {
            name: 'No I\'ll do it myself',
            value: false
          }
        ]
      },
      {
        type: 'list',
        name: 'headerFont',
        message: 'Select your header font',
        choices: [
          {
            name: 'Helvetica Neue, Arial, sans-serif',
            value: 'Helvetica Neue, Arial, sans-serif;'
          },
          {
            name: 'Lato (Google Fonts)',
            value: 'Lato'
          },
          {
            name: 'Open Sans (Google Fonts)',
            value: 'Open Sans'
          },
          {
            name: 'Source Sans Pro (Google Fonts)',
            value: 'Source Sans Pro'
          },
          {
            name: 'Droid (Google Fonts)',
            value: 'Droid'
          },
          {
            name: 'I\'ll choose on my own',
            value: false
          }
        ],
        when: when.customize
      },
      {
        type: 'list',
        name: 'pageFont',
        message: 'Select your page font',
        choices: [
          {
            name: 'Helvetica Neue, Arial, sans-serif',
            value: 'Helvetica Neue, Arial, sans-serif;'
          },
          {
            name: 'Lato (Import from Google Fonts)',
            value: 'Lato'
          },
          {
            name: 'Open Sans (Import from Google Fonts)',
            value: 'Open Sans'
          },
          {
            name: 'Source Sans Pro (Import from Google Fonts)',
            value: 'Source Sans Pro'
          },
          {
            name: 'Droid (Google Fonts)',
            value: 'Droid'
          },
          {
            name: 'I\'ll choose on my own',
            value: false
          }
        ],
        when: when.customize
      },
      {
        type: 'list',
        name: 'fontSize',
        message: 'Select your base font size',
        default: '14px',
        choices: [
          {
            name: '12px',
          },
          {
            name: '13px',
          },
          {
            name: '14px (Recommended)',
            value: '14px'
          },
          {
            name: '15px',
          },
          {
            name: '16px',
          },
          {
            name: 'I\'ll choose on my own',
            value: false
          }
        ],
        when: when.customize
      },
      {
        type: 'list',
        name: 'primaryColor',
        message: 'Select the closest name for your primary brand color',
        default: '14px',
        choices: [
          {
            name: 'Blue'
          },
          {
            name: 'Green'
          },
          {
            name: 'Orange'
          },
          {
            name: 'Pink'
          },
          {
            name: 'Purple'
          },
          {
            name: 'Red'
          },
          {
            name: 'Teal'
          },
          {
            name: 'Yellow'
          },
          {
            name: 'Black'
          },
          {
            name: 'I\'ll choose on my own',
            value: false
          }
        ],
        when: when.customize
      },
      {
        type: 'input',
        name: 'PrimaryHex',
        message: 'Enter a hexcode for your primary brand color',
        when: when.primaryColor
      },
      {
        type: 'list',
        name: 'secondaryColor',
        message: 'Select the closest name for your secondary brand color',
        default: '14px',
        choices: [
          {
            name: 'Blue'
          },
          {
            name: 'Green'
          },
          {
            name: 'Orange'
          },
          {
            name: 'Pink'
          },
          {
            name: 'Purple'
          },
          {
            name: 'Red'
          },
          {
            name: 'Teal'
          },
          {
            name: 'Yellow'
          },
          {
            name: 'Black'
          },
          {
            name: 'I\'ll choose on my own',
            value: false
          }
        ],
        when: when.customize
      },
      {
        type: 'input',
        name: 'secondaryHex',
        message: 'Enter a hexcode for your secondary brand color',
        when: when.secondaryColor
      }
    ]

  },

  settings: {

    /* Rename Files */
    rename: {
      json : { extname : '.json' }
    },

    /* Copy Install Folders */
    wrench: {

      // overwrite existing files update & install (default theme / definition)
      overwrite: {
        forceDelete       : true,
        excludeHiddenUnix : true,
        preserveFiles     : false
      },

      // only create files that don't exist (site theme update)
      merge: {
        forceDelete       : false,
        excludeHiddenUnix : true,
        preserveFiles     : true
      }

    }
  }
};
