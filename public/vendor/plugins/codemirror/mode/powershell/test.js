// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: http://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "powershell");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  MT('comment', '[number 1][comment # A]');
  MT('comment_multiline', '[number 1][comment <#]',
    '[comment ABC]',
  '[comment #>][number 2]');

  [
    '0', '1234',
    '12kb', '12mb', '12Gb', '12Tb', '12PB', '12L', '12D', '12lkb', '12dtb',
    '1.234', '1.234e56', '1.', '1.e2', '.2', '.2e34',
    '1.2MB', '1.kb', '.1dTB', '1.e1gb', '.2', '.2e34',
    '0x1', '0xabcdef', '0x3tb', '0xelmb'
  ].forEach(function(number) {
    MT("number_" + number, "[number " + number + "]");
  });

  MT('string_literal_escaping', "[string 'a''']");
  MT('string_literal_variable', "[string 'a $x']");
  MT('string_escaping_1', '[string "a `""]');
  MT('string_escaping_2', '[string "a """]');
  MT('string_variable_escaping', '[string "a `$x"]');
  MT('string_variable', '[string "a ][variable-2 $x][string  b"]');
  MT('string_variable_spaces', '[string "a ][variable-2 ${x y}][string  b"]');
  MT('string_expression', '[string "a ][punctuation $(][variable-2 $x][operator +][number 3][punctuation )][string  b"]');
  MT('string_expression_nested', '[string "A][punctuation $(][string "a][punctuation $(][string "w"][punctuation )][string b"][punctuation )][string B"]');

  MT('string_heredoc', '[string @"]',
    '[string abc]',
  '[string "@]');
  MT('string_heredoc_quotes', '[string @"]',
    '[string abc "\']',
  '[string "@]');
  MT('string_heredoc_variable', '[string @"]',
    '[string a ][variable-2 $x][string  b]',
  '[string "@]');
  MT('string_heredoc_nested_string', '[string @"]',
    '[string a][punctuation $(][string "w"][punctuation )][string b]',
  '[string "@]');
  MT('string_heredoc_literal_quotes', "[string @']",
    '[string abc "\']',
  "[string '@]");

  MT('array', "[punctuation @(][string 'a'][punctuation ,][string 'b'][punctuation )]");
  MT('hash', "[punctuation @{][string 'key'][operator :][string 'value'][punctuation }]");

  MT('variable', "[variable-2 $test]");
  MT('variable_global',  "[variable-2 $global:test]");
  MT('variable_spaces',  "[variable-2 ${test test}]");
  MT('operator_splat',   "[variable-2 @x]");
  MT('variable_builtin', "[builtin $ErrorActionPreference]");
  MT('variable_builtin_symbols', "[builtin $$]");

  MT('operator', "[operator +]");
  MT('operator_unary', "[operator +][number 3]");
  MT('operator_long', "[operator -match]");

  [
    '(', ')', '[[', ']]', '{', '}', ',', '`', ';', '.'
  ].forEach(function(punctuation) {
    MT("punctuation_" + punctuation.replace(/^[\[\]]/,''), "[punctuation " + punctuation + "]");
  });

  MT('keyword', "[keyword if]");

  MT('call_builtin', "[builtin Get-ChildItem]");
})();
