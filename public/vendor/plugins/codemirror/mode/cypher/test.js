// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({tabSize: 4, indentUnit: 2}, "cypher");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  MT("unbalancedDoubledQuotedString",
      "[string \"a'b\"][variable c]");

  MT("unbalancedSingleQuotedString",
      "[string 'a\"b'][variable c]");

  MT("doubleQuotedString",
      "[string \"a\"][variable b]");

  MT("singleQuotedString",
      "[string 'a'][variable b]");

  MT("single attribute (with content)",
      "[node {][atom a:][string 'a'][node }]");

  MT("multiple attribute, singleQuotedString (with content)",
      "[node {][atom a:][string 'a'][node ,][atom b:][string 'b'][node }]");

  MT("multiple attribute, doubleQuotedString (with content)",
      "[node {][atom a:][string \"a\"][node ,][atom b:][string \"b\"][node }]");

  MT("single attribute (without content)",
      "[node {][atom a:][string 'a'][node }]");

  MT("multiple attribute, singleQuotedString (without content)",
      "[node {][atom a:][string ''][node ,][atom b:][string ''][node }]");

  MT("multiple attribute, doubleQuotedString (without content)",
      "[node {][atom a:][string \"\"][node ,][atom b:][string \"\"][node }]");
 })();
