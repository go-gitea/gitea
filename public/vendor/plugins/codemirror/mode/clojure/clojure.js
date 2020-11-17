// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function(mod) {
  if (typeof exports === "object" && typeof module === "object") // CommonJS
    mod(require("../../lib/codemirror"));
  else if (typeof define === "function" && define.amd) // AMD
    define(["../../lib/codemirror"], mod);
  else // Plain browser env
    mod(CodeMirror);
})(function(CodeMirror) {
"use strict";

CodeMirror.defineMode("clojure", function (options) {
  var atoms = ["false", "nil", "true"];
  var specialForms = [".", "catch", "def", "do", "if", "monitor-enter",
      "monitor-exit", "new", "quote", "recur", "set!", "throw", "try", "var"];
  var coreSymbols = ["*", "*'", "*1", "*2", "*3", "*agent*",
      "*allow-unresolved-vars*", "*assert*", "*clojure-version*",
      "*command-line-args*", "*compile-files*", "*compile-path*",
      "*compiler-options*", "*data-readers*", "*default-data-reader-fn*", "*e",
      "*err*", "*file*", "*flush-on-newline*", "*fn-loader*", "*in*",
      "*math-context*", "*ns*", "*out*", "*print-dup*", "*print-length*",
      "*print-level*", "*print-meta*", "*print-namespace-maps*",
      "*print-readably*", "*read-eval*", "*reader-resolver*", "*source-path*",
      "*suppress-read*", "*unchecked-math*", "*use-context-classloader*",
      "*verbose-defrecords*", "*warn-on-reflection*", "+", "+'", "-", "-'",
      "->", "->>", "->ArrayChunk", "->Eduction", "->Vec", "->VecNode",
      "->VecSeq", "-cache-protocol-fn", "-reset-methods", "..", "/", "<", "<=",
      "=", "==", ">", ">=", "EMPTY-NODE", "Inst", "StackTraceElement->vec",
      "Throwable->map", "accessor", "aclone", "add-classpath", "add-watch",
      "agent", "agent-error", "agent-errors", "aget", "alength", "alias",
      "all-ns", "alter", "alter-meta!", "alter-var-root", "amap", "ancestors",
      "and", "any?", "apply", "areduce", "array-map", "as->", "aset",
      "aset-boolean", "aset-byte", "aset-char", "aset-double", "aset-float",
      "aset-int", "aset-long", "aset-short", "assert", "assoc", "assoc!",
      "assoc-in", "associative?", "atom", "await", "await-for", "await1",
      "bases", "bean", "bigdec", "bigint", "biginteger", "binding", "bit-and",
      "bit-and-not", "bit-clear", "bit-flip", "bit-not", "bit-or", "bit-set",
      "bit-shift-left", "bit-shift-right", "bit-test", "bit-xor", "boolean",
      "boolean-array", "boolean?", "booleans", "bound-fn", "bound-fn*",
      "bound?", "bounded-count", "butlast", "byte", "byte-array", "bytes",
      "bytes?", "case", "cast", "cat", "char", "char-array",
      "char-escape-string", "char-name-string", "char?", "chars", "chunk",
      "chunk-append", "chunk-buffer", "chunk-cons", "chunk-first", "chunk-next",
      "chunk-rest", "chunked-seq?", "class", "class?", "clear-agent-errors",
      "clojure-version", "coll?", "comment", "commute", "comp", "comparator",
      "compare", "compare-and-set!", "compile", "complement", "completing",
      "concat", "cond", "cond->", "cond->>", "condp", "conj", "conj!", "cons",
      "constantly", "construct-proxy", "contains?", "count", "counted?",
      "create-ns", "create-struct", "cycle", "dec", "dec'", "decimal?",
      "declare", "dedupe", "default-data-readers", "definline", "definterface",
      "defmacro", "defmethod", "defmulti", "defn", "defn-", "defonce",
      "defprotocol", "defrecord", "defstruct", "deftype", "delay", "delay?",
      "deliver", "denominator", "deref", "derive", "descendants", "destructure",
      "disj", "disj!", "dissoc", "dissoc!", "distinct", "distinct?", "doall",
      "dorun", "doseq", "dosync", "dotimes", "doto", "double", "double-array",
      "double?", "doubles", "drop", "drop-last", "drop-while", "eduction",
      "empty", "empty?", "ensure", "ensure-reduced", "enumeration-seq",
      "error-handler", "error-mode", "eval", "even?", "every-pred", "every?",
      "ex-data", "ex-info", "extend", "extend-protocol", "extend-type",
      "extenders", "extends?", "false?", "ffirst", "file-seq", "filter",
      "filterv", "find", "find-keyword", "find-ns", "find-protocol-impl",
      "find-protocol-method", "find-var", "first", "flatten", "float",
      "float-array", "float?", "floats", "flush", "fn", "fn?", "fnext", "fnil",
      "for", "force", "format", "frequencies", "future", "future-call",
      "future-cancel", "future-cancelled?", "future-done?", "future?",
      "gen-class", "gen-interface", "gensym", "get", "get-in", "get-method",
      "get-proxy-class", "get-thread-bindings", "get-validator", "group-by",
      "halt-when", "hash", "hash-combine", "hash-map", "hash-ordered-coll",
      "hash-set", "hash-unordered-coll", "ident?", "identical?", "identity",
      "if-let", "if-not", "if-some", "ifn?", "import", "in-ns", "inc", "inc'",
      "indexed?", "init-proxy", "inst-ms", "inst-ms*", "inst?", "instance?",
      "int", "int-array", "int?", "integer?", "interleave", "intern",
      "interpose", "into", "into-array", "ints", "io!", "isa?", "iterate",
      "iterator-seq", "juxt", "keep", "keep-indexed", "key", "keys", "keyword",
      "keyword?", "last", "lazy-cat", "lazy-seq", "let", "letfn", "line-seq",
      "list", "list*", "list?", "load", "load-file", "load-reader",
      "load-string", "loaded-libs", "locking", "long", "long-array", "longs",
      "loop", "macroexpand", "macroexpand-1", "make-array", "make-hierarchy",
      "map", "map-entry?", "map-indexed", "map?", "mapcat", "mapv", "max",
      "max-key", "memfn", "memoize", "merge", "merge-with", "meta",
      "method-sig", "methods", "min", "min-key", "mix-collection-hash", "mod",
      "munge", "name", "namespace", "namespace-munge", "nat-int?", "neg-int?",
      "neg?", "newline", "next", "nfirst", "nil?", "nnext", "not", "not-any?",
      "not-empty", "not-every?", "not=", "ns", "ns-aliases", "ns-imports",
      "ns-interns", "ns-map", "ns-name", "ns-publics", "ns-refers",
      "ns-resolve", "ns-unalias", "ns-unmap", "nth", "nthnext", "nthrest",
      "num", "number?", "numerator", "object-array", "odd?", "or", "parents",
      "partial", "partition", "partition-all", "partition-by", "pcalls", "peek",
      "persistent!", "pmap", "pop", "pop!", "pop-thread-bindings", "pos-int?",
      "pos?", "pr", "pr-str", "prefer-method", "prefers",
      "primitives-classnames", "print", "print-ctor", "print-dup",
      "print-method", "print-simple", "print-str", "printf", "println",
      "println-str", "prn", "prn-str", "promise", "proxy",
      "proxy-call-with-super", "proxy-mappings", "proxy-name", "proxy-super",
      "push-thread-bindings", "pvalues", "qualified-ident?",
      "qualified-keyword?", "qualified-symbol?", "quot", "rand", "rand-int",
      "rand-nth", "random-sample", "range", "ratio?", "rational?",
      "rationalize", "re-find", "re-groups", "re-matcher", "re-matches",
      "re-pattern", "re-seq", "read", "read-line", "read-string",
      "reader-conditional", "reader-conditional?", "realized?", "record?",
      "reduce", "reduce-kv", "reduced", "reduced?", "reductions", "ref",
      "ref-history-count", "ref-max-history", "ref-min-history", "ref-set",
      "refer", "refer-clojure", "reify", "release-pending-sends", "rem",
      "remove", "remove-all-methods", "remove-method", "remove-ns",
      "remove-watch", "repeat", "repeatedly", "replace", "replicate", "require",
      "reset!", "reset-meta!", "reset-vals!", "resolve", "rest",
      "restart-agent", "resultset-seq", "reverse", "reversible?", "rseq",
      "rsubseq", "run!", "satisfies?", "second", "select-keys", "send",
      "send-off", "send-via", "seq", "seq?", "seqable?", "seque", "sequence",
      "sequential?", "set", "set-agent-send-executor!",
      "set-agent-send-off-executor!", "set-error-handler!", "set-error-mode!",
      "set-validator!", "set?", "short", "short-array", "shorts", "shuffle",
      "shutdown-agents", "simple-ident?", "simple-keyword?", "simple-symbol?",
      "slurp", "some", "some->", "some->>", "some-fn", "some?", "sort",
      "sort-by", "sorted-map", "sorted-map-by", "sorted-set", "sorted-set-by",
      "sorted?", "special-symbol?", "spit", "split-at", "split-with", "str",
      "string?", "struct", "struct-map", "subs", "subseq", "subvec", "supers",
      "swap!", "swap-vals!", "symbol", "symbol?", "sync", "tagged-literal",
      "tagged-literal?", "take", "take-last", "take-nth", "take-while", "test",
      "the-ns", "thread-bound?", "time", "to-array", "to-array-2d",
      "trampoline", "transduce", "transient", "tree-seq", "true?", "type",
      "unchecked-add", "unchecked-add-int", "unchecked-byte", "unchecked-char",
      "unchecked-dec", "unchecked-dec-int", "unchecked-divide-int",
      "unchecked-double", "unchecked-float", "unchecked-inc",
      "unchecked-inc-int", "unchecked-int", "unchecked-long",
      "unchecked-multiply", "unchecked-multiply-int", "unchecked-negate",
      "unchecked-negate-int", "unchecked-remainder-int", "unchecked-short",
      "unchecked-subtract", "unchecked-subtract-int", "underive", "unquote",
      "unquote-splicing", "unreduced", "unsigned-bit-shift-right", "update",
      "update-in", "update-proxy", "uri?", "use", "uuid?", "val", "vals",
      "var-get", "var-set", "var?", "vary-meta", "vec", "vector", "vector-of",
      "vector?", "volatile!", "volatile?", "vreset!", "vswap!", "when",
      "when-first", "when-let", "when-not", "when-some", "while",
      "with-bindings", "with-bindings*", "with-in-str", "with-loading-context",
      "with-local-vars", "with-meta", "with-open", "with-out-str",
      "with-precision", "with-redefs", "with-redefs-fn", "xml-seq", "zero?",
      "zipmap"];
  var haveBodyParameter = [
      "->", "->>", "as->", "binding", "bound-fn", "case", "catch", "comment",
      "cond", "cond->", "cond->>", "condp", "def", "definterface", "defmethod",
      "defn", "defmacro", "defprotocol", "defrecord", "defstruct", "deftype",
      "do", "doseq", "dotimes", "doto", "extend", "extend-protocol",
      "extend-type", "fn", "for", "future", "if", "if-let", "if-not", "if-some",
      "let", "letfn", "locking", "loop", "ns", "proxy", "reify", "struct-map",
      "some->", "some->>", "try", "when", "when-first", "when-let", "when-not",
      "when-some", "while", "with-bindings", "with-bindings*", "with-in-str",
      "with-loading-context", "with-local-vars", "with-meta", "with-open",
      "with-out-str", "with-precision", "with-redefs", "with-redefs-fn"];

  CodeMirror.registerHelper("hintWords", "clojure",
    [].concat(atoms, specialForms, coreSymbols));

  var atom = createLookupMap(atoms);
  var specialForm = createLookupMap(specialForms);
  var coreSymbol = createLookupMap(coreSymbols);
  var hasBodyParameter = createLookupMap(haveBodyParameter);
  var delimiter = /^(?:[\\\[\]\s"(),;@^`{}~]|$)/;
  var numberLiteral = /^(?:[+\-]?\d+(?:(?:N|(?:[eE][+\-]?\d+))|(?:\.?\d*(?:M|(?:[eE][+\-]?\d+))?)|\/\d+|[xX][0-9a-fA-F]+|r[0-9a-zA-Z]+)?(?=[\\\[\]\s"#'(),;@^`{}~]|$))/;
  var characterLiteral = /^(?:\\(?:backspace|formfeed|newline|return|space|tab|o[0-7]{3}|u[0-9A-Fa-f]{4}|x[0-9A-Fa-f]{4}|.)?(?=[\\\[\]\s"(),;@^`{}~]|$))/;

  // simple-namespace := /^[^\\\/\[\]\d\s"#'(),;@^`{}~][^\\\[\]\s"(),;@^`{}~]*/
  // simple-symbol    := /^(?:\/|[^\\\/\[\]\d\s"#'(),;@^`{}~][^\\\[\]\s"(),;@^`{}~]*)/
  // qualified-symbol := (<simple-namespace>(<.><simple-namespace>)*</>)?<simple-symbol>
  var qualifiedSymbol = /^(?:(?:[^\\\/\[\]\d\s"#'(),;@^`{}~][^\\\[\]\s"(),;@^`{}~]*(?:\.[^\\\/\[\]\d\s"#'(),;@^`{}~][^\\\[\]\s"(),;@^`{}~]*)*\/)?(?:\/|[^\\\/\[\]\d\s"#'(),;@^`{}~][^\\\[\]\s"(),;@^`{}~]*)*(?=[\\\[\]\s"(),;@^`{}~]|$))/;

  function base(stream, state) {
    if (stream.eatSpace() || stream.eat(",")) return ["space", null];
    if (stream.match(numberLiteral)) return [null, "number"];
    if (stream.match(characterLiteral)) return [null, "string-2"];
    if (stream.eat(/^"/)) return (state.tokenize = inString)(stream, state);
    if (stream.eat(/^[(\[{]/)) return ["open", "bracket"];
    if (stream.eat(/^[)\]}]/)) return ["close", "bracket"];
    if (stream.eat(/^;/)) {stream.skipToEnd(); return ["space", "comment"];}
    if (stream.eat(/^[#'@^`~]/)) return [null, "meta"];

    var matches = stream.match(qualifiedSymbol);
    var symbol = matches && matches[0];

    if (!symbol) {
      // advance stream by at least one character so we don't get stuck.
      stream.next();
      stream.eatWhile(function (c) {return !is(c, delimiter);});
      return [null, "error"];
    }

    if (symbol === "comment" && state.lastToken === "(")
      return (state.tokenize = inComment)(stream, state);
    if (is(symbol, atom) || symbol.charAt(0) === ":") return ["symbol", "atom"];
    if (is(symbol, specialForm) || is(symbol, coreSymbol)) return ["symbol", "keyword"];
    if (state.lastToken === "(") return ["symbol", "builtin"]; // other operator

    return ["symbol", "variable"];
  }

  function inString(stream, state) {
    var escaped = false, next;

    while (next = stream.next()) {
      if (next === "\"" && !escaped) {state.tokenize = base; break;}
      escaped = !escaped && next === "\\";
    }

    return [null, "string"];
  }

  function inComment(stream, state) {
    var parenthesisCount = 1;
    var next;

    while (next = stream.next()) {
      if (next === ")") parenthesisCount--;
      if (next === "(") parenthesisCount++;
      if (parenthesisCount === 0) {
        stream.backUp(1);
        state.tokenize = base;
        break;
      }
    }

    return ["space", "comment"];
  }

  function createLookupMap(words) {
    var obj = {};

    for (var i = 0; i < words.length; ++i) obj[words[i]] = true;

    return obj;
  }

  function is(value, test) {
    if (test instanceof RegExp) return test.test(value);
    if (test instanceof Object) return test.propertyIsEnumerable(value);
  }

  return {
    startState: function () {
      return {
        ctx: {prev: null, start: 0, indentTo: 0},
        lastToken: null,
        tokenize: base
      };
    },

    token: function (stream, state) {
      if (stream.sol() && (typeof state.ctx.indentTo !== "number"))
        state.ctx.indentTo = state.ctx.start + 1;

      var typeStylePair = state.tokenize(stream, state);
      var type = typeStylePair[0];
      var style = typeStylePair[1];
      var current = stream.current();

      if (type !== "space") {
        if (state.lastToken === "(" && state.ctx.indentTo === null) {
          if (type === "symbol" && is(current, hasBodyParameter))
            state.ctx.indentTo = state.ctx.start + options.indentUnit;
          else state.ctx.indentTo = "next";
        } else if (state.ctx.indentTo === "next") {
          state.ctx.indentTo = stream.column();
        }

        state.lastToken = current;
      }

      if (type === "open")
        state.ctx = {prev: state.ctx, start: stream.column(), indentTo: null};
      else if (type === "close") state.ctx = state.ctx.prev || state.ctx;

      return style;
    },

    indent: function (state) {
      var i = state.ctx.indentTo;

      return (typeof i === "number") ?
        i :
        state.ctx.start + 1;
    },

    closeBrackets: {pairs: "()[]{}\"\""},
    lineComment: ";;"
  };
});

CodeMirror.defineMIME("text/x-clojure", "clojure");
CodeMirror.defineMIME("text/x-clojurescript", "clojure");
CodeMirror.defineMIME("application/edn", "clojure");

});
