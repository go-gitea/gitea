// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: http://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({tabSize: 4}, "markdown");
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }
  var modeHighlightFormatting = CodeMirror.getMode({tabSize: 4}, {name: "markdown", highlightFormatting: true});
  function FT(name) { test.mode(name, modeHighlightFormatting, Array.prototype.slice.call(arguments, 1)); }
  var modeAtxNoSpace = CodeMirror.getMode({tabSize: 4}, {name: "markdown", allowAtxHeaderWithoutSpace: true});
  function AtxNoSpaceTest(name) { test.mode(name, modeAtxNoSpace, Array.prototype.slice.call(arguments, 1)); }
  var modeFenced = CodeMirror.getMode({tabSize: 4}, {name: "markdown", fencedCodeBlocks: true});
  function FencedTest(name) { test.mode(name, modeFenced, Array.prototype.slice.call(arguments, 1)); }
  var modeOverrideClasses = CodeMirror.getMode({tabsize: 4}, {
    name: "markdown",
    strikethrough: true,
    tokenTypeOverrides: {
      "header" : "override-header",
      "code" : "override-code",
      "quote" : "override-quote",
      "list1" : "override-list1",
      "list2" : "override-list2",
      "list3" : "override-list3",
      "hr" : "override-hr",
      "image" : "override-image",
      "imageAltText": "override-image-alt-text",
      "imageMarker": "override-image-marker",
      "linkInline" : "override-link-inline",
      "linkEmail" : "override-link-email",
      "linkText" : "override-link-text",
      "linkHref" : "override-link-href",
      "em" : "override-em",
      "strong" : "override-strong",
      "strikethrough" : "override-strikethrough"
  }});
  function TokenTypeOverrideTest(name) { test.mode(name, modeOverrideClasses, Array.prototype.slice.call(arguments, 1)); }
  var modeFormattingOverride = CodeMirror.getMode({tabsize: 4}, {
    name: "markdown",
    highlightFormatting: true,
    tokenTypeOverrides: {
      "formatting" : "override-formatting"
  }});
  function FormatTokenTypeOverrideTest(name) { test.mode(name, modeFormattingOverride, Array.prototype.slice.call(arguments, 1)); }


  FT("formatting_emAsterisk",
     "[em&formatting&formatting-em *][em foo][em&formatting&formatting-em *]");

  FT("formatting_emUnderscore",
     "[em&formatting&formatting-em _][em foo][em&formatting&formatting-em _]");

  FT("formatting_strongAsterisk",
     "[strong&formatting&formatting-strong **][strong foo][strong&formatting&formatting-strong **]");

  FT("formatting_strongUnderscore",
     "[strong&formatting&formatting-strong __][strong foo][strong&formatting&formatting-strong __]");

  FT("formatting_codeBackticks",
     "[comment&formatting&formatting-code `][comment foo][comment&formatting&formatting-code `]");

  FT("formatting_doubleBackticks",
     "[comment&formatting&formatting-code ``][comment foo ` bar][comment&formatting&formatting-code ``]");

  FT("formatting_atxHeader",
     "[header&header-1&formatting&formatting-header&formatting-header-1 # ][header&header-1 foo # bar ][header&header-1&formatting&formatting-header&formatting-header-1 #]");

  FT("formatting_setextHeader",
     "foo",
     "[header&header-1&formatting&formatting-header&formatting-header-1 =]");

  FT("formatting_blockquote",
     "[quote&quote-1&formatting&formatting-quote&formatting-quote-1 > ][quote&quote-1 foo]");

  FT("formatting_list",
     "[variable-2&formatting&formatting-list&formatting-list-ul - ][variable-2 foo]");
  FT("formatting_list",
     "[variable-2&formatting&formatting-list&formatting-list-ol 1. ][variable-2 foo]");

  FT("formatting_link",
     "[link&formatting&formatting-link [][link foo][link&formatting&formatting-link ]]][string&formatting&formatting-link-string&url (][string&url http://example.com/][string&formatting&formatting-link-string&url )]");

  FT("formatting_linkReference",
     "[link&formatting&formatting-link [][link foo][link&formatting&formatting-link ]]][string&formatting&formatting-link-string&url [][string&url bar][string&formatting&formatting-link-string&url ]]]",
     "[link&formatting&formatting-link [][link bar][link&formatting&formatting-link ]]:] [string&url http://example.com/]");

  FT("formatting_linkWeb",
     "[link&formatting&formatting-link <][link http://example.com/][link&formatting&formatting-link >]");

  FT("formatting_linkEmail",
     "[link&formatting&formatting-link <][link user@example.com][link&formatting&formatting-link >]");

  FT("formatting_escape",
     "[formatting-escape \\*]");

  FT("formatting_image",
     "[formatting&formatting-image&image&image-marker !][formatting&formatting-image&image&image-alt-text&link [[][image&image-alt-text&link alt text][formatting&formatting-image&image&image-alt-text&link ]]][formatting&formatting-link-string&string&url (][url&string http://link.to/image.jpg][formatting&formatting-link-string&string&url )]");

  MT("plainText",
     "foo");

  // Don't style single trailing space
  MT("trailingSpace1",
     "foo ");

  // Two or more trailing spaces should be styled with line break character
  MT("trailingSpace2",
     "foo[trailing-space-a  ][trailing-space-new-line  ]");

  MT("trailingSpace3",
     "foo[trailing-space-a  ][trailing-space-b  ][trailing-space-new-line  ]");

  MT("trailingSpace4",
     "foo[trailing-space-a  ][trailing-space-b  ][trailing-space-a  ][trailing-space-new-line  ]");

  // Code blocks using 4 spaces (regardless of CodeMirror.tabSize value)
  MT("codeBlocksUsing4Spaces",
     "    [comment foo]");

  // Code blocks using 4 spaces with internal indentation
  MT("codeBlocksUsing4SpacesIndentation",
     "    [comment bar]",
     "        [comment hello]",
     "            [comment world]",
     "    [comment foo]",
     "bar");

  // Code blocks should end even after extra indented lines
  MT("codeBlocksWithTrailingIndentedLine",
     "    [comment foo]",
     "        [comment bar]",
     "    [comment baz]",
     "    ",
     "hello");

  // Code blocks using 1 tab (regardless of CodeMirror.indentWithTabs value)
  MT("codeBlocksUsing1Tab",
     "\t[comment foo]");

  // No code blocks directly after paragraph
  // http://spec.commonmark.org/0.19/#example-65
  MT("noCodeBlocksAfterParagraph",
     "Foo",
     "    Bar");

  // Inline code using backticks
  MT("inlineCodeUsingBackticks",
     "foo [comment `bar`]");

  // Block code using single backtick (shouldn't work)
  MT("blockCodeSingleBacktick",
     "[comment `]",
     "[comment foo]",
     "[comment `]");

  // Unclosed backticks
  // Instead of simply marking as CODE, it would be nice to have an
  // incomplete flag for CODE, that is styled slightly different.
  MT("unclosedBackticks",
     "foo [comment `bar]");

  // Per documentation: "To include a literal backtick character within a
  // code span, you can use multiple backticks as the opening and closing
  // delimiters"
  MT("doubleBackticks",
     "[comment ``foo ` bar``]");

  // Tests based on Dingus
  // http://daringfireball.net/projects/markdown/dingus
  //
  // Multiple backticks within an inline code block
  MT("consecutiveBackticks",
     "[comment `foo```bar`]");

  // Multiple backticks within an inline code block with a second code block
  MT("consecutiveBackticks",
     "[comment `foo```bar`] hello [comment `world`]");

  // Unclosed with several different groups of backticks
  MT("unclosedBackticks",
     "[comment ``foo ``` bar` hello]");

  // Closed with several different groups of backticks
  MT("closedBackticks",
     "[comment ``foo ``` bar` hello``] world");

  // atx headers
  // http://daringfireball.net/projects/markdown/syntax#header

  MT("atxH1",
     "[header&header-1 # foo]");

  MT("atxH2",
     "[header&header-2 ## foo]");

  MT("atxH3",
     "[header&header-3 ### foo]");

  MT("atxH4",
     "[header&header-4 #### foo]");

  MT("atxH5",
     "[header&header-5 ##### foo]");

  MT("atxH6",
     "[header&header-6 ###### foo]");

  // http://spec.commonmark.org/0.19/#example-24
  MT("noAtxH7",
     "####### foo");

  // http://spec.commonmark.org/0.19/#example-25
  MT("noAtxH1WithoutSpace",
     "#5 bolt");

  // CommonMark requires a space after # but most parsers don't
  AtxNoSpaceTest("atxNoSpaceAllowed_H1NoSpace",
     "[header&header-1 #foo]");

  AtxNoSpaceTest("atxNoSpaceAllowed_H4NoSpace",
     "[header&header-4 ####foo]");

  AtxNoSpaceTest("atxNoSpaceAllowed_H1Space",
     "[header&header-1 # foo]");

  // Inline styles should be parsed inside headers
  MT("atxH1inline",
     "[header&header-1 # foo ][header&header-1&em *bar*]");

  // Setext headers - H1, H2
  // Per documentation, "Any number of underlining =’s or -’s will work."
  // http://daringfireball.net/projects/markdown/syntax#header
  // Ideally, the text would be marked as `header` as well, but this is
  // not really feasible at the moment. So, instead, we're testing against
  // what works today, to avoid any regressions.
  //
  // Check if single underlining = works
  MT("setextH1",
     "foo",
     "[header&header-1 =]");

  // Check if 3+ ='s work
  MT("setextH1",
     "foo",
     "[header&header-1 ===]");

  // Check if single underlining - works
  MT("setextH2",
     "foo",
     "[header&header-2 -]");

  // Check if 3+ -'s work
  MT("setextH2",
     "foo",
     "[header&header-2 ---]");

  // http://spec.commonmark.org/0.19/#example-45
  MT("setextH2AllowSpaces",
     "foo",
     "   [header&header-2 ----      ]");

  // http://spec.commonmark.org/0.19/#example-44
  MT("noSetextAfterIndentedCodeBlock",
     "     [comment foo]",
     "[hr ---]");

  // http://spec.commonmark.org/0.19/#example-51
  MT("noSetextAfterQuote",
     "[quote&quote-1 > foo]",
     "[hr ---]");

  MT("noSetextAfterList",
     "[variable-2 - foo]",
     "[hr ---]");

  // Single-line blockquote with trailing space
  MT("blockquoteSpace",
     "[quote&quote-1 > foo]");

  // Single-line blockquote
  MT("blockquoteNoSpace",
     "[quote&quote-1 >foo]");

  // No blank line before blockquote
  MT("blockquoteNoBlankLine",
     "foo",
     "[quote&quote-1 > bar]");

  // Nested blockquote
  MT("blockquoteSpace",
     "[quote&quote-1 > foo]",
     "[quote&quote-1 >][quote&quote-2 > foo]",
     "[quote&quote-1 >][quote&quote-2 >][quote&quote-3 > foo]");

  // Single-line blockquote followed by normal paragraph
  MT("blockquoteThenParagraph",
     "[quote&quote-1 >foo]",
     "",
     "bar");

  // Multi-line blockquote (lazy mode)
  MT("multiBlockquoteLazy",
     "[quote&quote-1 >foo]",
     "[quote&quote-1 bar]");

  // Multi-line blockquote followed by normal paragraph (lazy mode)
  MT("multiBlockquoteLazyThenParagraph",
     "[quote&quote-1 >foo]",
     "[quote&quote-1 bar]",
     "",
     "hello");

  // Multi-line blockquote (non-lazy mode)
  MT("multiBlockquote",
     "[quote&quote-1 >foo]",
     "[quote&quote-1 >bar]");

  // Multi-line blockquote followed by normal paragraph (non-lazy mode)
  MT("multiBlockquoteThenParagraph",
     "[quote&quote-1 >foo]",
     "[quote&quote-1 >bar]",
     "",
     "hello");

  // Header with leading space after continued blockquote (#3287, negative indentation)
  MT("headerAfterContinuedBlockquote",
     "[quote&quote-1 > foo]",
     "[quote&quote-1 bar]",
     "",
     " [header&header-1 # hello]");

  // Check list types

  MT("listAsterisk",
     "foo",
     "bar",
     "",
     "[variable-2 * foo]",
     "[variable-2 * bar]");

  MT("listPlus",
     "foo",
     "bar",
     "",
     "[variable-2 + foo]",
     "[variable-2 + bar]");

  MT("listDash",
     "foo",
     "bar",
     "",
     "[variable-2 - foo]",
     "[variable-2 - bar]");

  MT("listNumber",
     "foo",
     "bar",
     "",
     "[variable-2 1. foo]",
     "[variable-2 2. bar]");

  // Lists require a preceding blank line (per Dingus)
  MT("listBogus",
     "foo",
     "1. bar",
     "2. hello");

  // List after hr
  MT("listAfterHr",
     "[hr ---]",
     "[variable-2 - bar]");

  // List after header
  MT("listAfterHeader",
     "[header&header-1 # foo]",
     "[variable-2 - bar]");

  // hr after list
  MT("hrAfterList",
     "[variable-2 - foo]",
     "[hr -----]");

  // Formatting in lists (*)
  MT("listAsteriskFormatting",
     "[variable-2 * ][variable-2&em *foo*][variable-2  bar]",
     "[variable-2 * ][variable-2&strong **foo**][variable-2  bar]",
     "[variable-2 * ][variable-2&strong **][variable-2&em&strong *foo**][variable-2&em *][variable-2  bar]",
     "[variable-2 * ][variable-2&comment `foo`][variable-2  bar]");

  // Formatting in lists (+)
  MT("listPlusFormatting",
     "[variable-2 + ][variable-2&em *foo*][variable-2  bar]",
     "[variable-2 + ][variable-2&strong **foo**][variable-2  bar]",
     "[variable-2 + ][variable-2&strong **][variable-2&em&strong *foo**][variable-2&em *][variable-2  bar]",
     "[variable-2 + ][variable-2&comment `foo`][variable-2  bar]");

  // Formatting in lists (-)
  MT("listDashFormatting",
     "[variable-2 - ][variable-2&em *foo*][variable-2  bar]",
     "[variable-2 - ][variable-2&strong **foo**][variable-2  bar]",
     "[variable-2 - ][variable-2&strong **][variable-2&em&strong *foo**][variable-2&em *][variable-2  bar]",
     "[variable-2 - ][variable-2&comment `foo`][variable-2  bar]");

  // Formatting in lists (1.)
  MT("listNumberFormatting",
     "[variable-2 1. ][variable-2&em *foo*][variable-2  bar]",
     "[variable-2 2. ][variable-2&strong **foo**][variable-2  bar]",
     "[variable-2 3. ][variable-2&strong **][variable-2&em&strong *foo**][variable-2&em *][variable-2  bar]",
     "[variable-2 4. ][variable-2&comment `foo`][variable-2  bar]");

  // Paragraph lists
  MT("listParagraph",
     "[variable-2 * foo]",
     "",
     "[variable-2 * bar]");

  // Multi-paragraph lists
  //
  // 4 spaces
  MT("listMultiParagraph",
     "[variable-2 * foo]",
     "",
     "[variable-2 * bar]",
     "",
     "    [variable-2 hello]");

  // 4 spaces, extra blank lines (should still be list, per Dingus)
  MT("listMultiParagraphExtra",
     "[variable-2 * foo]",
     "",
     "[variable-2 * bar]",
     "",
     "",
     "    [variable-2 hello]");

  // 4 spaces, plus 1 space (should still be list, per Dingus)
  MT("listMultiParagraphExtraSpace",
     "[variable-2 * foo]",
     "",
     "[variable-2 * bar]",
     "",
     "     [variable-2 hello]",
     "",
     "    [variable-2 world]");

  // 1 tab
  MT("listTab",
     "[variable-2 * foo]",
     "",
     "[variable-2 * bar]",
     "",
     "\t[variable-2 hello]");

  // No indent
  MT("listNoIndent",
     "[variable-2 * foo]",
     "",
     "[variable-2 * bar]",
     "",
     "hello");

  MT("listCommonMarkIndentationCode",
     "[variable-2 * Code blocks also affect]",
     "  [variable-3 * The next level starts where the contents start.]",
     "   [variable-3 *    Anything less than that will keep the item on the same level.]",
     "       [variable-3 * Each list item can indent the first level further and further.]",
     "  [variable-3 * For the most part, this makes sense while writing a list.]",
     "    [keyword * This means two items with same indentation can be different levels.]",
     "     [keyword *  Each level has an indent requirement that can change between items.]",
     "       [keyword * A list item that meets this will be part of the next level.]",
     "   [variable-3 * Otherwise, it will be part of the level where it does meet this.]",
     " [variable-2 * World]");

  // Blockquote
  MT("blockquote",
     "[variable-2 * foo]",
     "",
     "[variable-2 * bar]",
     "",
     "    [variable-2&quote&quote-1 > hello]");

  // Code block
  MT("blockquoteCode",
     "[variable-2 * foo]",
     "",
     "[variable-2 * bar]",
     "",
     "        [comment > hello]",
     "",
     "    [variable-2 world]");

  // Code block followed by text
  MT("blockquoteCodeText",
     "[variable-2 * foo]",
     "",
     "    [variable-2 bar]",
     "",
     "        [comment hello]",
     "",
     "    [variable-2 world]");

  // Nested list

  MT("listAsteriskNested",
     "[variable-2 * foo]",
     "",
     "    [variable-3 * bar]");

  MT("listPlusNested",
     "[variable-2 + foo]",
     "",
     "    [variable-3 + bar]");

  MT("listDashNested",
     "[variable-2 - foo]",
     "",
     "    [variable-3 - bar]");

  MT("listNumberNested",
     "[variable-2 1. foo]",
     "",
     "    [variable-3 2. bar]");

  MT("listMixed",
     "[variable-2 * foo]",
     "",
     "    [variable-3 + bar]",
     "",
     "        [keyword - hello]",
     "",
     "            [variable-2 1. world]");

  MT("listBlockquote",
     "[variable-2 * foo]",
     "",
     "    [variable-3 + bar]",
     "",
     "        [quote&quote-1&variable-3 > hello]");

  MT("listCode",
     "[variable-2 * foo]",
     "",
     "    [variable-3 + bar]",
     "",
     "            [comment hello]");

  // Code with internal indentation
  MT("listCodeIndentation",
     "[variable-2 * foo]",
     "",
     "        [comment bar]",
     "            [comment hello]",
     "                [comment world]",
     "        [comment foo]",
     "    [variable-2 bar]");

  // List nesting edge cases
  MT("listNested",
    "[variable-2 * foo]",
    "",
    "    [variable-3 * bar]",
    "",
    "       [variable-3 hello]"
  );
  MT("listNested",
    "[variable-2 * foo]",
    "",
    "    [variable-3 * bar]",
    "",
    "      [keyword * foo]"
  );

  // Code followed by text
  MT("listCodeText",
     "[variable-2 * foo]",
     "",
     "        [comment bar]",
     "",
     "hello");

  // Following tests directly from official Markdown documentation
  // http://daringfireball.net/projects/markdown/syntax#hr

  MT("hrSpace",
     "[hr * * *]");

  MT("hr",
     "[hr ***]");

  MT("hrLong",
     "[hr *****]");

  MT("hrSpaceDash",
     "[hr - - -]");

  MT("hrDashLong",
     "[hr ---------------------------------------]");

  //Images
  MT("Images",
     "[image&image-marker !][image&image-alt-text&link [[alt text]]][string&url (http://link.to/image.jpg)]")

  //Images with highlight alt text
  MT("imageEm",
     "[image&image-marker !][image&image-alt-text&link [[][image-alt-text&em&image&link *alt text*][image&image-alt-text&link ]]][string&url (http://link.to/image.jpg)]");

  MT("imageStrong",
     "[image&image-marker !][image&image-alt-text&link [[][image-alt-text&strong&image&link **alt text**][image&image-alt-text&link ]]][string&url (http://link.to/image.jpg)]");

  MT("imageEmStrong",
     "[image&image-marker !][image&image-alt-text&link [[][image-alt-text&image&strong&link **][image&image-alt-text&em&strong&link *alt text**][image&image-alt-text&em&link *][image&image-alt-text&link ]]][string&url (http://link.to/image.jpg)]");

  // Inline link with title
  MT("linkTitle",
     "[link [[foo]]][string&url (http://example.com/ \"bar\")] hello");

  // Inline link without title
  MT("linkNoTitle",
     "[link [[foo]]][string&url (http://example.com/)] bar");

  // Inline link with image
  MT("linkImage",
     "[link [[][link&image&image-marker !][link&image&image-alt-text&link [[alt text]]][string&url (http://link.to/image.jpg)][link ]]][string&url (http://example.com/)] bar");

  // Inline link with Em
  MT("linkEm",
     "[link [[][link&em *foo*][link ]]][string&url (http://example.com/)] bar");

  // Inline link with Strong
  MT("linkStrong",
     "[link [[][link&strong **foo**][link ]]][string&url (http://example.com/)] bar");

  // Inline link with EmStrong
  MT("linkEmStrong",
     "[link [[][link&strong **][link&em&strong *foo**][link&em *][link ]]][string&url (http://example.com/)] bar");

  // Image with title
  MT("imageTitle",
     "[image&image-marker !][image&image-alt-text&link [[alt text]]][string&url (http://example.com/ \"bar\")] hello");

  // Image without title
  MT("imageNoTitle",
     "[image&image-marker !][image&image-alt-text&link [[alt text]]][string&url (http://example.com/)] bar");

  // Image with asterisks
  MT("imageAsterisks",
     "[image&image-marker !][image&image-alt-text&link [[ ][image&image-alt-text&em&link *alt text*][image&image-alt-text&link ]]][string&url (http://link.to/image.jpg)] bar");

  // Not a link. Should be normal text due to square brackets being used
  // regularly in text, especially in quoted material, and no space is allowed
  // between square brackets and parentheses (per Dingus).
  MT("notALink",
     "[[foo]] (bar)");

  // Reference-style links
  MT("linkReference",
     "[link [[foo]]][string&url [[bar]]] hello");

  // Reference-style links with Em
  MT("linkReferenceEm",
     "[link [[][link&em *foo*][link ]]][string&url [[bar]]] hello");

  // Reference-style links with Strong
  MT("linkReferenceStrong",
     "[link [[][link&strong **foo**][link ]]][string&url [[bar]]] hello");

  // Reference-style links with EmStrong
  MT("linkReferenceEmStrong",
     "[link [[][link&strong **][link&em&strong *foo**][link&em *][link ]]][string&url [[bar]]] hello");

  // Reference-style links with optional space separator (per documentation)
  // "You can optionally use a space to separate the sets of brackets"
  MT("linkReferenceSpace",
     "[link [[foo]]] [string&url [[bar]]] hello");

  // Should only allow a single space ("...use *a* space...")
  MT("linkReferenceDoubleSpace",
     "[[foo]]  [[bar]] hello");

  // Reference-style links with implicit link name
  MT("linkImplicit",
     "[link [[foo]]][string&url [[]]] hello");

  // @todo It would be nice if, at some point, the document was actually
  // checked to see if the referenced link exists

  // Link label, for reference-style links (taken from documentation)

  MT("labelNoTitle",
     "[link [[foo]]:] [string&url http://example.com/]");

  MT("labelIndented",
     "   [link [[foo]]:] [string&url http://example.com/]");

  MT("labelSpaceTitle",
     "[link [[foo bar]]:] [string&url http://example.com/ \"hello\"]");

  MT("labelDoubleTitle",
     "[link [[foo bar]]:] [string&url http://example.com/ \"hello\"] \"world\"");

  MT("labelTitleDoubleQuotes",
     "[link [[foo]]:] [string&url http://example.com/  \"bar\"]");

  MT("labelTitleSingleQuotes",
     "[link [[foo]]:] [string&url http://example.com/  'bar']");

  MT("labelTitleParentheses",
     "[link [[foo]]:] [string&url http://example.com/  (bar)]");

  MT("labelTitleInvalid",
     "[link [[foo]]:] [string&url http://example.com/] bar");

  MT("labelLinkAngleBrackets",
     "[link [[foo]]:] [string&url <http://example.com/>  \"bar\"]");

  MT("labelTitleNextDoubleQuotes",
     "[link [[foo]]:] [string&url http://example.com/]",
     "[string \"bar\"] hello");

  MT("labelTitleNextSingleQuotes",
     "[link [[foo]]:] [string&url http://example.com/]",
     "[string 'bar'] hello");

  MT("labelTitleNextParentheses",
     "[link [[foo]]:] [string&url http://example.com/]",
     "[string (bar)] hello");

  MT("labelTitleNextMixed",
     "[link [[foo]]:] [string&url http://example.com/]",
     "(bar\" hello");

  MT("labelEscape",
     "[link [[foo \\]] ]]:] [string&url http://example.com/]");

  MT("labelEscapeColon",
     "[link [[foo \\]]: bar]]:] [string&url http://example.com/]");

  MT("labelEscapeEnd",
     "[[foo\\]]: http://example.com/");

  MT("linkWeb",
     "[link <http://example.com/>] foo");

  MT("linkWebDouble",
     "[link <http://example.com/>] foo [link <http://example.com/>]");

  MT("linkEmail",
     "[link <user@example.com>] foo");

  MT("linkEmailDouble",
     "[link <user@example.com>] foo [link <user@example.com>]");

  MT("emAsterisk",
     "[em *foo*] bar");

  MT("emUnderscore",
     "[em _foo_] bar");

  MT("emInWordAsterisk",
     "foo[em *bar*]hello");

  MT("emInWordUnderscore",
     "foo[em _bar_]hello");

  // Per documentation: "...surround an * or _ with spaces, it’ll be
  // treated as a literal asterisk or underscore."

  MT("emEscapedBySpaceIn",
     "foo [em _bar _ hello_] world");

  MT("emEscapedBySpaceOut",
     "foo _ bar[em _hello_]world");

  MT("emEscapedByNewline",
     "foo",
     "_ bar[em _hello_]world");

  // Unclosed emphasis characters
  // Instead of simply marking as EM / STRONG, it would be nice to have an
  // incomplete flag for EM and STRONG, that is styled slightly different.
  MT("emIncompleteAsterisk",
     "foo [em *bar]");

  MT("emIncompleteUnderscore",
     "foo [em _bar]");

  MT("strongAsterisk",
     "[strong **foo**] bar");

  MT("strongUnderscore",
     "[strong __foo__] bar");

  MT("emStrongAsterisk",
     "[em *foo][em&strong **bar*][strong hello**] world");

  MT("emStrongUnderscore",
     "[em _foo][em&strong __bar_][strong hello__] world");

  // "...same character must be used to open and close an emphasis span.""
  MT("emStrongMixed",
     "[em _foo][em&strong **bar*hello__ world]");

  MT("emStrongMixed",
     "[em *foo][em&strong __bar_hello** world]");

  MT("linkWithNestedParens",
     "[link [[foo]]][string&url (bar(baz))]")

  // These characters should be escaped:
  // \   backslash
  // `   backtick
  // *   asterisk
  // _   underscore
  // {}  curly braces
  // []  square brackets
  // ()  parentheses
  // #   hash mark
  // +   plus sign
  // -   minus sign (hyphen)
  // .   dot
  // !   exclamation mark

  MT("escapeBacktick",
     "foo \\`bar\\`");

  MT("doubleEscapeBacktick",
     "foo \\\\[comment `bar\\\\`]");

  MT("escapeAsterisk",
     "foo \\*bar\\*");

  MT("doubleEscapeAsterisk",
     "foo \\\\[em *bar\\\\*]");

  MT("escapeUnderscore",
     "foo \\_bar\\_");

  MT("doubleEscapeUnderscore",
     "foo \\\\[em _bar\\\\_]");

  MT("escapeHash",
     "\\# foo");

  MT("doubleEscapeHash",
     "\\\\# foo");

  MT("escapeNewline",
     "\\",
     "[em *foo*]");

  // Class override tests
  TokenTypeOverrideTest("overrideHeader1",
    "[override-header&override-header-1 # Foo]");

  TokenTypeOverrideTest("overrideHeader2",
    "[override-header&override-header-2 ## Foo]");

  TokenTypeOverrideTest("overrideHeader3",
    "[override-header&override-header-3 ### Foo]");

  TokenTypeOverrideTest("overrideHeader4",
    "[override-header&override-header-4 #### Foo]");

  TokenTypeOverrideTest("overrideHeader5",
    "[override-header&override-header-5 ##### Foo]");

  TokenTypeOverrideTest("overrideHeader6",
    "[override-header&override-header-6 ###### Foo]");

  TokenTypeOverrideTest("overrideCode",
    "[override-code `foo`]");

  TokenTypeOverrideTest("overrideCodeBlock",
    "[override-code ```]",
    "[override-code foo]",
    "[override-code ```]");

  TokenTypeOverrideTest("overrideQuote",
    "[override-quote&override-quote-1 > foo]",
    "[override-quote&override-quote-1 > bar]");

  TokenTypeOverrideTest("overrideQuoteNested",
    "[override-quote&override-quote-1 > foo]",
    "[override-quote&override-quote-1 >][override-quote&override-quote-2 > bar]",
    "[override-quote&override-quote-1 >][override-quote&override-quote-2 >][override-quote&override-quote-3 > baz]");

  TokenTypeOverrideTest("overrideLists",
    "[override-list1 - foo]",
    "",
    "    [override-list2 + bar]",
    "",
    "        [override-list3 * baz]",
    "",
    "            [override-list1 1. qux]",
    "",
    "                [override-list2 - quux]");

  TokenTypeOverrideTest("overrideHr",
    "[override-hr * * *]");

  TokenTypeOverrideTest("overrideImage",
    "[override-image&override-image-marker !][override-image&override-image-alt-text&link [[alt text]]][override-link-href&url (http://link.to/image.jpg)]");

  TokenTypeOverrideTest("overrideLinkText",
    "[override-link-text [[foo]]][override-link-href&url (http://example.com)]");

  TokenTypeOverrideTest("overrideLinkEmailAndInline",
    "[override-link-email <][override-link-inline foo@example.com>]");

  TokenTypeOverrideTest("overrideEm",
    "[override-em *foo*]");

  TokenTypeOverrideTest("overrideStrong",
    "[override-strong **foo**]");

  TokenTypeOverrideTest("overrideStrikethrough",
    "[override-strikethrough ~~foo~~]");

  FormatTokenTypeOverrideTest("overrideFormatting",
    "[override-formatting-escape \\*]");

  // Tests to make sure GFM-specific things aren't getting through

  MT("taskList",
     "[variable-2 * [ ]] bar]");

  MT("noFencedCodeBlocks",
     "~~~",
     "foo",
     "~~~");

  FencedTest("fencedCodeBlocks",
     "[comment ```]",
     "[comment foo]",
     "[comment ```]",
     "bar");

  FencedTest("fencedCodeBlocksMultipleChars",
     "[comment `````]",
     "[comment foo]",
     "[comment ```]",
     "[comment foo]",
     "[comment `````]",
     "bar");

  FencedTest("fencedCodeBlocksTildes",
     "[comment ~~~]",
     "[comment foo]",
     "[comment ~~~]",
     "bar");

  FencedTest("fencedCodeBlocksTildesMultipleChars",
     "[comment ~~~~~]",
     "[comment ~~~]",
     "[comment foo]",
     "[comment ~~~~~]",
     "bar");

  FencedTest("fencedCodeBlocksMultipleChars",
     "[comment `````]",
     "[comment foo]",
     "[comment ```]",
     "[comment foo]",
     "[comment `````]",
     "bar");

  FencedTest("fencedCodeBlocksMixed",
     "[comment ~~~]",
     "[comment ```]",
     "[comment foo]",
     "[comment ~~~]",
     "bar");

  // Tests that require XML mode

  MT("xmlMode",
     "[tag&bracket <][tag div][tag&bracket >]",
     "*foo*",
     "[tag&bracket <][tag http://github.com][tag&bracket />]",
     "[tag&bracket </][tag div][tag&bracket >]",
     "[link <http://github.com/>]");

  MT("xmlModeWithMarkdownInside",
     "[tag&bracket <][tag div] [attribute markdown]=[string 1][tag&bracket >]",
     "[em *foo*]",
     "[link <http://github.com/>]",
     "[tag </div>]",
     "[link <http://github.com/>]",
     "[tag&bracket <][tag div][tag&bracket >]",
     "[tag&bracket </][tag div][tag&bracket >]");

})();
