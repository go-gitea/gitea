/*******************************
          Default Paths
*******************************/

module.exports = {

  // base path added to all other paths
  base : '',

  // base path when installed with npm
  pmRoot: 'semantic/',

  // octal permission for output files, i.e. 0o644 or '644' (false does not adjust)
  permission : '744',

  // whether to generate rtl files
  rtl        : false,

  // file paths
  files: {
    config   : 'semantic.json',
    site     : 'src/site',
    theme    : 'src/theme.config'
  },

  // folder paths
  paths: {
    source: {
      config      : 'src/theme.config',
      definitions : 'src/definitions/',
      site        : 'src/site/',
      themes      : 'src/themes/'
    },
    output: {
      packaged     : 'dist/',
      uncompressed : 'dist/components/',
      compressed   : 'dist/components/',
      themes       : 'dist/themes/'
    },
    clean : 'dist/'
  },

  // components to include in package
  components: [

    // global
    'reset',
    'site',

    // elements
    'button',
    'container',
    'divider',
    'emoji',
    'flag',
    'header',
    'icon',
    'image',
    'input',
    'label',
    'list',
    'loader',
    'placeholder',
    'rail',
    'reveal',
    'segment',
    'step',
    'text',

    // collections
    'breadcrumb',
    'form',
    'grid',
    'menu',
    'message',
    'table',

    // views
    'ad',
    'card',
    'comment',
    'feed',
    'item',
    'statistic',

    // modules
    'accordion',
    'calendar',
    'checkbox',
    'dimmer',
    'dropdown',
    'embed',
    'modal',
    'nag',
    'popup',
    'progress',
    'slider',
    'rating',
    'search',
    'shape',
    'sidebar',
    'sticky',
    'tab',
    'toast',
    'transition',

    // behaviors
    'api',
    'form',
    'state',
    'visibility'
  ],

  // whether to load admin tasks
  admin: false,

  // globs used for matching file patterns
  globs      : {
    ignored    : '!(*.min|*.map|*.rtl)',
    ignoredRTL : '!(*.min|*.map)'
  }

};
