package s

import (
	"fmt"

	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// Scala lexer.
var Scala = internal.Register(MustNewLazyLexer(
	&Config{
		Name:      "Scala",
		Aliases:   []string{"scala"},
		Filenames: []string{"*.scala"},
		MimeTypes: []string{"text/x-scala"},
		DotAll:    true,
	},
	scalaRules,
))

func scalaRules() Rules {
	var (
		scalaOp     = "[-~\\^\\*!%&\\\\<>\\|+=:/?@\xa6-\xa7\xa9\xac\xae\xb0-\xb1\xb6\xd7\xf7\u03f6\u0482\u0606-\u0608\u060e-\u060f\u06e9\u06fd-\u06fe\u07f6\u09fa\u0b70\u0bf3-\u0bf8\u0bfa\u0c7f\u0cf1-\u0cf2\u0d79\u0f01-\u0f03\u0f13-\u0f17\u0f1a-\u0f1f\u0f34\u0f36\u0f38\u0fbe-\u0fc5\u0fc7-\u0fcf\u109e-\u109f\u1360\u1390-\u1399\u1940\u19e0-\u19ff\u1b61-\u1b6a\u1b74-\u1b7c\u2044\u2052\u207a-\u207c\u208a-\u208c\u2100-\u2101\u2103-\u2106\u2108-\u2109\u2114\u2116-\u2118\u211e-\u2123\u2125\u2127\u2129\u212e\u213a-\u213b\u2140-\u2144\u214a-\u214d\u214f\u2190-\u2328\u232b-\u244a\u249c-\u24e9\u2500-\u2767\u2794-\u27c4\u27c7-\u27e5\u27f0-\u2982\u2999-\u29d7\u29dc-\u29fb\u29fe-\u2b54\u2ce5-\u2cea\u2e80-\u2ffb\u3004\u3012-\u3013\u3020\u3036-\u3037\u303e-\u303f\u3190-\u3191\u3196-\u319f\u31c0-\u31e3\u3200-\u321e\u322a-\u3250\u3260-\u327f\u328a-\u32b0\u32c0-\u33ff\u4dc0-\u4dff\ua490-\ua4c6\ua828-\ua82b\ufb29\ufdfd\ufe62\ufe64-\ufe66\uff0b\uff1c-\uff1e\uff5c\uff5e\uffe2\uffe4\uffe8-\uffee\ufffc-\ufffd]+"
		scalaUpper  = "[A-Z\\$_\xc0-\xd6\xd8-\xde\u0100\u0102\u0104\u0106\u0108\u010a\u010c\u010e\u0110\u0112\u0114\u0116\u0118\u011a\u011c\u011e\u0120\u0122\u0124\u0126\u0128\u012a\u012c\u012e\u0130\u0132\u0134\u0136\u0139\u013b\u013d\u013f\u0141\u0143\u0145\u0147\u014a\u014c\u014e\u0150\u0152\u0154\u0156\u0158\u015a\u015c\u015e\u0160\u0162\u0164\u0166\u0168\u016a\u016c\u016e\u0170\u0172\u0174\u0176\u0178-\u0179\u017b\u017d\u0181-\u0182\u0184\u0186-\u0187\u0189-\u018b\u018e-\u0191\u0193-\u0194\u0196-\u0198\u019c-\u019d\u019f-\u01a0\u01a2\u01a4\u01a6-\u01a7\u01a9\u01ac\u01ae-\u01af\u01b1-\u01b3\u01b5\u01b7-\u01b8\u01bc\u01c4\u01c7\u01ca\u01cd\u01cf\u01d1\u01d3\u01d5\u01d7\u01d9\u01db\u01de\u01e0\u01e2\u01e4\u01e6\u01e8\u01ea\u01ec\u01ee\u01f1\u01f4\u01f6-\u01f8\u01fa\u01fc\u01fe\u0200\u0202\u0204\u0206\u0208\u020a\u020c\u020e\u0210\u0212\u0214\u0216\u0218\u021a\u021c\u021e\u0220\u0222\u0224\u0226\u0228\u022a\u022c\u022e\u0230\u0232\u023a-\u023b\u023d-\u023e\u0241\u0243-\u0246\u0248\u024a\u024c\u024e\u0370\u0372\u0376\u0386\u0388-\u038f\u0391-\u03ab\u03cf\u03d2-\u03d4\u03d8\u03da\u03dc\u03de\u03e0\u03e2\u03e4\u03e6\u03e8\u03ea\u03ec\u03ee\u03f4\u03f7\u03f9-\u03fa\u03fd-\u042f\u0460\u0462\u0464\u0466\u0468\u046a\u046c\u046e\u0470\u0472\u0474\u0476\u0478\u047a\u047c\u047e\u0480\u048a\u048c\u048e\u0490\u0492\u0494\u0496\u0498\u049a\u049c\u049e\u04a0\u04a2\u04a4\u04a6\u04a8\u04aa\u04ac\u04ae\u04b0\u04b2\u04b4\u04b6\u04b8\u04ba\u04bc\u04be\u04c0-\u04c1\u04c3\u04c5\u04c7\u04c9\u04cb\u04cd\u04d0\u04d2\u04d4\u04d6\u04d8\u04da\u04dc\u04de\u04e0\u04e2\u04e4\u04e6\u04e8\u04ea\u04ec\u04ee\u04f0\u04f2\u04f4\u04f6\u04f8\u04fa\u04fc\u04fe\u0500\u0502\u0504\u0506\u0508\u050a\u050c\u050e\u0510\u0512\u0514\u0516\u0518\u051a\u051c\u051e\u0520\u0522\u0531-\u0556\u10a0-\u10c5\u1e00\u1e02\u1e04\u1e06\u1e08\u1e0a\u1e0c\u1e0e\u1e10\u1e12\u1e14\u1e16\u1e18\u1e1a\u1e1c\u1e1e\u1e20\u1e22\u1e24\u1e26\u1e28\u1e2a\u1e2c\u1e2e\u1e30\u1e32\u1e34\u1e36\u1e38\u1e3a\u1e3c\u1e3e\u1e40\u1e42\u1e44\u1e46\u1e48\u1e4a\u1e4c\u1e4e\u1e50\u1e52\u1e54\u1e56\u1e58\u1e5a\u1e5c\u1e5e\u1e60\u1e62\u1e64\u1e66\u1e68\u1e6a\u1e6c\u1e6e\u1e70\u1e72\u1e74\u1e76\u1e78\u1e7a\u1e7c\u1e7e\u1e80\u1e82\u1e84\u1e86\u1e88\u1e8a\u1e8c\u1e8e\u1e90\u1e92\u1e94\u1e9e\u1ea0\u1ea2\u1ea4\u1ea6\u1ea8\u1eaa\u1eac\u1eae\u1eb0\u1eb2\u1eb4\u1eb6\u1eb8\u1eba\u1ebc\u1ebe\u1ec0\u1ec2\u1ec4\u1ec6\u1ec8\u1eca\u1ecc\u1ece\u1ed0\u1ed2\u1ed4\u1ed6\u1ed8\u1eda\u1edc\u1ede\u1ee0\u1ee2\u1ee4\u1ee6\u1ee8\u1eea\u1eec\u1eee\u1ef0\u1ef2\u1ef4\u1ef6\u1ef8\u1efa\u1efc\u1efe\u1f08-\u1f0f\u1f18-\u1f1d\u1f28-\u1f2f\u1f38-\u1f3f\u1f48-\u1f4d\u1f59-\u1f5f\u1f68-\u1f6f\u1fb8-\u1fbb\u1fc8-\u1fcb\u1fd8-\u1fdb\u1fe8-\u1fec\u1ff8-\u1ffb\u2102\u2107\u210b-\u210d\u2110-\u2112\u2115\u2119-\u211d\u2124\u2126\u2128\u212a-\u212d\u2130-\u2133\u213e-\u213f\u2145\u2183\u2c00-\u2c2e\u2c60\u2c62-\u2c64\u2c67\u2c69\u2c6b\u2c6d-\u2c6f\u2c72\u2c75\u2c80\u2c82\u2c84\u2c86\u2c88\u2c8a\u2c8c\u2c8e\u2c90\u2c92\u2c94\u2c96\u2c98\u2c9a\u2c9c\u2c9e\u2ca0\u2ca2\u2ca4\u2ca6\u2ca8\u2caa\u2cac\u2cae\u2cb0\u2cb2\u2cb4\u2cb6\u2cb8\u2cba\u2cbc\u2cbe\u2cc0\u2cc2\u2cc4\u2cc6\u2cc8\u2cca\u2ccc\u2cce\u2cd0\u2cd2\u2cd4\u2cd6\u2cd8\u2cda\u2cdc\u2cde\u2ce0\u2ce2\ua640\ua642\ua644\ua646\ua648\ua64a\ua64c\ua64e\ua650\ua652\ua654\ua656\ua658\ua65a\ua65c\ua65e\ua662\ua664\ua666\ua668\ua66a\ua66c\ua680\ua682\ua684\ua686\ua688\ua68a\ua68c\ua68e\ua690\ua692\ua694\ua696\ua722\ua724\ua726\ua728\ua72a\ua72c\ua72e\ua732\ua734\ua736\ua738\ua73a\ua73c\ua73e\ua740\ua742\ua744\ua746\ua748\ua74a\ua74c\ua74e\ua750\ua752\ua754\ua756\ua758\ua75a\ua75c\ua75e\ua760\ua762\ua764\ua766\ua768\ua76a\ua76c\ua76e\ua779\ua77b\ua77d-\ua77e\ua780\ua782\ua784\ua786\ua78b\uff21-\uff3a]"
		scalaLetter = `[a-zA-Z\\$_ªµºÀ-ÖØ-öø-ʯͰ-ͳͶ-ͷͻ-ͽΆΈ-ϵϷ-ҁҊ-Ֆա-ևא-ײء-ؿف-يٮ-ٯٱ-ۓەۮ-ۯۺ-ۼۿܐܒ-ܯݍ-ޥޱߊ-ߪऄ-हऽॐक़-ॡॲ-ॿঅ-হঽৎড়-ৡৰ-ৱਅ-ਹਖ਼-ਫ਼ੲ-ੴઅ-હઽૐ-ૡଅ-ହଽଡ଼-ୡୱஃ-ஹௐఅ-ఽౘ-ౡಅ-ಹಽೞ-ೡഅ-ഽൠ-ൡൺ-ൿඅ-ෆก-ะา-ำเ-ๅກ-ະາ-ຳຽ-ໄໜ-ༀཀ-ཬྈ-ྋက-ဪဿၐ-ၕၚ-ၝၡၥ-ၦၮ-ၰၵ-ႁႎႠ-ჺᄀ-ፚᎀ-ᎏᎠ-ᙬᙯ-ᙶᚁ-ᚚᚠ-ᛪᛮ-ᜑᜠ-ᜱᝀ-ᝑᝠ-ᝰក-ឳៜᠠ-ᡂᡄ-ᢨᢪ-ᤜᥐ-ᦩᧁ-ᧇᨀ-ᨖᬅ-ᬳᭅ-ᭋᮃ-ᮠᮮ-ᮯᰀ-ᰣᱍ-ᱏᱚ-ᱷᴀ-ᴫᵢ-ᵷᵹ-ᶚḀ-ᾼιῂ-ῌῐ-Ίῠ-Ῥῲ-ῼⁱⁿℂℇℊ-ℓℕℙ-ℝℤΩℨK-ℭℯ-ℹℼ-ℿⅅ-ⅉⅎⅠ-ↈⰀ-ⱼⲀ-ⳤⴀ-ⵥⶀ-ⷞ〆-〇〡-〩〸-〺〼ぁ-ゖゟァ-ヺヿ-ㆎㆠ-ㆷㇰ-ㇿ㐀-䶵一-ꀔꀖ-ꒌꔀ-ꘋꘐ-ꘟꘪ-ꙮꚀ-ꚗꜢ-ꝯꝱ-ꞇꞋ-ꠁꠃ-ꠅꠇ-ꠊꠌ-ꠢꡀ-ꡳꢂ-ꢳꤊ-ꤥꤰ-ꥆꨀ-ꨨꩀ-ꩂꩄ-ꩋ가-힣豈-יִײַ-ﬨשׁ-ﴽﵐ-ﷻﹰ-ﻼＡ-Ｚａ-ｚｦ-ｯｱ-ﾝﾠ-ￜ]`
		scalaIDRest = fmt.Sprintf(`%s(?:%s|[0-9])*(?:(?<=_)%s)?`, scalaLetter, scalaLetter, scalaOp)
	)

	return Rules{
		"root": {
			{`(class|trait|object)(\s+)`, ByGroups(Keyword, Text), Push("class")},
			{`[^\S\n]+`, Text, nil},
			{`//.*?\n`, CommentSingle, nil},
			{`/\*`, CommentMultiline, Push("comment")},
			{`@` + scalaIDRest, NameDecorator, nil},
			{`(abstract|ca(?:se|tch)|d(?:ef|o)|e(?:lse|xtends)|f(?:inal(?:ly)?|or(?:Some)?)|i(?:f|mplicit)|lazy|match|new|override|pr(?:ivate|otected)|re(?:quires|turn)|s(?:ealed|uper)|t(?:h(?:is|row)|ry)|va[lr]|w(?:hile|ith)|yield)\b|(<[%:-]|=>|>:|[#=@_⇒←])(\b|(?=\s)|$)`, Keyword, nil},
			{`:(?!` + scalaOp + `%s)`, Keyword, Push("type")},
			{fmt.Sprintf("%s%s\\b", scalaUpper, scalaIDRest), NameClass, nil},
			{`(true|false|null)\b`, KeywordConstant, nil},
			{`(import|package)(\s+)`, ByGroups(Keyword, Text), Push("import")},
			{`(type)(\s+)`, ByGroups(Keyword, Text), Push("type")},
			{`""".*?"""(?!")`, LiteralString, nil},
			{`"(\\\\|\\"|[^"])*"`, LiteralString, nil},
			{`'\\.'|'[^\\]'|'\\u[0-9a-fA-F]{4}'`, LiteralStringChar, nil},
			{"'" + scalaIDRest, TextSymbol, nil},
			{`[fs]"""`, LiteralString, Push("interptriplestring")},
			{`[fs]"`, LiteralString, Push("interpstring")},
			{`raw"(\\\\|\\"|[^"])*"`, LiteralString, nil},
			{scalaIDRest, Name, nil},
			{"`[^`]+`", Name, nil},
			{`\[`, Operator, Push("typeparam")},
			{`[(){};,.#]`, Operator, nil},
			{scalaOp, Operator, nil},
			{`([0-9][0-9]*\.[0-9]*|\.[0-9]+)([eE][+-]?[0-9]+)?[fFdD]?`, LiteralNumberFloat, nil},
			{`0x[0-9a-fA-F]+`, LiteralNumberHex, nil},
			{`[0-9]+L?`, LiteralNumberInteger, nil},
			{`\n`, Text, nil},
		},
		"class": {
			{fmt.Sprintf("(%s|%s|`[^`]+`)(\\s*)(\\[)", scalaIDRest, scalaOp), ByGroups(NameClass, Text, Operator), Push("typeparam")},
			{`\s+`, Text, nil},
			{`\{`, Operator, Pop(1)},
			{`\(`, Operator, Pop(1)},
			{`//.*?\n`, CommentSingle, Pop(1)},
			{fmt.Sprintf("%s|%s|`[^`]+`", scalaIDRest, scalaOp), NameClass, Pop(1)},
		},
		"type": {
			{`\s+`, Text, nil},
			{`<[%:]|>:|[#_]|forSome|type`, Keyword, nil},
			{`([,);}]|=>|=|⇒)(\s*)`, ByGroups(Operator, Text), Pop(1)},
			{`[({]`, Operator, Push()},
			{fmt.Sprintf("((?:%s|%s|`[^`]+`)(?:\\.(?:%s|%s|`[^`]+`))*)(\\s*)(\\[)", scalaIDRest, scalaOp, scalaIDRest, scalaOp), ByGroups(KeywordType, Text, Operator), Push("#pop", "typeparam")},
			{fmt.Sprintf("((?:%s|%s|`[^`]+`)(?:\\.(?:%s|%s|`[^`]+`))*)(\\s*)$", scalaIDRest, scalaOp, scalaIDRest, scalaOp), ByGroups(KeywordType, Text), Pop(1)},
			{`//.*?\n`, CommentSingle, Pop(1)},
			{fmt.Sprintf("\\.|%s|%s|`[^`]+`", scalaIDRest, scalaOp), KeywordType, nil},
		},
		"typeparam": {
			{`[\s,]+`, Text, nil},
			{`<[%:]|=>|>:|[#_⇒]|forSome|type`, Keyword, nil},
			{`([\])}])`, Operator, Pop(1)},
			{`[(\[{]`, Operator, Push()},
			{fmt.Sprintf("\\.|%s|%s|`[^`]+`", scalaIDRest, scalaOp), KeywordType, nil},
		},
		"comment": {
			{`[^/*]+`, CommentMultiline, nil},
			{`/\*`, CommentMultiline, Push()},
			{`\*/`, CommentMultiline, Pop(1)},
			{`[*/]`, CommentMultiline, nil},
		},
		"import": {
			{fmt.Sprintf("(%s|\\.)+", scalaIDRest), NameNamespace, Pop(1)},
		},
		"interpstringcommon": {
			{`[^"$\\]+`, LiteralString, nil},
			{`\$\$`, LiteralString, nil},
			{`\$` + scalaLetter + `(?:` + scalaLetter + `|\d)*`, LiteralStringInterpol, nil},
			{`\$\{`, LiteralStringInterpol, Push("interpbrace")},
			{`\\.`, LiteralString, nil},
		},
		"interptriplestring": {
			{`"""(?!")`, LiteralString, Pop(1)},
			{`"`, LiteralString, nil},
			Include("interpstringcommon"),
		},
		"interpstring": {
			{`"`, LiteralString, Pop(1)},
			Include("interpstringcommon"),
		},
		"interpbrace": {
			{`\}`, LiteralStringInterpol, Pop(1)},
			{`\{`, LiteralStringInterpol, Push()},
			Include("root"),
		},
	}
}
