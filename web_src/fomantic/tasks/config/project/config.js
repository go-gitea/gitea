/*******************************
            Set-up
*******************************/

var
  extend   = require('extend'),
  fs       = require('fs'),
  path     = require('path'),

  defaults = require('../defaults')
;


/*******************************
            Exports
*******************************/

module.exports = {

  getPath: function(file, directory) {
    var
      configPath,
      walk = function(directory) {
        var
          nextDirectory = path.resolve( path.join(directory, path.sep, '..') ),
          currentPath   = path.normalize( path.join(directory, file) )
        ;
        if( fs.existsSync(currentPath) ) {
          // found file
          configPath = path.normalize(directory);
          return;
        }
        else {
          // reached file system root, let's stop
          if(nextDirectory == directory) {
            return;
          }
          // otherwise recurse
          walk(nextDirectory, file);
        }
      }
    ;

    // start walk from outside require-dot-files directory
    file      = file || defaults.files.config;
    directory = directory || path.join(__dirname, path.sep, '..');
    walk(directory);
    return configPath || '';
  },

  // adds additional derived values to a config object
  addDerivedValues: function(config) {

    config = config || extend(false, {}, defaults);

    /*--------------
       File Paths
    ---------------*/

    var
      configPath = this.getPath(),
      sourcePaths = {},
      outputPaths = {},
      folder
    ;

    // resolve paths (config location + base + path)
    for(folder in config.paths.source) {
      if(config.paths.source.hasOwnProperty(folder)) {
        sourcePaths[folder] = path.resolve(path.join(configPath, config.base, config.paths.source[folder]));
      }
    }
    for(folder in config.paths.output) {
      if(config.paths.output.hasOwnProperty(folder)) {
        outputPaths[folder] = path.resolve(path.join(configPath, config.base, config.paths.output[folder]));
      }
    }

    // set config paths to full paths
    config.paths.source = sourcePaths;
    config.paths.output = outputPaths;

    // resolve "clean" command path
    config.paths.clean = path.resolve( path.join(configPath, config.base, config.paths.clean) );

    /*--------------
        CSS URLs
    ---------------*/

    // determine asset paths in css by finding relative path between themes and output
    // force forward slashes

    config.paths.assets = {
      source       : '../../themes', // source asset path is always the same
      uncompressed : './' + path.relative(config.paths.output.uncompressed, config.paths.output.themes).replace(/\\/g, '/'),
      compressed   : './' + path.relative(config.paths.output.compressed, config.paths.output.themes).replace(/\\/g, '/'),
      packaged     : './' + path.relative(config.paths.output.packaged, config.paths.output.themes).replace(/\\/g, '/')
    };

    /*--------------
       Permission
    ---------------*/

    if(config.permission) {
      config.hasPermissions = true;
      config.parsedPermissions = typeof config.permission === 'string' ? parseInt(config.permission, 8) : config.permission;
    }
    else {
      // pass blank object to avoid causing errors
      config.permission     = {};
      config.hasPermissions = false;
      config.parsedPermissions = {};
    }

    /*--------------
         Globs
    ---------------*/

    if(!config.globs) {
      config.globs = {};
    }

    // remove duplicates from component array
    if(config.components instanceof Array) {
      config.components = config.components.filter(function(component, index) {
        return config.components.indexOf(component) == index;
      });
    }

    // takes component object and creates file glob matching selected components
    config.globs.components = (typeof config.components == 'object')
      ? (config.components.length > 1)
        ? '{' + config.components.join(',') + '}'
        : config.components[0]
      : '{' + defaults.components.join(',') + '}'
    ;

    return config;

  }

};

