// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "ruby");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  MT("divide_equal_operator",
     "[variable bar] [operator /=] [variable foo]");

  MT("divide_equal_operator_no_spacing",
     "[variable foo][operator /=][number 42]");

  MT("complex_regexp",
     "[keyword if] [variable cr] [operator =~] [string-2 /(?: \\( #{][tag RE_NOT][string-2 }\\( | #{][tag RE_NOT_PAR_OR][string-2 }* #{][tag RE_OPA_OR][string-2 } )/][variable x]")

  MT("indented_heredoc",
     "[keyword def] [def x]",
     "  [variable y] [operator =] [string <<-FOO]",
     "[string     bar]",
     "[string   FOO]",
     "[keyword end]")
})();
