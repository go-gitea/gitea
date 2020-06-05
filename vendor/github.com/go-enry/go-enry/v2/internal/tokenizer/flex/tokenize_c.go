// +build flex

package flex

// #include <stdlib.h>
// #include "linguist.h"
// #include "lex.linguist_yy.h"
// int linguist_yywrap(yyscan_t yyscanner) {
// 	return 1;
// }
import "C"
import "unsafe"

const maxTokenLen = 32 // bytes

// TokenizeFlex implements tokenizer by calling Flex generated code from linguist in C
// This is a transliteration from C https://github.com/github/linguist/blob/master/ext/linguist/linguist.c#L12
func TokenizeFlex(content []byte) []string {
	var buf C.YY_BUFFER_STATE
	var scanner C.yyscan_t
	var extra C.struct_tokenizer_extra
	var _len C.ulong
	var r C.int

	_len = C.ulong(len(content))
	cs := C.CBytes(content)
	defer C.free(unsafe.Pointer(cs))

	C.linguist_yylex_init_extra(&extra, &scanner)
	buf = C.linguist_yy_scan_bytes((*C.char)(cs), _len, scanner)

	ary := []string{}
	for {
		extra._type = C.NO_ACTION
		extra.token = nil
		r = C.linguist_yylex(scanner)
		switch extra._type {
		case C.NO_ACTION:
			break
		case C.REGULAR_TOKEN:
			_len = C.strlen(extra.token)
			if _len <= maxTokenLen {
				ary = append(ary, C.GoStringN(extra.token, (C.int)(_len)))
			}
			C.free(unsafe.Pointer(extra.token))
			break
		case C.SHEBANG_TOKEN:
			_len = C.strlen(extra.token)
			if _len <= maxTokenLen {
				s := "SHEBANG#!" + C.GoStringN(extra.token, (C.int)(_len))
				ary = append(ary, s)
			}
			C.free(unsafe.Pointer(extra.token))
			break
		case C.SGML_TOKEN:
			_len = C.strlen(extra.token)
			if _len <= maxTokenLen {
				s := C.GoStringN(extra.token, (C.int)(_len)) + ">"
				ary = append(ary, s)
			}
			C.free(unsafe.Pointer(extra.token))
			break
		}
		if r == 0 {
			break
		}
	}

	C.linguist_yy_delete_buffer(buf, scanner)
	C.linguist_yylex_destroy(scanner)

	return ary
}
