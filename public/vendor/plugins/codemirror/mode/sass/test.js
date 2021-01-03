// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function() {
  var mode = CodeMirror.getMode({indentUnit: 2}, "sass");
  // Since Sass has an indent-based syntax, is almost impossible to test correctly the indentation in all cases.
  // So disable it for tests.
  mode.indent = undefined;
  function MT(name) { test.mode(name, mode, Array.prototype.slice.call(arguments, 1)); }

  MT("comment",
     "[comment // this is a comment]",
     "[comment   also this is a comment]")

  MT("comment_multiline",
     "[comment /* this is a comment]",
     "[comment   also this is a comment]")

  MT("variable",
     "[variable-2 $page-width][operator :] [number 800][unit px]")

  MT("global_attributes",
     "[tag body]",
     "  [property font][operator :]",
     "    [property family][operator :] [atom sans-serif]",
     "    [property size][operator :] [number 30][unit em]",
     "    [property weight][operator :] [atom bold]")

  MT("scoped_styles",
     "[builtin #contents]",
     "  [property width][operator :] [variable-2 $page-width]",
     "  [builtin #sidebar]",
     "    [property float][operator :] [atom right]",
     "    [property width][operator :] [variable-2 $sidebar-width]",
     "  [builtin #main]",
     "    [property width][operator :] [variable-2 $page-width] [operator -] [variable-2 $sidebar-width]",
     "    [property background][operator :] [variable-2 $primary-color]",
     "    [tag h2]",
     "      [property color][operator :] [keyword blue]")

  // Sass allows to write the colon as first char instead of a "separator".
  //   :color red
  // Not supported
  // MT("property_syntax",
  //    "[qualifier .foo]",
  //    "  [operator :][property color] [keyword red]")

  MT("import",
     "[def @import] [string \"sass/variables\"]",
     // Probably it should parsed as above: as a string even without the " or '
     // "[def @import] [string sass/baz]"
     "[def @import] [tag sass][operator /][tag baz]")

  MT("def",
     "[def @if] [variable-2 $foo] [def @else]")

  MT("tag_on_more_lines",
    "[tag td],",
    "[tag th]",
    "  [property font-family][operator :] [string \"Arial\"], [atom serif]")

  MT("important",
     "[qualifier .foo]",
     "  [property text-decoration][operator :] [atom none] [keyword !important]",
     "[tag h1]",
     "  [property font-size][operator :] [number 2.5][unit em]")

  MT("selector",
     // SCSS doesn't highlight the :
     // "[tag h1]:[variable-3 before],",
     // "[tag h2]:[variable-3 before]",
     "[tag h1][variable-3 :before],",
     "[tag h2][variable-3 :before]",
     "  [property content][operator :] [string \"::\"]")

  MT("definition_mixin_equal",
     "[variable-2 $defined-bs-type][operator :] [atom border-box] [keyword !default]",
     "[meta =bs][operator (][variable-2 $bs-type][operator :] [variable-2 $defined-bs-type][operator )]",
     "  [meta -webkit-][property box-sizing][operator :] [variable-2 $bs-type]",
     "  [property box-sizing][operator :] [variable-2 $bs-type]")

  MT("definition_mixin_with_space",
     "[variable-2 $defined-bs-type][operator :] [atom border-box] [keyword !default]",
     "[def @mixin] [tag bs][operator (][variable-2 $bs-type][operator :] [variable-2 $defined-bs-type][operator )] ",
     "  [meta -moz-][property box-sizing][operator :] [variable-2 $bs-type]",
     "  [property box-sizing][operator :] [variable-2 $bs-type]")

  MT("numbers_start_dot_include_plus",
     // The % is not highlighted correctly
     // "[meta =button-links][operator (][variable-2 $button-base][operator :] [atom darken][operator (][variable-2 $color11], [number 10][unit %][operator )][operator )]",
     "[meta =button-links][operator (][variable-2 $button-base][operator :] [atom darken][operator (][variable-2 $color11], [number 10][operator %))]",
     "  [property padding][operator :] [number .3][unit em] [number .6][unit em]",
     "  [variable-3 +border-radius][operator (][number 8][unit px][operator )]",
     "  [property background-color][operator :] [variable-2 $button-base]")

  MT("include",
     "[qualifier .bar]",
     "  [def @include] [tag border-radius][operator (][number 8][unit px][operator )]")

  MT("reference_parent",
     "[qualifier .col]",
     "  [property clear][operator :] [atom both]",
     // SCSS doesn't highlight the :
     // "  &:[variable-3 after]",
     "  &[variable-3 :after]",
     "    [property content][operator :] [string '']",
     "    [property clear][operator :] [atom both]")

  MT("reference_parent_with_spaces",
     "[tag section]",
     "  [property border-left][operator :]  [number 20][unit px] [atom transparent] [atom solid] ",
     "  &[qualifier .section3]",
     "    [qualifier .title]",
     "      [property color][operator :] [keyword white] ",
     "    [qualifier .vermas]",
     "      [property display][operator :] [atom none]")

  MT("font_face",
     "[def @font-face]",
     "  [property font-family][operator :] [string 'icomoon']",
     "  [property src][operator :] [atom url][operator (][string fonts/icomoon.ttf][operator )]")
})();
