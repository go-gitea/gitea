// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "text/x-xu");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1), "xu"); }

  MT("empty chart",
     "[keyword msc][bracket {]",
     "[base   ]",
     "[bracket }]"
  );

  MT("empty chart",
     "[keyword xu][bracket {]",
     "[base   ]",
     "[bracket }]"
  );

  MT("comments",
    "[comment // a single line comment]",
    "[comment # another  single line comment /* and */ ignored here]",
    "[comment /* A multi-line comment even though it contains]",
    "[comment msc keywords and \"quoted text\"*/]");

  MT("strings",
    "[string \"// a string\"]",
    "[string \"a string running over]",
    "[string two lines\"]",
    "[string \"with \\\"escaped quote\"]"
  );

  MT("xù/ msgenny keywords classify as 'keyword'",
    "[keyword watermark]",
    "[keyword alt]","[keyword loop]","[keyword opt]","[keyword ref]","[keyword else]","[keyword break]","[keyword par]","[keyword seq]","[keyword assert]"
  );

  MT("xù/ msgenny constants classify as 'variable'",
    "[variable auto]",
    "[variable true]", "[variable false]", "[variable on]", "[variable off]"
  );

  MT("mscgen options classify as keyword",
    "[keyword hscale]", "[keyword width]", "[keyword arcgradient]", "[keyword wordwraparcs]"
  );

  MT("mscgen arcs classify as keyword",
    "[keyword note]","[keyword abox]","[keyword rbox]","[keyword box]",
    "[keyword |||...---]", "[keyword ..--==::]",
    "[keyword ->]", "[keyword <-]", "[keyword <->]",
    "[keyword =>]", "[keyword <=]", "[keyword <=>]",
    "[keyword =>>]", "[keyword <<=]", "[keyword <<=>>]",
    "[keyword >>]", "[keyword <<]", "[keyword <<>>]",
    "[keyword -x]", "[keyword x-]", "[keyword -X]", "[keyword X-]",
    "[keyword :>]", "[keyword <:]", "[keyword <:>]"
  );

  MT("within an attribute list, attributes classify as attribute",
    "[bracket [[][attribute label]",
    "[attribute id]","[attribute url]","[attribute idurl]",
    "[attribute linecolor]","[attribute linecolour]","[attribute textcolor]","[attribute textcolour]","[attribute textbgcolor]","[attribute textbgcolour]",
    "[attribute arclinecolor]","[attribute arclinecolour]","[attribute arctextcolor]","[attribute arctextcolour]","[attribute arctextbgcolor]","[attribute arctextbgcolour]",
    "[attribute arcskip]","[attribute title]",
    "[attribute activate]","[attribute deactivate]","[attribute activation][bracket ]]]"
  );

  MT("outside an attribute list, attributes classify as base",
    "[base label]",
    "[base id]","[base url]","[base idurl]",
    "[base linecolor]","[base linecolour]","[base textcolor]","[base textcolour]","[base textbgcolor]","[base textbgcolour]",
    "[base arclinecolor]","[base arclinecolour]","[base arctextcolor]","[base arctextcolour]","[base arctextbgcolor]","[base arctextbgcolour]",
    "[base arcskip]", "[base title]"
  );

  MT("a typical program",
    "[comment # typical xu program]",
    "[keyword xu][base  ][bracket {]",
    "[keyword wordwraparcs][operator =][string \"true\"][base , ][keyword hscale][operator =][string \"0.8\"][base , ][keyword arcgradient][operator =][base 30, ][keyword width][operator =][variable auto][base ;]",
    "[base   a][bracket [[][attribute label][operator =][string \"Entity A\"][bracket ]]][base ,]",
    "[base   b][bracket [[][attribute label][operator =][string \"Entity B\"][bracket ]]][base ,]",
    "[base   c][bracket [[][attribute label][operator =][string \"Entity C\"][bracket ]]][base ;]",
    "[base   a ][keyword =>>][base  b][bracket [[][attribute label][operator =][string \"Hello entity B\"][bracket ]]][base ;]",
    "[base   a ][keyword <<][base  b][bracket [[][attribute label][operator =][string \"Here's an answer dude!\"][base , ][attribute title][operator =][string \"This is a title for this message\"][bracket ]]][base ;]",
    "[base   c ][keyword :>][base  *][bracket [[][attribute label][operator =][string \"What about me?\"][base , ][attribute textcolor][operator =][base red][bracket ]]][base ;]",
    "[bracket }]"
  );
})();
