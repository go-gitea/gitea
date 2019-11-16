// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

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
     "  [keyword get] [property prop]() { [keyword return] [number 24]; }",
     "  [property constructor]([def x], [def y]) {",
     "    [keyword super]([string 'something']);",
     "    [keyword this].[property x] [operator =] [variable-2 x];",
     "  }",
     "}");

  MT("anonymous_class_expression",
     "[keyword const] [def Adder] [operator =] [keyword class] [keyword extends] [variable Arithmetic] {",
     "  [property add]([def a], [def b]) {}",
     "};");

  MT("named_class_expression",
     "[keyword const] [def Subber] [operator =] [keyword class] [def Subtract] {",
     "  [property sub]([def a], [def b]) {}",
     "};");

  MT("class_async_method",
     "[keyword class] [def Foo] {",
     "  [property sayName1]() {}",
     "  [keyword async] [property sayName2]() {}",
     "}");

  MT("import",
     "[keyword function] [def foo]() {",
     "  [keyword import] [def $] [keyword from] [string 'jquery'];",
     "  [keyword import] { [def encrypt], [def decrypt] } [keyword from] [string 'crypto'];",
     "}");

  MT("import_trailing_comma",
     "[keyword import] {[def foo], [def bar],} [keyword from] [string 'baz']")

  MT("import_dynamic",
     "[keyword import]([string 'baz']).[property then]")

  MT("import_dynamic",
     "[keyword const] [def t] [operator =] [keyword import]([string 'baz']).[property then]")

  MT("const",
     "[keyword function] [def f]() {",
     "  [keyword const] [[ [def a], [def b] ]] [operator =] [[ [number 1], [number 2] ]];",
     "}");

  MT("for/of",
     "[keyword for]([keyword let] [def of] [keyword of] [variable something]) {}");

  MT("for await",
     "[keyword for] [keyword await]([keyword let] [def of] [keyword of] [variable something]) {}");

  MT("generator",
     "[keyword function*] [def repeat]([def n]) {",
     "  [keyword for]([keyword var] [def i] [operator =] [number 0]; [variable-2 i] [operator <] [variable-2 n]; [operator ++][variable-2 i])",
     "    [keyword yield] [variable-2 i];",
     "}");

  MT("let_scoping",
     "[keyword function] [def scoped]([def n]) {",
     "  { [keyword var] [def i]; } [variable-2 i];",
     "  { [keyword let] [def j]; [variable-2 j]; } [variable j];",
     "  [keyword if] ([atom true]) { [keyword const] [def k]; [variable-2 k]; } [variable k];",
     "}");

  MT("switch_scoping",
     "[keyword switch] ([variable x]) {",
     "  [keyword default]:",
     "    [keyword let] [def j];",
     "    [keyword return] [variable-2 j]",
     "}",
     "[variable j];")

  MT("leaving_scope",
     "[keyword function] [def a]() {",
     "  {",
     "    [keyword const] [def x] [operator =] [number 1]",
     "    [keyword if] ([atom true]) {",
     "      [keyword let] [def y] [operator =] [number 2]",
     "      [keyword var] [def z] [operator =] [number 3]",
     "      [variable console].[property log]([variable-2 x], [variable-2 y], [variable-2 z])",
     "    }",
     "    [variable console].[property log]([variable-2 x], [variable y], [variable-2 z])",
     "  }",
     "  [variable console].[property log]([variable x], [variable y], [variable-2 z])",
     "}")

  MT("quotedStringAddition",
     "[keyword let] [def f] [operator =] [variable a] [operator +] [string 'fatarrow'] [operator +] [variable c];");

  MT("quotedFatArrow",
     "[keyword let] [def f] [operator =] [variable a] [operator +] [string '=>'] [operator +] [variable c];");

  MT("fatArrow",
     "[variable array].[property filter]([def a] [operator =>] [variable-2 a] [operator +] [number 1]);",
     "[variable a];", // No longer in scope
     "[keyword let] [def f] [operator =] ([[ [def a], [def b] ]], [def c]) [operator =>] [variable-2 a] [operator +] [variable-2 c];",
     "[variable c];");

  MT("fatArrow_stringDefault",
     "([def a], [def b] [operator =] [string 'x\\'y']) [operator =>] [variable-2 a] [operator +] [variable-2 b]")

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

  MT("indent_semicolonless_if",
     "[keyword function] [def foo]() {",
     "  [keyword if] ([variable x])",
     "    [variable foo]()",
     "}")

  MT("indent_semicolonless_if_with_statement",
     "[keyword function] [def foo]() {",
     "  [keyword if] ([variable x])",
     "    [variable foo]()",
     "  [variable bar]()",
     "}")

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

  MT(
    "param_destructuring",
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

  MT("async",
     "[keyword async] [keyword function] [def foo]([def args]) { [keyword return] [atom true]; }");

  MT("async_assignment",
     "[keyword const] [def foo] [operator =] [keyword async] [keyword function] ([def args]) { [keyword return] [atom true]; };");

  MT("async_object",
     "[keyword let] [def obj] [operator =] { [property async]: [atom false] };");

  // async be highlighet as keyword and foo as def, but it requires potentially expensive look-ahead. See #4173
  MT("async_object_function",
     "[keyword let] [def obj] [operator =] { [property async] [property foo]([def args]) { [keyword return] [atom true]; } };");

  MT("async_object_properties",
     "[keyword let] [def obj] [operator =] {",
     "  [property prop1]: [keyword async] [keyword function] ([def args]) { [keyword return] [atom true]; },",
     "  [property prop2]: [keyword async] [keyword function] ([def args]) { [keyword return] [atom true]; },",
     "  [property prop3]: [keyword async] [keyword function] [def prop3]([def args]) { [keyword return] [atom true]; },",
     "};");

  MT("async_arrow",
     "[keyword const] [def foo] [operator =] [keyword async] ([def args]) [operator =>] { [keyword return] [atom true]; };");

  MT("async_jquery",
     "[variable $].[property ajax]({",
     "  [property url]: [variable url],",
     "  [property async]: [atom true],",
     "  [property method]: [string 'GET']",
     "});");

  MT("async_variable",
     "[keyword const] [def async] [operator =] {[property a]: [number 1]};",
     "[keyword const] [def foo] [operator =] [string-2 `bar ${][variable async].[property a][string-2 }`];")

  MT("bigint", "[number 1n] [operator +] [number 0x1afn] [operator +] [number 0o064n] [operator +] [number 0b100n];")

  MT("async_comment",
     "[keyword async] [comment /**/] [keyword function] [def foo]([def args]) { [keyword return] [atom true]; }");

  MT("indent_switch",
     "[keyword switch] ([variable x]) {",
     "  [keyword default]:",
     "    [keyword return] [number 2]",
     "}")

  MT("regexp_corner_case",
     "[operator +]{} [operator /] [atom undefined];",
     "[[[meta ...][string-2 /\\//] ]];",
     "[keyword void] [string-2 /\\//];",
     "[keyword do] [string-2 /\\//]; [keyword while] ([number 0]);",
     "[keyword if] ([number 0]) {} [keyword else] [string-2 /\\//];",
     "[string-2 `${][variable async][operator ++][string-2 }//`];",
     "[string-2 `${]{} [operator /] [string-2 /\\//}`];")

  MT("return_eol",
     "[keyword return]",
     "{} [string-2 /5/]")

  MT("numeric separator",
     "[number 123_456];",
     "[number 0xdead_c0de];",
     "[number 0o123_456];",
     "[number 0b1101_1101];",
     "[number .123_456e0_1];",
     "[number 1E+123_456];",
     "[number 12_34_56n];")

  MT("underscore property",
     "[variable something].[property _property];",
     "[variable something].[property _123];",
     "[variable something].[property _for];",
     "[variable _for];",
     "[variable _123];")

  var ts_mode = CodeMirror.getMode({indentUnit: 2}, "application/typescript")
  function TS(name) {
    test.mode(name, ts_mode, Array.prototype.slice.call(arguments, 1))
  }

  TS("typescript_extend_type",
     "[keyword class] [def Foo] [keyword extends] [type Some][operator <][type Type][operator >] {}")

  TS("typescript_arrow_type",
     "[keyword let] [def x]: ([variable arg]: [type Type]) [operator =>] [type ReturnType]")

  TS("typescript_class",
     "[keyword class] [def Foo] {",
     "  [keyword public] [keyword static] [property main]() {}",
     "  [keyword private] [property _foo]: [type string];",
     "}")

  TS("typescript_literal_types",
     "[keyword import] [keyword *] [keyword as] [def Sequelize] [keyword from] [string 'sequelize'];",
     "[keyword interface] [def MyAttributes] {",
     "  [property truthy]: [string 'true'] [operator |] [number 1] [operator |] [atom true];",
     "  [property falsy]: [string 'false'] [operator |] [number 0] [operator |] [atom false];",
     "}",
     "[keyword interface] [def MyInstance] [keyword extends] [type Sequelize].[type Instance] [operator <] [type MyAttributes] [operator >] {",
     "  [property rawAttributes]: [type MyAttributes];",
     "  [property truthy]: [string 'true'] [operator |] [number 1] [operator |] [atom true];",
     "  [property falsy]: [string 'false'] [operator |] [number 0] [operator |] [atom false];",
     "}")

  TS("typescript_extend_operators",
     "[keyword export] [keyword interface] [def UserModel] [keyword extends]",
     "  [type Sequelize].[type Model] [operator <] [type UserInstance], [type UserAttributes] [operator >] {",
     "    [property findById]: (",
     "    [variable userId]: [type number]",
     "    ) [operator =>] [type Promise] [operator <] [type Array] [operator <] { [property id], [property name] } [operator >>];",
     "    [property updateById]: (",
     "    [variable userId]: [type number],",
     "    [variable isActive]: [type boolean]",
     "    ) [operator =>] [type Promise] [operator <] [type AccountHolderNotificationPreferenceInstance] [operator >];",
     "  }")

  TS("typescript_interface_with_const",
     "[keyword const] [def hello]: {",
     "  [property prop1][operator ?]: [type string];",
     "  [property prop2][operator ?]: [type string];",
     "} [operator =] {};")

  TS("typescript_double_extend",
     "[keyword export] [keyword interface] [def UserAttributes] {",
     "  [property id][operator ?]: [type number];",
     "  [property createdAt][operator ?]: [type Date];",
     "}",
     "[keyword export] [keyword interface] [def UserInstance] [keyword extends] [type Sequelize].[type Instance][operator <][type UserAttributes][operator >], [type UserAttributes] {",
     "  [property id]: [type number];",
     "  [property createdAt]: [type Date];",
     "}");

  TS("typescript_index_signature",
     "[keyword interface] [def A] {",
     "  [[ [variable prop]: [type string] ]]: [type any];",
     "  [property prop1]: [type any];",
     "}");

  TS("typescript_generic_class",
     "[keyword class] [def Foo][operator <][type T][operator >] {",
     "  [property bar]() {}",
     "  [property foo](): [type Foo] {}",
     "}")

  TS("typescript_type_when_keyword",
     "[keyword export] [keyword type] [type AB] [operator =] [type A] [operator |] [type B];",
     "[keyword type] [type Flags] [operator =] {",
     "  [property p1]: [type string];",
     "  [property p2]: [type boolean];",
     "};")

  TS("typescript_type_when_not_keyword",
     "[keyword class] [def HasType] {",
     "  [property type]: [type string];",
     "  [property constructor]([def type]: [type string]) {",
     "    [keyword this].[property type] [operator =] [variable-2 type];",
     "  }",
     "  [property setType]({ [def type] }: { [property type]: [type string]; }) {",
     "    [keyword this].[property type] [operator =] [variable-2 type];",
     "  }",
     "}")

  TS("typescript_function_generics",
     "[keyword function] [def a]() {}",
     "[keyword function] [def b][operator <][type IA] [keyword extends] [type object], [type IB] [keyword extends] [type object][operator >]() {}",
     "[keyword function] [def c]() {}")

  TS("typescript_complex_return_type",
     "[keyword function] [def A]() {",
     "  [keyword return] [keyword this].[property property];",
     "}",
     "[keyword function] [def B](): [type Promise][operator <]{ [[ [variable key]: [type string] ]]: [type any] } [operator |] [atom null][operator >] {",
     "  [keyword return] [keyword this].[property property];",
     "}")

  TS("typescript_complex_type_casting",
     "[keyword const] [def giftpay] [operator =] [variable config].[property get]([string 'giftpay']) [keyword as] { [[ [variable platformUuid]: [type string] ]]: { [property version]: [type number]; [property apiCode]: [type string]; } };")

  TS("typescript_keyof",
     "[keyword function] [def x][operator <][type T] [keyword extends] [keyword keyof] [type X][operator >]([def a]: [type T]) {",
     "  [keyword return]")

  TS("typescript_new_typeargs",
     "[keyword let] [def x] [operator =] [keyword new] [variable Map][operator <][type string], [type Date][operator >]([string-2 `foo${][variable bar][string-2 }`])")

  TS("modifiers",
     "[keyword class] [def Foo] {",
     "  [keyword public] [keyword abstract] [property bar]() {}",
     "  [property constructor]([keyword readonly] [keyword private] [def x]) {}",
     "}")

  TS("arrow prop",
     "({[property a]: [def p] [operator =>] [variable-2 p]})")

  TS("generic in function call",
     "[keyword this].[property a][operator <][type Type][operator >]([variable foo]);",
     "[keyword this].[property a][operator <][variable Type][operator >][variable foo];")

  TS("type guard",
     "[keyword class] [def Appler] {",
     "  [keyword static] [property assertApple]([def fruit]: [type Fruit]): [variable-2 fruit] [keyword is] [type Apple] {",
     "    [keyword if] ([operator !]([variable-2 fruit] [keyword instanceof] [variable Apple]))",
     "      [keyword throw] [keyword new] [variable Error]();",
     "  }",
     "}")

  TS("type as variable",
     "[variable type] [operator =] [variable x] [keyword as] [type Bar];");

  TS("enum body",
     "[keyword export] [keyword const] [keyword enum] [def CodeInspectionResultType] {",
     "  [def ERROR] [operator =] [string 'problem_type_error'],",
     "  [def WARNING] [operator =] [string 'problem_type_warning'],",
     "  [def META],",
     "}")

  TS("parenthesized type",
     "[keyword class] [def Foo] {",
     "  [property x] [operator =] [keyword new] [variable A][operator <][type B], [type string][operator |](() [operator =>] [type void])[operator >]();",
     "  [keyword private] [property bar]();",
     "}")

  TS("abstract class",
     "[keyword export] [keyword abstract] [keyword class] [def Foo] {}")

  TS("interface without semicolons",
     "[keyword interface] [def Foo] {",
     "  [property greet]([def x]: [type int]): [type blah]",
     "  [property bar]: [type void]",
     "}")

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
