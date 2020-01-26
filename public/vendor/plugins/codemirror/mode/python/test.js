// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

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
  var before_equal_sign = ["+", "-", "*", "/", "=", "!", ">", "<"];
  for (var i = 0; i < before_equal_sign.length; ++i) {
    var c = before_equal_sign[i]
    MT("before_equal_sign_" + c, "[variable a] [operator " + c + "=] [variable b]");
  }

  MT("fValidStringPrefix", "[string f'this is a]{[variable formatted]}[string string']");
  MT("fValidExpressioninFString", "[string f'expression ]{[number 100][operator *][number 5]}[string string']");
  MT("fInvalidFString", "[error f'this is wrong}]");
  MT("fNestedFString", "[string f'expression ]{[number 100] [operator +] [string f'inner]{[number 5]}[string ']}[string string']");
  MT("uValidStringPrefix", "[string u'this is an unicode string']");

  MT("nestedString", "[string f']{[variable b][[ [string \"c\"] ]]}[string f'] [comment # oops]")

  MT("bracesInFString", "[string f']{[variable x] [operator +] {}}[string !']")

  MT("nestedFString", "[string f']{[variable b][[ [string f\"c\"] ]]}[string f'] [comment # oops]")
})();
