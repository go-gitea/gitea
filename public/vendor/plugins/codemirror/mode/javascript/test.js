// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: http://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "javascript");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  MT("locals",
     "[keyword function] [def foo]([def a], [def b]) { [keyword var] [def c] [operator =] [number 10]; [keyword return] [variable-2 a] [operator +] [variable-2 c] [operator +] [variable d]; }");

  MT("comma-and-binop",
     "[keyword function](){ [keyword var] [def x] [operator =] [number 1] [operator +] [number 2], [def y]; }");

  MT("destructuring",
     "([keyword function]([def a], [[[def b], [def c] ]]) {",
     "  [keyword let] {[def d], [property foo]: [def c][operator =][number 10], [def x]} [operator =] [variable foo]([variable-2 a]);",
     "  [[[variable-2 c], [variable y] ]] [operator =] [variable-2 c];",
     "})();");

  MT("destructure_trailing_comma",
    "[keyword let] {[def a], [def b],} [operator =] [variable foo];",
    "[keyword let] [def c];"); // Parser still in good state?

  MT("class_body",
     "[keyword class] [def Foo] {",
     "  [property constructor]() {}",
     "  [property sayName]() {",
     "    [keyword return] [string-2 `foo${][variable foo][string-2 }oo`];",
     "  }",
     "}");

  MT("class",
     "[keyword class] [def Point] [keyword extends] [variable SuperThing] {",
     "  [property get] [property prop]() { [keyword return] [number 24]; }",
     "  [property constructor]([def x], [def y]) {",
     "    [keyword super]([string 'something']);",
     "    [keyword this].[property x] [operator =] [variable-2 x];",
     "  }",
     "}");

  MT("import",
     "[keyword function] [def foo]() {",
     "  [keyword import] [def $] [keyword from] [string 'jquery'];",
     "  [keyword import] { [def encrypt], [def decrypt] } [keyword from] [string 'crypto'];",
     "}");

  MT("import_trailing_comma",
     "[keyword import] {[def foo], [def bar],} [keyword from] [string 'baz']")

  MT("const",
     "[keyword function] [def f]() {",
     "  [keyword const] [[ [def a], [def b] ]] [operator =] [[ [number 1], [number 2] ]];",
     "}");

  MT("for/of",
     "[keyword for]([keyword let] [def of] [keyword of] [variable something]) {}");

  MT("generator",
     "[keyword function*] [def repeat]([def n]) {",
     "  [keyword for]([keyword var] [def i] [operator =] [number 0]; [variable-2 i] [operator <] [variable-2 n]; [operator ++][variable-2 i])",
     "    [keyword yield] [variable-2 i];",
     "}");

  MT("quotedStringAddition",
     "[keyword let] [def f] [operator =] [variable a] [operator +] [string 'fatarrow'] [operator +] [variable c];");

  MT("quotedFatArrow",
     "[keyword let] [def f] [operator =] [variable a] [operator +] [string '=>'] [operator +] [variable c];");

  MT("fatArrow",
     "[variable array].[property filter]([def a] [operator =>] [variable-2 a] [operator +] [number 1]);",
     "[variable a];", // No longer in scope
     "[keyword let] [def f] [operator =] ([[ [def a], [def b] ]], [def c]) [operator =>] [variable-2 a] [operator +] [variable-2 c];",
     "[variable c];");

  MT("spread",
     "[keyword function] [def f]([def a], [meta ...][def b]) {",
     "  [variable something]([variable-2 a], [meta ...][variable-2 b]);",
     "}");

  MT("quasi",
     "[variable re][string-2 `fofdlakj${][variable x] [operator +] ([variable re][string-2 `foo`]) [operator +] [number 1][string-2 }fdsa`] [operator +] [number 2]");

  MT("quasi_no_function",
     "[variable x] [operator =] [string-2 `fofdlakj${][variable x] [operator +] [string-2 `foo`] [operator +] [number 1][string-2 }fdsa`] [operator +] [number 2]");

  MT("indent_statement",
     "[keyword var] [def x] [operator =] [number 10]",
     "[variable x] [operator +=] [variable y] [operator +]",
     "  [atom Infinity]",
     "[keyword debugger];");

  MT("indent_if",
     "[keyword if] ([number 1])",
     "  [keyword break];",
     "[keyword else] [keyword if] ([number 2])",
     "  [keyword continue];",
     "[keyword else]",
     "  [number 10];",
     "[keyword if] ([number 1]) {",
     "  [keyword break];",
     "} [keyword else] [keyword if] ([number 2]) {",
     "  [keyword continue];",
     "} [keyword else] {",
     "  [number 10];",
     "}");

  MT("indent_for",
     "[keyword for] ([keyword var] [def i] [operator =] [number 0];",
     "     [variable i] [operator <] [number 100];",
     "     [variable i][operator ++])",
     "  [variable doSomething]([variable i]);",
     "[keyword debugger];");

  MT("indent_c_style",
     "[keyword function] [def foo]()",
     "{",
     "  [keyword debugger];",
     "}");

  MT("indent_else",
     "[keyword for] (;;)",
     "  [keyword if] ([variable foo])",
     "    [keyword if] ([variable bar])",
     "      [number 1];",
     "    [keyword else]",
     "      [number 2];",
     "  [keyword else]",
     "    [number 3];");

  MT("indent_funarg",
     "[variable foo]([number 10000],",
     "    [keyword function]([def a]) {",
     "  [keyword debugger];",
     "};");

  MT("indent_below_if",
     "[keyword for] (;;)",
     "  [keyword if] ([variable foo])",
     "    [number 1];",
     "[number 2];");

  MT("multilinestring",
     "[keyword var] [def x] [operator =] [string 'foo\\]",
     "[string bar'];");

  MT("scary_regexp",
     "[string-2 /foo[[/]]bar/];");

  MT("indent_strange_array",
     "[keyword var] [def x] [operator =] [[",
     "  [number 1],,",
     "  [number 2],",
     "]];",
     "[number 10];");

  MT("param_default",
     "[keyword function] [def foo]([def x] [operator =] [string-2 `foo${][number 10][string-2 }bar`]) {",
     "  [keyword return] [variable-2 x];",
     "}");

  MT("new_target",
     "[keyword function] [def F]([def target]) {",
     "  [keyword if] ([variable-2 target] [operator &&] [keyword new].[keyword target].[property name]) {",
     "    [keyword return] [keyword new]",
     "      .[keyword target];",
     "  }",
     "}");

  var jsonld_mode = CodeMirror.getMode(
    {indentUnit: 2},
    {name: "javascript", jsonld: true}
  );
  function LD(name) {
    test.mode(name, jsonld_mode, Array.prototype.slice.call(arguments, 1));
  }

  LD("json_ld_keywords",
    '{',
    '  [meta "@context"]: {',
    '    [meta "@base"]: [string "http://example.com"],',
    '    [meta "@vocab"]: [string "http://xmlns.com/foaf/0.1/"],',
    '    [property "likesFlavor"]: {',
    '      [meta "@container"]: [meta "@list"]',
    '      [meta "@reverse"]: [string "@beFavoriteOf"]',
    '    },',
    '    [property "nick"]: { [meta "@container"]: [meta "@set"] },',
    '    [property "nick"]: { [meta "@container"]: [meta "@index"] }',
    '  },',
    '  [meta "@graph"]: [[ {',
    '    [meta "@id"]: [string "http://dbpedia.org/resource/John_Lennon"],',
    '    [property "name"]: [string "John Lennon"],',
    '    [property "modified"]: {',
    '      [meta "@value"]: [string "2010-05-29T14:17:39+02:00"],',
    '      [meta "@type"]: [string "http://www.w3.org/2001/XMLSchema#dateTime"]',
    '    }',
    '  } ]]',
    '}');

  LD("json_ld_fake",
    '{',
    '  [property "@fake"]: [string "@fake"],',
    '  [property "@contextual"]: [string "@identifier"],',
    '  [property "user@domain.com"]: [string "@graphical"],',
    '  [property "@ID"]: [string "@@ID"]',
    '}');
})();
