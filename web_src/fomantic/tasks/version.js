/*******************************
          Version Task
*******************************/

var
  release = require('./config/project/release')
;

module.exports = function(callback) {
  console.log(release.title + ' ' + release.version);
  callback();
};