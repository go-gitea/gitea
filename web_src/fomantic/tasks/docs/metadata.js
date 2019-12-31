
/*******************************
           Summarize Docs
*******************************/

var
  // node dependencies
  console      = require('better-console'),
  fs           = require('fs'),
  YAML         = require('yamljs')
;

var data = {};

/**
 * Test for prefix in string.
 * @param {string} str
 * @param {string} prefix
 * @return {boolean}
 */
function startsWith(str, prefix) {
  return str.indexOf(prefix) === 0;
};

function inArray(needle, haystack) {
  var length = haystack.length;
  for(var i = 0; i < length; i++) {
      if(haystack[i] == needle) return true;
  }
  return false;
}

/**
 * Parses a file for metadata and stores result in data object.
 * @param {File} file - object provided by map-stream.
 * @param {function(?,File)} - callback provided by map-stream to
 * reply when done.
 */
function parser(file, callback) {
  // file exit conditions
  if(file.isNull()) {
    return callback(null, file); // pass along
  }

  if(file.isStream()) {
    return callback(new Error('Streaming not supported'));
  }

  try {

    var
      /** @type {string} */
      text     = String(file.contents.toString('utf8')),
      lines    = text.split('\n'),
      filename = file.path.substring(0, file.path.length - 4),
      key      = 'server/documents',
      position = filename.indexOf(key)
    ;

    // exit conditions
    if(!lines) {
      return;
    }
    if(position < 0) {
      return callback(null, file);
    }

    filename = filename.substring(position + key.length + 1, filename.length);

    var
      lineCount = lines.length,
      active    = false,
      yaml      = [],
      categories = [
        'UI Element',
        'UI Global',
        'UI Collection',
        'UI View',
        'UI Module',
        'UI Behavior'
      ],
      index,
      meta,
      line
    ;

    for(index = 0; index < lineCount; index++) {

      line = lines[index];

      // Wait for metadata block to begin
      if(!active) {
        if(startsWith(line, '---')) {
          active = true;
        }
        continue;
      }
      // End of metadata block, stop parsing.
      if(startsWith(line, '---')) {
        break;
      }
      yaml.push(line);
    }


    // Parse yaml.
    meta = YAML.parse(yaml.join('\n'));
    if(meta && meta.type && meta.title && inArray(meta.type, categories) ) {
      meta.category = meta.type;
      meta.filename = filename;
      meta.url      = '/' + filename;
      meta.title    = meta.title;
      // Primary key will by filepath
      data[meta.element] = meta;
    }
    else {
      // skip
      // console.log(meta);
    }


  }

  catch(error) {
    console.log(error, filename);
  }

  callback(null, file);

}

/**
 * Export function expected by map-stream.
 */
module.exports = {
  result : data,
  parser : parser
};
