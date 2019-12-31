## How to use

These are collections of tasks that are imported together.

To import them into gulp:
```javascript
var
  gulp    = require('gulp'),
  // modified to point to semantic folder
  install = require('tasks/collections/install')
;
gulp = install(gulp);

// tasks are now injected and ready to be used
gulp.start('install');
```