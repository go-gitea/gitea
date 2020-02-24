// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "swift");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  // Ensure all number types are properly represented.
  MT("numbers",
     "[keyword var] [def a] [operator =] [number 17]",
     "[keyword var] [def b] [operator =] [number -0.5]",
     "[keyword var] [def c] [operator =] [number 0.3456e-4]",
     "[keyword var] [def d] [operator =] [number 345e2]",
     "[keyword var] [def e] [operator =] [number 0o7324]",
     "[keyword var] [def f] [operator =] [number 0b10010]",
     "[keyword var] [def g] [operator =] [number -0x35ade]",
     "[keyword var] [def h] [operator =] [number 0xaea.ep-13]",
     "[keyword var] [def i] [operator =] [number 0x13ep6]");

  // Variable/class/etc definition.
  MT("definition",
     "[keyword var] [def a] [operator =] [number 5]",
     "[keyword let] [def b][punctuation :] [variable-2 Int] [operator =] [number 10]",
     "[keyword class] [def C] [punctuation {] [punctuation }]",
     "[keyword struct] [def D] [punctuation {] [punctuation }]",
     "[keyword enum] [def E] [punctuation {] [punctuation }]",
     "[keyword extension] [def F] [punctuation {] [punctuation }]",
     "[keyword protocol] [def G] [punctuation {] [punctuation }]",
     "[keyword func] [def h][punctuation ()] [punctuation {] [punctuation }]",
     "[keyword import] [def Foundation]",
     "[keyword typealias] [def NewString] [operator =] [variable-2 String]",
     "[keyword associatedtype] [def I]",
     "[keyword for] [def j] [keyword in] [number 0][punctuation ..][operator <][number 3] [punctuation {] [punctuation }]");

  // Strings and string interpolation.
  MT("strings",
     "[keyword var] [def a][punctuation :] [variable-2 String] [operator =] [string \"test\"]",
     "[keyword var] [def b][punctuation :] [variable-2 String] [operator =] [string \"\\(][variable a][string )\"]",
     "[keyword var] [def c] [operator =] [string \"\"\"]",
     "[string multi]",
     "[string line]",
     "[string \"test\"]",
     "[string \"\"\"]",
     "[variable print][punctuation (][string \"\"][punctuation )]");

  // Comments.
  MT("comments",
     "[comment // This is a comment]",
     "[comment /* This is another comment */]",
     "[keyword var] [def a] [operator =] [number 5] [comment // Third comment]");

  // Atoms.
  MT("atoms",
     "[keyword class] [def FooClass] [punctuation {]",
     "  [keyword let] [def fooBool][punctuation :] [variable-2 Bool][operator ?]",
     "  [keyword let] [def fooInt][punctuation :] [variable-2 Int][operator ?]",
     "  [keyword func] [keyword init][punctuation (][variable fooBool][punctuation :] [variable-2 Bool][punctuation ,] [variable barBool][punctuation :] [variable-2 Bool][punctuation )] [punctuation {]",
     "    [atom super][property .init][punctuation ()]",
     "    [atom self][property .fooBool] [operator =] [variable fooBool]",
     "    [variable fooInt] [operator =] [atom nil]",
     "    [keyword if] [variable barBool] [operator ==] [atom true] [punctuation {]",
     "      [variable print][punctuation (][string \"True!\"][punctuation )]",
     "    [punctuation }] [keyword else] [keyword if] [variable barBool] [operator ==] [atom false] [punctuation {]",
     "      [keyword for] [atom _] [keyword in] [number 0][punctuation ...][number 5] [punctuation {]",
     "        [variable print][punctuation (][string \"False!\"][punctuation )]",
     "      [punctuation }]",
     "    [punctuation }]",
     "  [punctuation }]",
     "[punctuation }]");

  // Types.
  MT("types",
     "[keyword var] [def a] [operator =] [variable-2 Array][operator <][variable-2 Int][operator >]",
     "[keyword var] [def b] [operator =] [variable-2 Set][operator <][variable-2 Bool][operator >]",
     "[keyword var] [def c] [operator =] [variable-2 Dictionary][operator <][variable-2 String][punctuation ,][variable-2 Character][operator >]",
     "[keyword var] [def d][punctuation :] [variable-2 Int64][operator ?] [operator =] [variable-2 Optional][punctuation (][number 8][punctuation )]",
     "[keyword func] [def e][punctuation ()] [operator ->] [variable-2 Void] [punctuation {]",
     "  [keyword var] [def e1][punctuation :] [variable-2 Float] [operator =] [number 1.2]",
     "[punctuation }]",
     "[keyword func] [def f][punctuation ()] [operator ->] [variable-2 Never] [punctuation {]",
     "  [keyword var] [def f1][punctuation :] [variable-2 Double] [operator =] [number 2.4]",
     "[punctuation }]");

  // Operators.
  MT("operators",
     "[keyword var] [def a] [operator =] [number 1] [operator +] [number 2]",
     "[keyword var] [def b] [operator =] [number 1] [operator -] [number 2]",
     "[keyword var] [def c] [operator =] [number 1] [operator *] [number 2]",
     "[keyword var] [def d] [operator =] [number 1] [operator /] [number 2]",
     "[keyword var] [def e] [operator =] [number 1] [operator %] [number 2]",
     "[keyword var] [def f] [operator =] [number 1] [operator |] [number 2]",
     "[keyword var] [def g] [operator =] [number 1] [operator &] [number 2]",
     "[keyword var] [def h] [operator =] [number 1] [operator <<] [number 2]",
     "[keyword var] [def i] [operator =] [number 1] [operator >>] [number 2]",
     "[keyword var] [def j] [operator =] [number 1] [operator ^] [number 2]",
     "[keyword var] [def k] [operator =] [operator ~][number 1]",
     "[keyword var] [def l] [operator =] [variable foo] [operator ?] [number 1] [punctuation :] [number 2]",
     "[keyword var] [def m][punctuation :] [variable-2 Int] [operator =] [variable-2 Optional][punctuation (][number 8][punctuation )][operator !]");

  // Punctuation.
  MT("punctuation",
     "[keyword let] [def a] [operator =] [number 1][punctuation ;] [keyword let] [def b] [operator =] [number 2]",
     "[keyword let] [def testArr][punctuation :] [punctuation [[][variable-2 Int][punctuation ]]] [operator =] [punctuation [[][variable a][punctuation ,] [variable b][punctuation ]]]",
     "[keyword for] [def i] [keyword in] [number 0][punctuation ..][operator <][variable testArr][property .count] [punctuation {]",
     "  [variable print][punctuation (][variable testArr][punctuation [[][variable i][punctuation ]])]",
     "[punctuation }]");

  // Identifiers.
  MT("identifiers",
     "[keyword let] [def abc] [operator =] [number 1]",
     "[keyword let] [def ABC] [operator =] [number 2]",
     "[keyword let] [def _123] [operator =] [number 3]",
     "[keyword let] [def _$1$2$3] [operator =] [number 4]",
     "[keyword let] [def A1$_c32_$_] [operator =] [number 5]",
     "[keyword let] [def `var`] [operator =] [punctuation [[][number 1][punctuation ,] [number 2][punctuation ,] [number 3][punctuation ]]]",
     "[keyword let] [def square$] [operator =] [variable `var`][property .map] [punctuation {][variable $0] [operator *] [variable $0][punctuation }]",
     "$$ [number 1][variable a] $[atom _] [variable _$] [variable __] `[variable a] [variable b]`");

  // Properties.
  MT("properties",
     "[variable print][punctuation (][variable foo][property .abc][punctuation )]",
     "[variable print][punctuation (][variable foo][property .ABC][punctuation )]",
     "[variable print][punctuation (][variable foo][property ._123][punctuation )]",
     "[variable print][punctuation (][variable foo][property ._$1$2$3][punctuation )]",
     "[variable print][punctuation (][variable foo][property .A1$_c32_$_][punctuation )]",
     "[variable print][punctuation (][variable foo][property .`var`][punctuation )]",
     "[variable print][punctuation (][variable foo][property .__][punctuation )]");

  // Instructions or other things that start with #.
  MT("instructions",
     "[keyword if] [builtin #available][punctuation (][variable iOS] [number 9][punctuation ,] [operator *][punctuation )] [punctuation {}]",
     "[variable print][punctuation (][builtin #file][punctuation ,] [builtin #function][punctuation )]",
     "[variable print][punctuation (][builtin #line][punctuation ,] [builtin #column][punctuation )]",
     "[builtin #if] [atom true]",
     "[keyword import] [def A]",
     "[builtin #elseif] [atom false]",
     "[keyword import] [def B]",
     "[builtin #endif]",
     "[builtin #sourceLocation][punctuation (][variable file][punctuation :] [string \"file.swift\"][punctuation ,] [variable line][punctuation :] [number 2][punctuation )]");

  // Attributes; things that start with @.
  MT("attributes",
     "[attribute @objc][punctuation (][variable objcFoo][punctuation :)]",
     "[attribute @available][punctuation (][variable iOS][punctuation )]");

  // Property/number edge case.
  MT("property_number",
     "[variable print][punctuation (][variable foo][property ._123][punctuation )]",
     "[variable print][punctuation (]")

  MT("nested_comments",
     "[comment /*]",
     "[comment But wait /* this is a nested comment */ for real]",
     "[comment /**** let * me * show * you ****/]",
     "[comment ///// let / me / show / you /////]",
     "[comment */]");

  // TODO: correctly identify when multiple variables are being declared
  // by use of a comma-separated list.
  // TODO: correctly identify when variables are being declared in a tuple.
  // TODO: identify protocols as types when used before an extension?
})();
