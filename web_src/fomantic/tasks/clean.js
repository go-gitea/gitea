/*******************************
          Clean Task
*******************************/

var
  del    = require('del'),
  config = require('./config/user'),
  tasks  = require('./config/tasks')
;

// cleans distribution files
module.exports = function() {
  return del([config.paths.clean], tasks.settings.del);
};