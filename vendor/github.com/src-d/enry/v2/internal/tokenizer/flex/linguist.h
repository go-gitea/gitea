// https://github.com/github/linguist/blob/f72f2a21dfe80ebd16af3bc6216da75cd983a4f6/ext/linguist/linguist.h#L1
enum tokenizer_type {
  NO_ACTION,
  REGULAR_TOKEN,
  SHEBANG_TOKEN,
  SGML_TOKEN,
};

struct tokenizer_extra {
  char *token;
  enum tokenizer_type type;
};

// TODO(bzz) port Win support from
// https://github.com/github/linguist/commit/8e912b4d8bf2aef7948de59eba48b75cfcbc97e0