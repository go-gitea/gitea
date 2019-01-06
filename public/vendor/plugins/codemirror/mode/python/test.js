// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: http://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 4},
              {name: "python",
               version: 3,
               singleLineStringErrors: false});
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  // Error, because "foobarhello" is neither a known type or property, but
  // property was expected (after "and"), and it should be in parentheses.
  MT("decoratorStartOfLine",
     "[meta @dec]",
     "[keyword def] [def function]():",
     "    [keyword pass]");

  MT("decoratorIndented",
     "[keyword class] [def Foo]:",
     "    [meta @dec]",
     "    [keyword def] [def function]():",
     "        [keyword pass]");

  MT("matmulWithSpace:", "[variable a] [operator @] [variable b]");
  MT("matmulWithoutSpace:", "[variable a][operator @][variable b]");
  MT("matmulSpaceBefore:", "[variable a] [operator @][variable b]");

  MT("fValidStringPrefix", "[string f'this is a {formatted} string']");
  MT("uValidStringPrefix", "[string u'this is an unicode string']");
})();
