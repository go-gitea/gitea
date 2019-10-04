// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "soy");
  function MT(name) {test.mode(name, mode, Array.prototype.slice.call(arguments, 1));}

  // Test of small keywords and words containing them.
  MT('keywords-test',
     '[keyword {] [keyword as] worrying [keyword and] notorious [keyword as]',
     '    the Fandor[operator -]alias assassin, [keyword or]',
     '    Corcand cannot fit [keyword in] [keyword }]');

  MT('let-test',
     '[keyword {template] [def .name][keyword }]',
     '  [keyword {let] [def $name]: [string "world"][keyword /}]',
     '  [tag&bracket <][tag h1][tag&bracket >]',
     '    Hello, [keyword {][variable-2 $name][keyword }]',
     '  [tag&bracket </][tag h1][tag&bracket >]',
     '[keyword {/template}]',
     '');

  MT('function-test',
     '[keyword {] [callee&variable css]([string "MyClass"])[keyword }]',
     '[tag&bracket <][tag input] [attribute value]=[string "]' +
     '[keyword {] [callee&variable index]([variable-2&error $list])[keyword }]' +
        '[string "][tag&bracket />]');

  MT('namespace-test',
     '[keyword {namespace] [variable namespace][keyword }]')

  MT('namespace-with-attribute-test',
     '[keyword {namespace] [variable my.namespace.templates] ' +
         '[attribute requirecss]=[string "my.namespace"][keyword }]');

  MT('operators-test',
     '[keyword {] [atom 1] [operator ==] [atom 1] [keyword }]',
     '[keyword {] [atom 1] [operator !=] [atom 2] [keyword }]',
     '[keyword {] [atom 2] [operator +] [atom 2] [keyword }]',
     '[keyword {] [atom 2] [operator -] [atom 2] [keyword }]',
     '[keyword {] [atom 2] [operator *] [atom 2] [keyword }]',
     '[keyword {] [atom 2] [operator /] [atom 2] [keyword }]',
     '[keyword {] [atom 2] [operator %] [atom 2] [keyword }]',
     '[keyword {] [atom 2] [operator <=] [atom 2] [keyword }]',
     '[keyword {] [atom 2] [operator >=] [atom 2] [keyword }]',
     '[keyword {] [atom 3] [operator >] [atom 2] [keyword }]',
     '[keyword {] [atom 2] [operator >] [atom 3] [keyword }]',
     '[keyword {] [atom null] [operator ?:] [string ""] [keyword }]',
     '[keyword {] [variable-2&error $variable] [operator |] safeHtml [keyword }]')

  MT('primitive-test',
     '[keyword {] [atom true] [keyword }]',
     '[keyword {] [atom false] [keyword }]',
     '[keyword {] truethy [keyword }]',
     '[keyword {] falsey [keyword }]',
     '[keyword {] [atom 42] [keyword }]',
     '[keyword {] [atom .42] [keyword }]',
     '[keyword {] [atom 0.42] [keyword }]',
     '[keyword {] [atom -0.42] [keyword }]',
     '[keyword {] [atom -.2] [keyword }]',
     '[keyword {] [atom 6.03e23] [keyword }]',
     '[keyword {] [atom -0.03e0] [keyword }]',
     '[keyword {] [atom 0x1F] [keyword }]',
     '[keyword {] [atom 0x1F00BBEA] [keyword }]');

  MT('param-type-test',
     '[keyword {@param] [def a]: ' +
         '[type list]<[[[type a]: [type int], ' +
         '[type b]: [type map]<[type string], ' +
         '[type bool]>]]>][keyword }]',
      '[keyword {@param] [def unknown]: [type ?][keyword }]',
      '[keyword {@param] [def list]: [type list]<[type ?]>[keyword }]');

  MT('undefined-var',
     '[keyword {][variable-2&error $var]');

  MT('param-scope-test',
     '[keyword {template] [def .a][keyword }]',
     '  [keyword {@param] [def x]: [type string][keyword }]',
     '  [keyword {][variable-2 $x][keyword }]',
     '[keyword {/template}]',
     '',
     '[keyword {template] [def .b][keyword }]',
     '  [keyword {][variable-2&error $x][keyword }]',
     '[keyword {/template}]',
     '');

  MT('if-variable-test',
     '[keyword {if] [variable-2&error $showThing][keyword }]',
     '  Yo!',
     '[keyword {/if}]',
     '');

  MT('defined-if-variable-test',
     '[keyword {template] [def .foo][keyword }]',
     '  [keyword {@param?] [def showThing]: [type bool][keyword }]',
     '  [keyword {if] [variable-2 $showThing][keyword }]',
     '    Yo!',
     '  [keyword {/if}]',
     '[keyword {/template}]',
     '');

  MT('template-calls-test',
     '[keyword {call] [variable-2 .foo][keyword /}]',
     '[keyword {call] [variable foo][keyword /}]',
     '[keyword {call] [variable foo][keyword }] [keyword {/call}]',
     '[keyword {call] [variable first1.second.third_3][keyword /}]',
     '[keyword {call] [variable first1.second.third_3] [keyword }] [keyword {/call}]',
     '');

  MT('foreach-scope-test',
     '[keyword {@param] [def bar]: [type string][keyword }]',
     '[keyword {foreach] [def $foo] [keyword in] [variable-2&error $foos][keyword }]',
     '  [keyword {][variable-2 $foo][keyword }]',
     '[keyword {/foreach}]',
     '[keyword {][variable-2&error $foo][keyword }]',
     '[keyword {][variable-2 $bar][keyword }]');

  MT('foreach-ifempty-indent-test',
     '[keyword {foreach] [def $foo] [keyword in] [variable-2&error $foos][keyword }]',
     '  something',
     '[keyword {ifempty}]',
     '  nothing',
     '[keyword {/foreach}]',
     '');

  MT('nested-kind-test',
     '[keyword {template] [def .foo] [attribute kind]=[string "html"][keyword }]',
     '  [tag&bracket <][tag div][tag&bracket >]',
     '    [keyword {call] [variable-2 .bar][keyword }]',
     '      [keyword {param] [property propertyName] [attribute kind]=[string "js"][keyword }]',
     '        [keyword var] [def bar] [operator =] [number 5];',
     '      [keyword {/param}]',
     '    [keyword {/call}]',
     '  [tag&bracket </][tag div][tag&bracket >]',
     '[keyword {/template}]',
     '');

  MT('tag-starting-with-function-call-is-not-a-keyword',
     '[keyword {][callee&variable index]([variable-2&error $foo])[keyword }]',
     '[keyword {css] [string "some-class"][keyword }]',
     '[keyword {][callee&variable css]([string "some-class"])[keyword }]',
     '');

  MT('allow-missing-colon-in-@param',
     '[keyword {template] [def .foo][keyword }]',
     '  [keyword {@param] [def showThing] [type bool][keyword }]',
     '  [keyword {if] [variable-2 $showThing][keyword }]',
     '    Yo!',
     '  [keyword {/if}]',
     '[keyword {/template}]',
     '');

  MT('single-quote-strings',
     '[keyword {][string "foo"] [string \'bar\'][keyword }]',
     '');

  MT('literal-comments',
     '[keyword {literal}]/* comment */ // comment[keyword {/literal}]');

  MT('highlight-command-at-eol',
     '[keyword {msg]',
     '    [keyword }]');

  MT('switch-indent-test',
     '[keyword {let] [def $marbles]: [atom 5] [keyword /}]',
     '[keyword {switch] [variable-2 $marbles][keyword }]',
     '  [keyword {case] [atom 0][keyword }]',
     '    No marbles',
     '  [keyword {default}]',
     '    At least 1 marble',
     '[keyword {/switch}]',
     '');

  MT('if-elseif-else-indent',
     '[keyword {if] [atom true][keyword }]',
     '  [keyword {let] [def $a]: [atom 5] [keyword /}]',
     '[keyword {elseif] [atom false][keyword }]',
     '  [keyword {let] [def $bar]: [atom 5] [keyword /}]',
     '[keyword {else}]',
     '  [keyword {let] [def $bar]: [atom 5] [keyword /}]',
     '[keyword {/if}]');

  MT('msg-fallbackmsg-indent',
     '[keyword {msg] [attribute desc]=[string "A message"][keyword }]',
     '  A message',
     '[keyword {fallbackmsg] [attribute desc]=[string "A message"][keyword }]',
     '  Old message',
     '[keyword {/msg}]');

  MT('special-chars',
     '[keyword {sp}]',
     '[keyword {nil}]',
     '[keyword {\\r}]',
     '[keyword {\\n}]',
     '[keyword {\\t}]',
     '[keyword {lb}]',
     '[keyword {rb}]');

  MT('wrong-closing-tag',
     '[keyword {if] [atom true][keyword }]',
     '  Optional',
     '[keyword&error {/badend][keyword }]');
})();
