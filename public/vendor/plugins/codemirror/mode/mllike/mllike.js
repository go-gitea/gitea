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
"use strict";

CodeMirror.defineMode('mllike', function(_config, parserConfig) {
  var words = {
    'as': 'keyword',
    'do': 'keyword',
    'else': 'keyword',
    'end': 'keyword',
    'exception': 'keyword',
    'fun': 'keyword',
    'functor': 'keyword',
    'if': 'keyword',
    'in': 'keyword',
    'include': 'keyword',
    'let': 'keyword',
    'of': 'keyword',
    'open': 'keyword',
    'rec': 'keyword',
    'struct': 'keyword',
    'then': 'keyword',
    'type': 'keyword',
    'val': 'keyword',
    'while': 'keyword',
    'with': 'keyword'
  };

  var extraWords = parserConfig.extraWords || {};
  for (var prop in extraWords) {
    if (extraWords.hasOwnProperty(prop)) {
      words[prop] = parserConfig.extraWords[prop];
    }
  }
  var hintWords = [];
  for (var k in words) { hintWords.push(k); }
  CodeMirror.registerHelper("hintWords", "mllike", hintWords);

  function tokenBase(stream, state) {
    var ch = stream.next();

    if (ch === '"') {
      state.tokenize = tokenString;
      return state.tokenize(stream, state);
    }
    if (ch === '{') {
      if (stream.eat('|')) {
        state.longString = true;
        state.tokenize = tokenLongString;
        return state.tokenize(stream, state);
      }
    }
    if (ch === '(') {
      if (stream.eat('*')) {
        state.commentLevel++;
        state.tokenize = tokenComment;
        return state.tokenize(stream, state);
      }
    }
    if (ch === '~' || ch === '?') {
      stream.eatWhile(/\w/);
      return 'variable-2';
    }
    if (ch === '`') {
      stream.eatWhile(/\w/);
      return 'quote';
    }
    if (ch === '/' && parserConfig.slashComments && stream.eat('/')) {
      stream.skipToEnd();
      return 'comment';
    }
    if (/\d/.test(ch)) {
      if (ch === '0' && stream.eat(/[bB]/)) {
        stream.eatWhile(/[01]/);
      } if (ch === '0' && stream.eat(/[xX]/)) {
        stream.eatWhile(/[0-9a-fA-F]/)
      } if (ch === '0' && stream.eat(/[oO]/)) {
        stream.eatWhile(/[0-7]/);
      } else {
        stream.eatWhile(/[\d_]/);
        if (stream.eat('.')) {
          stream.eatWhile(/[\d]/);
        }
        if (stream.eat(/[eE]/)) {
          stream.eatWhile(/[\d\-+]/);
        }
      }
      return 'number';
    }
    if ( /[+\-*&%=<>!?|@\.~:]/.test(ch)) {
      return 'operator';
    }
    if (/[\w\xa1-\uffff]/.test(ch)) {
      stream.eatWhile(/[\w\xa1-\uffff]/);
      var cur = stream.current();
      return words.hasOwnProperty(cur) ? words[cur] : 'variable';
    }
    return null
  }

  function tokenString(stream, state) {
    var next, end = false, escaped = false;
    while ((next = stream.next()) != null) {
      if (next === '"' && !escaped) {
        end = true;
        break;
      }
      escaped = !escaped && next === '\\';
    }
    if (end && !escaped) {
      state.tokenize = tokenBase;
    }
    return 'string';
  };

  function tokenComment(stream, state) {
    var prev, next;
    while(state.commentLevel > 0 && (next = stream.next()) != null) {
      if (prev === '(' && next === '*') state.commentLevel++;
      if (prev === '*' && next === ')') state.commentLevel--;
      prev = next;
    }
    if (state.commentLevel <= 0) {
      state.tokenize = tokenBase;
    }
    return 'comment';
  }

  function tokenLongString(stream, state) {
    var prev, next;
    while (state.longString && (next = stream.next()) != null) {
      if (prev === '|' && next === '}') state.longString = false;
      prev = next;
    }
    if (!state.longString) {
      state.tokenize = tokenBase;
    }
    return 'string';
  }

  return {
    startState: function() {return {tokenize: tokenBase, commentLevel: 0, longString: false};},
    token: function(stream, state) {
      if (stream.eatSpace()) return null;
      return state.tokenize(stream, state);
    },

    blockCommentStart: "(*",
    blockCommentEnd: "*)",
    lineComment: parserConfig.slashComments ? "//" : null
  };
});

CodeMirror.defineMIME('text/x-ocaml', {
  name: 'mllike',
  extraWords: {
    'and': 'keyword',
    'assert': 'keyword',
    'begin': 'keyword',
    'class': 'keyword',
    'constraint': 'keyword',
    'done': 'keyword',
    'downto': 'keyword',
    'external': 'keyword',
    'function': 'keyword',
    'initializer': 'keyword',
    'lazy': 'keyword',
    'match': 'keyword',
    'method': 'keyword',
    'module': 'keyword',
    'mutable': 'keyword',
    'new': 'keyword',
    'nonrec': 'keyword',
    'object': 'keyword',
    'private': 'keyword',
    'sig': 'keyword',
    'to': 'keyword',
    'try': 'keyword',
    'value': 'keyword',
    'virtual': 'keyword',
    'when': 'keyword',

    // builtins
    'raise': 'builtin',
    'failwith': 'builtin',
    'true': 'builtin',
    'false': 'builtin',

    // Pervasives builtins
    'asr': 'builtin',
    'land': 'builtin',
    'lor': 'builtin',
    'lsl': 'builtin',
    'lsr': 'builtin',
    'lxor': 'builtin',
    'mod': 'builtin',
    'or': 'builtin',

    // More Pervasives
    'raise_notrace': 'builtin',
    'trace': 'builtin',
    'exit': 'builtin',
    'print_string': 'builtin',
    'print_endline': 'builtin',

     'int': 'type',
     'float': 'type',
     'bool': 'type',
     'char': 'type',
     'string': 'type',
     'unit': 'type',

     // Modules
     'List': 'builtin'
  }
});

CodeMirror.defineMIME('text/x-fsharp', {
  name: 'mllike',
  extraWords: {
    'abstract': 'keyword',
    'assert': 'keyword',
    'base': 'keyword',
    'begin': 'keyword',
    'class': 'keyword',
    'default': 'keyword',
    'delegate': 'keyword',
    'do!': 'keyword',
    'done': 'keyword',
    'downcast': 'keyword',
    'downto': 'keyword',
    'elif': 'keyword',
    'extern': 'keyword',
    'finally': 'keyword',
    'for': 'keyword',
    'function': 'keyword',
    'global': 'keyword',
    'inherit': 'keyword',
    'inline': 'keyword',
    'interface': 'keyword',
    'internal': 'keyword',
    'lazy': 'keyword',
    'let!': 'keyword',
    'match': 'keyword',
    'member': 'keyword',
    'module': 'keyword',
    'mutable': 'keyword',
    'namespace': 'keyword',
    'new': 'keyword',
    'null': 'keyword',
    'override': 'keyword',
    'private': 'keyword',
    'public': 'keyword',
    'return!': 'keyword',
    'return': 'keyword',
    'select': 'keyword',
    'static': 'keyword',
    'to': 'keyword',
    'try': 'keyword',
    'upcast': 'keyword',
    'use!': 'keyword',
    'use': 'keyword',
    'void': 'keyword',
    'when': 'keyword',
    'yield!': 'keyword',
    'yield': 'keyword',

    // Reserved words
    'atomic': 'keyword',
    'break': 'keyword',
    'checked': 'keyword',
    'component': 'keyword',
    'const': 'keyword',
    'constraint': 'keyword',
    'constructor': 'keyword',
    'continue': 'keyword',
    'eager': 'keyword',
    'event': 'keyword',
    'external': 'keyword',
    'fixed': 'keyword',
    'method': 'keyword',
    'mixin': 'keyword',
    'object': 'keyword',
    'parallel': 'keyword',
    'process': 'keyword',
    'protected': 'keyword',
    'pure': 'keyword',
    'sealed': 'keyword',
    'tailcall': 'keyword',
    'trait': 'keyword',
    'virtual': 'keyword',
    'volatile': 'keyword',

    // builtins
    'List': 'builtin',
    'Seq': 'builtin',
    'Map': 'builtin',
    'Set': 'builtin',
    'Option': 'builtin',
    'int': 'builtin',
    'string': 'builtin',
    'not': 'builtin',
    'true': 'builtin',
    'false': 'builtin',

    'raise': 'builtin',
    'failwith': 'builtin'
  },
  slashComments: true
});


CodeMirror.defineMIME('text/x-sml', {
  name: 'mllike',
  extraWords: {
    'abstype': 'keyword',
    'and': 'keyword',
    'andalso': 'keyword',
    'case': 'keyword',
    'datatype': 'keyword',
    'fn': 'keyword',
    'handle': 'keyword',
    'infix': 'keyword',
    'infixr': 'keyword',
    'local': 'keyword',
    'nonfix': 'keyword',
    'op': 'keyword',
    'orelse': 'keyword',
    'raise': 'keyword',
    'withtype': 'keyword',
    'eqtype': 'keyword',
    'sharing': 'keyword',
    'sig': 'keyword',
    'signature': 'keyword',
    'structure': 'keyword',
    'where': 'keyword',
    'true': 'keyword',
    'false': 'keyword',

    // types
    'int': 'builtin',
    'real': 'builtin',
    'string': 'builtin',
    'char': 'builtin',
    'bool': 'builtin'
  },
  slashComments: true
});

});
