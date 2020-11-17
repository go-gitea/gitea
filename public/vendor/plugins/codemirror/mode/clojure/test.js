// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: https://codemirror.net/LICENSE

(function () {
    var mode = CodeMirror.getMode({indentUnit: 2}, "clojure");

    function MT(name) {
        test.mode(name, mode, Array.prototype.slice.call(arguments, 1));
    }

    MT("atoms",
        "[atom false]",
        "[atom nil]",
        "[atom true]"
    );

    MT("keywords",
        "[atom :foo]",
        "[atom ::bar]",
        "[atom :foo/bar]",
        "[atom :foo.bar/baz]"
    );

    MT("numbers",
        "[number 42] [number +42] [number -421]",
        "[number 42N] [number +42N] [number -42N]",
        "[number 0.42] [number +0.42] [number -0.42]",
        "[number 42M] [number +42M] [number -42M]",
        "[number 42.42M] [number +42.42M] [number -42.42M]",
        "[number 1/42] [number +1/42] [number -1/42]",
        "[number 0x42af] [number +0x42af] [number -0x42af]",
        "[number 0x42AF] [number +0x42AF] [number -0x42AF]",
        "[number 1e2] [number 1e+2] [number 1e-2]",
        "[number +1e2] [number +1e+2] [number +1e-2]",
        "[number -1e2] [number -1e+2] [number -1e-2]",
        "[number -1.0e2] [number -0.1e+2] [number -1.01e-2]",
        "[number 1E2] [number 1E+2] [number 1E-2]",
        "[number +1E2] [number +1E+2] [number +1E-2]",
        "[number -1E2] [number -1E+2] [number -1E-2]",
        "[number -1.0E2] [number -0.1E+2] [number -1.01E-2]",
        "[number 2r101010] [number +2r101010] [number -2r101010]",
        "[number 2r101010] [number +2r101010] [number -2r101010]",
        "[number 8r52] [number +8r52] [number -8r52]",
        "[number 36rhello] [number +36rhello] [number -36rhello]",
        "[number 36rz] [number +36rz] [number -36rz]",
        "[number 36rZ] [number +36rZ] [number -36rZ]",

        // invalid numbers
        "[error 42foo]",
        "[error 42Nfoo]",
        "[error 42Mfoo]",
        "[error 42.42Mfoo]",
        "[error 42.42M!]",
        "[error 42!]",
        "[error 0x42afm]"
    );

    MT("characters",
        "[string-2 \\1]",
        "[string-2 \\a]",
        "[string-2 \\a\\b\\c]",
        "[string-2 \\#]",
        "[string-2 \\\\]",
        "[string-2 \\\"]",
        "[string-2 \\(]",
        "[string-2 \\A]",
        "[string-2 \\backspace]",
        "[string-2 \\formfeed]",
        "[string-2 \\newline]",
        "[string-2 \\space]",
        "[string-2 \\return]",
        "[string-2 \\tab]",
        "[string-2 \\u1000]",
        "[string-2 \\uAaAa]",
        "[string-2 \\u9F9F]",
        "[string-2 \\o123]",
        "[string-2 \\符]",
        "[string-2 \\シ]",
        "[string-2 \\ۇ]",
        // FIXME
        // "[string-2 \\🙂]",

        // invalid character literals
        "[error \\abc]",
        "[error \\a123]",
        "[error \\a!]",
        "[error \\newlines]",
        "[error \\NEWLINE]",
        "[error \\u9F9FF]",
        "[error \\o1234]"
    );

    MT("strings",
        "[string \"I'm a teapot.\"]",
        "[string \"I'm a \\\"teapot\\\".\"]",
        "[string \"I'm]",       // this is
        "[string a]",           // a multi-line
        "[string teapot.\"]"    // string

        // TODO unterminated (multi-line) strings?
    );

    MT("comments",
        "[comment ; this is an in-line comment.]",
        "[comment ;; this is a line comment.]",
        "[keyword comment]",
        "[bracket (][comment comment (foo 1 2 3)][bracket )]"
    );

    MT("reader macro characters",
        "[meta #][variable _]",
        "[meta #][variable -Inf]",
        "[meta ##][variable Inf]",
        "[meta ##][variable NaN]",
        "[meta @][variable x]",
        "[meta ^][bracket {][atom :tag] [variable String][bracket }]",
        "[meta `][bracket (][builtin f] [variable x][bracket )]",
        "[meta ~][variable foo#]",
        "[meta '][number 1]",
        "[meta '][atom :foo]",
        "[meta '][string \"foo\"]",
        "[meta '][variable x]",
        "[meta '][bracket (][builtin a] [variable b] [variable c][bracket )]",
        "[meta '][bracket [[][variable a] [variable b] [variable c][bracket ]]]",
        "[meta '][bracket {][variable a] [number 1] [atom :foo] [number 2] [variable c] [number 3][bracket }]",
        "[meta '#][bracket {][variable a] [number 1] [atom :foo][bracket }]"
    );

    MT("symbols",
      "[variable foo!]",
      "[variable foo#]",
      "[variable foo$]",
      "[variable foo&]",
      "[variable foo']",
      "[variable foo*]",
      "[variable foo+]",
      "[variable foo-]",
      "[variable foo.]",
      "[variable foo/bar]",
      "[variable foo:bar]",
      "[variable foo<]",
      "[variable foo=]",
      "[variable foo>]",
      "[variable foo?]",
      "[variable foo_]",
      "[variable foo|]",
      "[variable foobarBaz]",
      "[variable foo¡]",
      "[variable 符号]",
      "[variable シンボル]",
      "[variable ئۇيغۇر]",
      "[variable 🙂❤🇺🇸]",

      // invalid symbols
      "[error 3foo]",
      "[error 3+]",
      "[error 3|]",
      "[error 3_]"
    );

    MT("numbers and other forms",
      "[number 42][bracket (][builtin foo][bracket )]",
      "[number 42][bracket [[][variable foo][bracket ]]]",
      "[number 42][meta #][bracket {][variable foo][bracket }]",
      "[number 42][bracket {][atom :foo] [variable bar][bracket }]",
      "[number 42][meta `][variable foo]",
      "[number 42][meta ~][variable foo]",
      "[number 42][meta #][variable foo]"
    );

    var specialForms = [".", "catch", "def", "do", "if", "monitor-enter",
        "monitor-exit", "new", "quote", "recur", "set!", "throw", "try", "var"];

    MT("should highlight special forms as keywords",
        typeTokenPairs("keyword", specialForms)
    );

    var coreSymbols1 = [
        "*", "*'", "*1", "*2", "*3", "*agent*", "*allow-unresolved-vars*", "*assert*",
        "*clojure-version*", "*command-line-args*", "*compile-files*", "*compile-path*", "*compiler-options*",
        "*data-readers*", "*default-data-reader-fn*", "*e", "*err*", "*file*", "*flush-on-newline*", "*fn-loader*",
        "*in*", "*math-context*", "*ns*", "*out*", "*print-dup*", "*print-length*", "*print-level*", "*print-meta*",
        "*print-namespace-maps*", "*print-readably*", "*read-eval*", "*reader-resolver*", "*source-path*",
        "*suppress-read*", "*unchecked-math*", "*use-context-classloader*", "*verbose-defrecords*",
        "*warn-on-reflection*", "+", "+'", "-", "-'", "->", "->>", "->ArrayChunk", "->Eduction", "->Vec", "->VecNode",
        "->VecSeq", "-cache-protocol-fn", "-reset-methods", "..", "/", "<", "<=", "=", "==", ">", ">=",
        "EMPTY-NODE", "Inst", "StackTraceElement->vec", "Throwable->map", "accessor", "aclone", "add-classpath",
        "add-watch", "agent", "agent-error", "agent-errors", "aget", "alength", "alias", "all-ns", "alter",
        "alter-meta!", "alter-var-root", "amap", "ancestors", "and", "any?", "apply", "areduce", "array-map",
        "as->", "aset", "aset-boolean", "aset-byte", "aset-char", "aset-double", "aset-float", "aset-int",
        "aset-long", "aset-short", "assert", "assoc", "assoc!", "assoc-in", "associative?", "atom", "await",
        "await-for", "await1", "bases", "bean", "bigdec", "bigint", "biginteger", "binding", "bit-and", "bit-and-not",
        "bit-clear", "bit-flip", "bit-not", "bit-or", "bit-set", "bit-shift-left", "bit-shift-right", "bit-test",
        "bit-xor", "boolean", "boolean-array", "boolean?", "booleans", "bound-fn", "bound-fn*", "bound?",
        "bounded-count", "butlast", "byte", "byte-array", "bytes", "bytes?", "case", "cast", "cat", "char",
        "char-array", "char-escape-string", "char-name-string", "char?", "chars", "chunk", "chunk-append",
        "chunk-buffer", "chunk-cons", "chunk-first", "chunk-next", "chunk-rest", "chunked-seq?", "class", "class?",
        "clear-agent-errors", "clojure-version", "coll?", "comment", "commute", "comp", "comparator", "compare",
        "compare-and-set!", "compile", "complement", "completing", "concat", "cond", "cond->", "cond->>", "condp",
        "conj", "conj!", "cons", "constantly", "construct-proxy", "contains?", "count", "counted?", "create-ns",
        "create-struct", "cycle", "dec", "dec'", "decimal?", "declare", "dedupe", "default-data-readers", "definline",
        "definterface", "defmacro", "defmethod", "defmulti", "defn", "defn-", "defonce", "defprotocol", "defrecord",
        "defstruct", "deftype", "delay", "delay?", "deliver", "denominator", "deref", "derive", "descendants",
        "destructure", "disj", "disj!", "dissoc", "dissoc!", "distinct", "distinct?", "doall", "dorun", "doseq",
        "dosync", "dotimes", "doto", "double", "double-array", "double?", "doubles", "drop", "drop-last", "drop-while",
        "eduction", "empty", "empty?", "ensure", "ensure-reduced", "enumeration-seq", "error-handler", "error-mode",
        "eval", "even?", "every-pred", "every?", "ex-data", "ex-info", "extend", "extend-protocol", "extend-type",
        "extenders", "extends?", "false?", "ffirst", "file-seq", "filter", "filterv", "find", "find-keyword", "find-ns",
        "find-protocol-impl", "find-protocol-method", "find-var", "first", "flatten", "float", "float-array", "float?",
        "floats", "flush", "fn", "fn?", "fnext", "fnil", "for", "force", "format", "frequencies", "future", "future-call",
        "future-cancel", "future-cancelled?", "future-done?", "future?", "gen-class", "gen-interface", "gensym", "get",
        "get-in", "get-method", "get-proxy-class", "get-thread-bindings", "get-validator", "group-by", "halt-when", "hash",
        "hash-combine", "hash-map", "hash-ordered-coll", "hash-set", "hash-unordered-coll", "ident?", "identical?",
        "identity", "if-let", "if-not", "if-some", "ifn?", "import", "in-ns", "inc", "inc'", "indexed?", "init-proxy",
        "inst-ms", "inst-ms*", "inst?", "instance?", "int", "int-array", "int?", "integer?", "interleave", "intern",
        "interpose", "into", "into-array", "ints", "io!", "isa?", "iterate", "iterator-seq", "juxt", "keep", "keep-indexed",
        "key", "keys", "keyword", "keyword?", "last", "lazy-cat", "lazy-seq", "let", "letfn", "line-seq", "list", "list*",
        "list?", "load", "load-file", "load-reader", "load-string", "loaded-libs", "locking", "long", "long-array", "longs",
        "loop", "macroexpand", "macroexpand-1", "make-array", "make-hierarchy", "map", "map-entry?", "map-indexed", "map?",
        "mapcat", "mapv", "max", "max-key", "memfn", "memoize", "merge", "merge-with", "meta", "method-sig", "methods"];

    var coreSymbols2 = [
        "min", "min-key", "mix-collection-hash", "mod", "munge", "name", "namespace", "namespace-munge", "nat-int?",
        "neg-int?", "neg?", "newline", "next", "nfirst", "nil?", "nnext", "not", "not-any?", "not-empty", "not-every?",
        "not=", "ns", "ns-aliases", "ns-imports", "ns-interns", "ns-map", "ns-name", "ns-publics", "ns-refers", "ns-resolve",
        "ns-unalias", "ns-unmap", "nth", "nthnext", "nthrest", "num", "number?", "numerator", "object-array", "odd?", "or",
        "parents", "partial", "partition", "partition-all", "partition-by", "pcalls", "peek", "persistent!", "pmap", "pop",
        "pop!", "pop-thread-bindings", "pos-int?", "pos?", "pr", "pr-str", "prefer-method", "prefers",
        "primitives-classnames", "print", "print-ctor", "print-dup", "print-method", "print-simple", "print-str", "printf",
        "println", "println-str", "prn", "prn-str", "promise", "proxy", "proxy-call-with-super", "proxy-mappings",
        "proxy-name", "proxy-super", "push-thread-bindings", "pvalues", "qualified-ident?", "qualified-keyword?",
        "qualified-symbol?", "quot", "rand", "rand-int", "rand-nth", "random-sample", "range", "ratio?", "rational?",
        "rationalize", "re-find", "re-groups", "re-matcher", "re-matches", "re-pattern", "re-seq", "read", "read-line",
        "read-string", "reader-conditional", "reader-conditional?", "realized?", "record?", "reduce", "reduce-kv", "reduced",
        "reduced?", "reductions", "ref", "ref-history-count", "ref-max-history", "ref-min-history", "ref-set", "refer",
        "refer-clojure", "reify", "release-pending-sends", "rem", "remove", "remove-all-methods", "remove-method", "remove-ns",
        "remove-watch", "repeat", "repeatedly", "replace", "replicate", "require", "reset!", "reset-meta!", "reset-vals!",
        "resolve", "rest", "restart-agent", "resultset-seq", "reverse", "reversible?", "rseq", "rsubseq", "run!", "satisfies?",
        "second", "select-keys", "send", "send-off", "send-via", "seq", "seq?", "seqable?", "seque", "sequence", "sequential?",
        "set", "set-agent-send-executor!", "set-agent-send-off-executor!", "set-error-handler!", "set-error-mode!",
        "set-validator!", "set?", "short", "short-array", "shorts", "shuffle", "shutdown-agents", "simple-ident?",
        "simple-keyword?", "simple-symbol?", "slurp", "some", "some->", "some->>", "some-fn", "some?", "sort", "sort-by",
        "sorted-map", "sorted-map-by", "sorted-set", "sorted-set-by", "sorted?", "special-symbol?", "spit", "split-at",
        "split-with", "str", "string?", "struct", "struct-map", "subs", "subseq", "subvec", "supers", "swap!", "swap-vals!",
        "symbol", "symbol?", "sync", "tagged-literal", "tagged-literal?", "take", "take-last", "take-nth", "take-while", "test",
        "the-ns", "thread-bound?", "time", "to-array", "to-array-2d", "trampoline", "transduce", "transient", "tree-seq",
        "true?", "type", "unchecked-add", "unchecked-add-int", "unchecked-byte", "unchecked-char", "unchecked-dec",
        "unchecked-dec-int", "unchecked-divide-int", "unchecked-double", "unchecked-float", "unchecked-inc", "unchecked-inc-int",
        "unchecked-int", "unchecked-long", "unchecked-multiply", "unchecked-multiply-int", "unchecked-negate",
        "unchecked-negate-int", "unchecked-remainder-int", "unchecked-short", "unchecked-subtract", "unchecked-subtract-int",
        "underive", "unquote", "unquote-splicing", "unreduced", "unsigned-bit-shift-right", "update", "update-in",
        "update-proxy", "uri?", "use", "uuid?", "val", "vals", "var-get", "var-set", "var?", "vary-meta", "vec", "vector",
        "vector-of", "vector?", "volatile!", "volatile?", "vreset!", "vswap!", "when", "when-first", "when-let", "when-not",
        "when-some", "while", "with-bindings", "with-bindings*", "with-in-str", "with-loading-context", "with-local-vars",
        "with-meta", "with-open", "with-out-str", "with-precision", "with-redefs", "with-redefs-fn", "xml-seq", "zero?",
        "zipmap"
    ];

    MT("should highlight core symbols as keywords (part 1/2)",
        typeTokenPairs("keyword", coreSymbols1)
    );

    MT("should highlight core symbols as keywords (part 2/2)",
        typeTokenPairs("keyword", coreSymbols2)
    );

    MT("should properly indent forms in list literals",
        "[bracket (][builtin foo] [atom :a] [number 1] [atom true] [atom nil][bracket )]",
        "",
        "[bracket (][builtin foo] [atom :a]",
        "     [number 1]",
        "     [atom true]",
        "     [atom nil][bracket )]",
        "",
        "[bracket (][builtin foo] [atom :a] [number 1]",
        "     [atom true]",
        "     [atom nil][bracket )]",
        "",
        "[bracket (]",
        " [builtin foo]",
        " [atom :a]",
        " [number 1]",
        " [atom true]",
        " [atom nil][bracket )]",
        "",
        "[bracket (][builtin foo] [bracket [[][atom :a][bracket ]]]",
        "     [number 1]",
        "     [atom true]",
        "     [atom nil][bracket )]"
    );

    MT("should properly indent forms in vector literals",
        "[bracket [[][atom :a] [number 1] [atom true] [atom nil][bracket ]]]",
        "",
        "[bracket [[][atom :a]",
        " [number 1]",
        " [atom true]",
        " [atom nil][bracket ]]]",
        "",
        "[bracket [[][atom :a] [number 1]",
        " [atom true]",
        " [atom nil][bracket ]]]",
        "",
        "[bracket [[]",
        " [variable foo]",
        " [atom :a]",
        " [number 1]",
        " [atom true]",
        " [atom nil][bracket ]]]"
    );

    MT("should properly indent forms in map literals",
        "[bracket {][atom :a] [atom :a] [atom :b] [number 1] [atom :c] [atom true] [atom :d] [atom nil] [bracket }]",
        "",
        "[bracket {][atom :a] [atom :a]",
        " [atom :b] [number 1]",
        " [atom :c] [atom true]",
        " [atom :d] [atom nil][bracket }]",
        "",
        "[bracket {][atom :a]",
        " [atom :a]",
        " [atom :b]",
        " [number 1]",
        " [atom :c]",
        " [atom true]",
        " [atom :d]",
        " [atom nil][bracket }]",
        "",
        "[bracket {]",
        " [atom :a] [atom :a]",
        " [atom :b] [number 1]",
        " [atom :c] [atom true]",
        " [atom :d] [atom nil][bracket }]"
    );

    MT("should properly indent forms in set literals",
        "[meta #][bracket {][atom :a] [number 1] [atom true] [atom nil] [bracket }]",
        "",
        "[meta #][bracket {][atom :a]",
        "  [number 1]",
        "  [atom true]",
        "  [atom nil][bracket }]",
        "",
        "[meta #][bracket {]",
        "  [atom :a]",
        "  [number 1]",
        "  [atom true]",
        "  [atom nil][bracket }]"
    );

    var haveBodyParameter = [
        "->", "->>", "as->", "binding", "bound-fn", "case", "catch", "cond",
        "cond->", "cond->>", "condp", "def", "definterface", "defmethod", "defn",
        "defmacro", "defprotocol", "defrecord", "defstruct", "deftype", "do",
        "doseq", "dotimes", "doto", "extend", "extend-protocol", "extend-type",
        "fn", "for", "future", "if", "if-let", "if-not", "if-some", "let",
        "letfn", "locking", "loop", "ns", "proxy", "reify", "some->", "some->>",
        "struct-map", "try", "when", "when-first", "when-let", "when-not",
        "when-some", "while", "with-bindings", "with-bindings*", "with-in-str",
        "with-loading-context", "with-local-vars", "with-meta", "with-open",
        "with-out-str", "with-precision", "with-redefs", "with-redefs-fn"];

    function testFormsThatHaveBodyParameter(forms) {
        for (var i = 0; i < forms.length; i++) {
            MT("should indent body argument of `" + forms[i] + "` by `options.indentUnit` spaces",
                "[bracket (][keyword " + forms[i] + "] [variable foo] [variable bar]",
                "  [variable baz]",
                "  [variable qux][bracket )]"
            );
        }
    }

    testFormsThatHaveBodyParameter(haveBodyParameter);

    MT("should indent body argument of `comment` by `options.indentUnit` spaces",
        "[bracket (][comment comment foo bar]",
        "[comment  baz]",
        "[comment  qux][bracket )]"
    );

    function typeTokenPairs(type, tokens) {
        return "[" + type + " " + tokens.join("] [" + type + " ") + "]";
    }
})();
