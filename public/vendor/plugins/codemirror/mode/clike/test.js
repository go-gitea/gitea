// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "text/x-c");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  MT("indent",
     "[type void] [def foo]([type void*] [variable a], [type int] [variable b]) {",
     "  [type int] [variable c] [operator =] [variable b] [operator +]",
     "    [number 1];",
     "  [keyword return] [operator *][variable a];",
     "}");

  MT("indent_switch",
     "[keyword switch] ([variable x]) {",
     "  [keyword case] [number 10]:",
     "    [keyword return] [number 20];",
     "  [keyword default]:",
     "    [variable printf]([string \"foo %c\"], [variable x]);",
     "}");

  MT("def",
     "[type void] [def foo]() {}",
     "[keyword struct] [def bar]{}",
     "[keyword enum] [def zot]{}",
     "[keyword union] [def ugh]{}",
     "[type int] [type *][def baz]() {}");

  MT("def_new_line",
     "::[variable std]::[variable SomeTerribleType][operator <][variable T][operator >]",
     "[def SomeLongMethodNameThatDoesntFitIntoOneLine]([keyword const] [variable MyType][operator &] [variable param]) {}")

  MT("double_block",
     "[keyword for] (;;)",
     "  [keyword for] (;;)",
     "    [variable x][operator ++];",
     "[keyword return];");

  MT("preprocessor",
     "[meta #define FOO 3]",
     "[type int] [variable foo];",
     "[meta #define BAR\\]",
     "[meta 4]",
     "[type unsigned] [type int] [variable bar] [operator =] [number 8];",
     "[meta #include <baz> ][comment // comment]")

  MT("c_underscores",
     "[builtin __FOO];",
     "[builtin _Complex];",
     "[builtin __aName];",
     "[variable _aName];");

  MT("c_types",
    "[type int];",
    "[type long];",
    "[type char];",
    "[type short];",
    "[type double];",
    "[type float];",
    "[type unsigned];",
    "[type signed];",
    "[type void];",
    "[type bool];",
    "[type foo_t];",
    "[variable foo_T];",
    "[variable _t];");

  var mode_cpp = CodeMirror.getMode({indentUnit: 2}, "text/x-c++src");
  function MTCPP(name) { test.mode(name, mode_cpp, Array.prototype.slice.call(arguments, 1)); }

  MTCPP("cpp14_literal",
    "[number 10'000];",
    "[number 0b10'000];",
    "[number 0x10'000];",
    "[string '100000'];");

  MTCPP("ctor_dtor",
     "[def Foo::Foo]() {}",
     "[def Foo::~Foo]() {}");

  MTCPP("cpp_underscores",
        "[builtin __FOO];",
        "[builtin _Complex];",
        "[builtin __aName];",
        "[variable _aName];");

  var mode_objc = CodeMirror.getMode({indentUnit: 2}, "text/x-objectivec");
  function MTOBJC(name) { test.mode(name, mode_objc, Array.prototype.slice.call(arguments, 1)); }

  MTOBJC("objc_underscores",
         "[builtin __FOO];",
         "[builtin _Complex];",
         "[builtin __aName];",
         "[variable _aName];");

  MTOBJC("objc_interface",
         "[keyword @interface] [def foo] {",
         "  [type int] [variable bar];",
         "}",
         "[keyword @property] ([keyword atomic], [keyword nullable]) [variable NSString][operator *] [variable a];",
         "[keyword @property] ([keyword nonatomic], [keyword assign]) [type int] [variable b];",
         "[operator -]([type instancetype])[variable initWithFoo]:([type int])[variable a] " +
           "[builtin NS_DESIGNATED_INITIALIZER];",
         "[keyword @end]");

  MTOBJC("objc_implementation",
         "[keyword @implementation] [def foo] {",
         "  [type int] [variable bar];",
         "}",
         "[keyword @property] ([keyword readwrite]) [type SEL] [variable a];",
         "[operator -]([type instancetype])[variable initWithFoo]:([type int])[variable a] {",
         "  [keyword if](([keyword self] [operator =] [[[keyword super] [variable init] ]])) {}",
         "  [keyword return] [keyword self];",
         "}",
         "[keyword @end]");

  MTOBJC("objc_types",
         "[type int];",
         "[type foo_t];",
         "[variable foo_T];",
         "[type id];",
         "[type SEL];",
         "[type instancetype];",
         "[type Class];",
         "[type Protocol];",
         "[type BOOL];"
         );

  var mode_scala = CodeMirror.getMode({indentUnit: 2}, "text/x-scala");
  function MTSCALA(name) { test.mode("scala_" + name, mode_scala, Array.prototype.slice.call(arguments, 1)); }
  MTSCALA("nested_comments",
     "[comment /*]",
     "[comment But wait /* this is a nested comment */ for real]",
     "[comment /**** let * me * show * you ****/]",
     "[comment ///// let / me / show / you /////]",
     "[comment */]");

  var mode_java = CodeMirror.getMode({indentUnit: 2}, "text/x-java");
  function MTJAVA(name) { test.mode("java_" + name, mode_java, Array.prototype.slice.call(arguments, 1)); }
  MTJAVA("types",
         "[type byte];",
         "[type short];",
         "[type int];",
         "[type long];",
         "[type float];",
         "[type double];",
         "[type boolean];",
         "[type char];",
         "[type void];",
         "[type Boolean];",
         "[type Byte];",
         "[type Character];",
         "[type Double];",
         "[type Float];",
         "[type Integer];",
         "[type Long];",
         "[type Number];",
         "[type Object];",
         "[type Short];",
         "[type String];",
         "[type StringBuffer];",
         "[type StringBuilder];",
         "[type Void];");
})();
