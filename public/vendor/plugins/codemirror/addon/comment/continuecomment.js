// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function(mod) {
  if (typeof exports == "object" && typeof module == "object") // CommonJS
    mod(require("../../lib/codemirror"));
  else if (typeof define == "function" && define.amd) // AMD
    define(["../../lib/codemirror"], mod);
  else // Plain browser env
    mod(CodeMirror);
})(function(CodeMirror) {
  function continueComment(cm) {
    if (cm.getOption("disableInput")) return CodeMirror.Pass;
    var ranges = cm.listSelections(), mode, inserts = [];
    for (var i = 0; i < ranges.length; i++) {
      var pos = ranges[i].head
      if (!/\bcomment\b/.test(cm.getTokenTypeAt(pos))) return CodeMirror.Pass;
      var modeHere = cm.getModeAt(pos)
      if (!mode) mode = modeHere;
      else if (mode != modeHere) return CodeMirror.Pass;

      var insert = null;
      if (mode.blockCommentStart && mode.blockCommentContinue) {
        var line = cm.getLine(pos.line).slice(0, pos.ch)
        var end = line.lastIndexOf(mode.blockCommentEnd), found
        if (end != -1 && end == pos.ch - mode.blockCommentEnd.length) {
          // Comment ended, don't continue it
        } else if ((found = line.lastIndexOf(mode.blockCommentStart)) > -1 && found > end) {
          insert = line.slice(0, found)
          if (/\S/.test(insert)) {
            insert = ""
            for (var j = 0; j < found; ++j) insert += " "
          }
        } else if ((found = line.indexOf(mode.blockCommentContinue)) > -1 && !/\S/.test(line.slice(0, found))) {
          insert = line.slice(0, found)
        }
        if (insert != null) insert += mode.blockCommentContinue
      }
      if (insert == null && mode.lineComment && continueLineCommentEnabled(cm)) {
        var line = cm.getLine(pos.line), found = line.indexOf(mode.lineComment);
        if (found > -1) {
          insert = line.slice(0, found);
          if (/\S/.test(insert)) insert = null;
          else insert += mode.lineComment + line.slice(found + mode.lineComment.length).match(/^\s*/)[0];
        }
      }
      if (insert == null) return CodeMirror.Pass;
      inserts[i] = "\n" + insert;
    }

    cm.operation(function() {
      for (var i = ranges.length - 1; i >= 0; i--)
        cm.replaceRange(inserts[i], ranges[i].from(), ranges[i].to(), "+insert");
    });
  }

  function continueLineCommentEnabled(cm) {
    var opt = cm.getOption("continueComments");
    if (opt && typeof opt == "object")
      return opt.continueLineComment !== false;
    return true;
  }

  CodeMirror.defineOption("continueComments", null, function(cm, val, prev) {
    if (prev && prev != CodeMirror.Init)
      cm.removeKeyMap("continueComment");
    if (val) {
      var key = "Enter";
      if (typeof val == "string")
        key = val;
      else if (typeof val == "object" && val.key)
        key = val.key;
      var map = {name: "continueComment"};
      map[key] = continueComment;
      cm.addKeyMap(map);
    }
  });
});
