/*******************************
        Release Settings
*******************************/

// release settings
module.exports = {

  // path to components for repos
  source     : './dist/components/',

  // modified asset paths for component repos
  paths: {
    source : '../themes/default/assets/',
    output : 'assets/'
  },

  templates: {
    bower    : './tasks/config/admin/templates/bower.json',
    composer : './tasks/config/admin/templates/composer.json',
    package  : './tasks/config/admin/templates/package.json',
    meteor   : {
      css       : './tasks/config/admin/templates/css-package.js',
      component : './tasks/config/admin/templates/component-package.js',
      less      : './tasks/config/admin/templates/less-package.js',
    },
    readme : './tasks/config/admin/templates/README.md',
    notes  : './RELEASE-NOTES.md'
  },

  org         : 'Semantic-Org',
  repo        : 'Semantic-UI',

  // files created for package managers
  files: {
    composer : 'composer.json',
    config   : 'semantic.json',
    npm      : 'package.json',
    meteor   : 'package.js'
  },

  // root name for distribution repos
  distRepoRoot      : 'Semantic-UI-',

  // root name for single component repos
  componentRepoRoot : 'UI-',

  // root name for package managers
  packageRoot          : 'semantic-ui-',

  // root path to repos
  outputRoot  : '../repos/',

  homepage    : 'http://www.semantic-ui.com',

  // distributions that get separate repos
  distributions: [
    'LESS',
    'CSS'
  ],

  // components that get separate repositories for bower/npm
  components : [
    'accordion',
    'ad',
    'api',
    'breadcrumb',
    'button',
    'card',
    'calendar',
    'checkbox',
    'comment',
    'container',
    'dimmer',
    'divider',
    'dropdown',
    'embed',
    'emoji',
    'feed',
    'flag',
    'form',
    'grid',
    'header',
    'icon',
    'image',
    'input',
    'item',
    'label',
    'list',
    'loader',
    'menu',
    'message',
    'modal',
    'nag',
    'placeholder',
    'popup',
    'progress',
    'rail',
    'slider',
    'rating',
    'reset',
    'reveal',
    'search',
    'segment',
    'shape',
    'sidebar',
    'site',
    'statistic',
    'step',
    'sticky',
    'tab',
    'table',
    'text',
    'toast',
    'transition',
    'visibility'
  ]
};
