package p

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

var python3Identifier = `[A-Z_a-zªµºÀ-ÖØ-öø-ˁˆ-ˑˠ-ˤˬˮͰ-ʹͶ-ͷͻ-ͽΆΈ-ΊΌΎ-ΡΣ-ϵϷ-ҁҊ-ԧԱ-Ֆՙա-ևא-תװ-ײؠ-يٮ-ٯٱ-ۓەۥ-ۦۮ-ۯۺ-ۼۿܐܒ-ܯݍ-ޥޱߊ-ߪߴ-ߵߺࠀ-ࠕࠚࠤࠨࡀ-ࡘࢠࢢ-ࢬऄ-हऽॐक़-ॡॱ-ॷॹ-ॿঅ-ঌএ-ঐও-নপ-রলশ-হঽৎড়-ঢ়য়-ৡৰ-ৱਅ-ਊਏ-ਐਓ-ਨਪ-ਰਲ-ਲ਼ਵ-ਸ਼ਸ-ਹਖ਼-ੜਫ਼ੲ-ੴઅ-ઍએ-ઑઓ-નપ-રલ-ળવ-હઽૐૠ-ૡଅ-ଌଏ-ଐଓ-ନପ-ରଲ-ଳଵ-ହଽଡ଼-ଢ଼ୟ-ୡୱஃஅ-ஊஎ-ஐஒ-கங-சஜஞ-டண-தந-பம-ஹௐఅ-ఌఎ-ఐఒ-నప-ళవ-హఽౘ-ౙౠ-ౡಅ-ಌಎ-ಐಒ-ನಪ-ಳವ-ಹಽೞೠ-ೡೱ-ೲഅ-ഌഎ-ഐഒ-ഺഽൎൠ-ൡൺ-ൿඅ-ඖක-නඳ-රලව-ෆก-ะาเ-ๆກ-ຂຄງ-ຈຊຍດ-ທນ-ຟມ-ຣລວສ-ຫອ-ະາຽເ-ໄໆໜ-ໟༀཀ-ཇཉ-ཬྈ-ྌက-ဪဿၐ-ၕၚ-ၝၡၥ-ၦၮ-ၰၵ-ႁႎႠ-ჅჇჍა-ჺჼ-ቈቊ-ቍቐ-ቖቘቚ-ቝበ-ኈኊ-ኍነ-ኰኲ-ኵኸ-ኾዀዂ-ዅወ-ዖዘ-ጐጒ-ጕጘ-ፚᎀ-ᎏᎠ-Ᏼᐁ-ᙬᙯ-ᙿᚁ-ᚚᚠ-ᛪᛮ-ᛰᜀ-ᜌᜎ-ᜑᜠ-ᜱᝀ-ᝑᝠ-ᝬᝮ-ᝰក-ឳៗៜᠠ-ᡷᢀ-ᢨᢪᢰ-ᣵᤀ-ᤜᥐ-ᥭᥰ-ᥴᦀ-ᦫᧁ-ᧇᨀ-ᨖᨠ-ᩔᪧᬅ-ᬳᭅ-ᭋᮃ-ᮠᮮ-ᮯᮺ-ᯥᰀ-ᰣᱍ-ᱏᱚ-ᱽᳩ-ᳬᳮ-ᳱᳵ-ᳶᴀ-ᶿḀ-ἕἘ-Ἕἠ-ὅὈ-Ὅὐ-ὗὙὛὝὟ-ώᾀ-ᾴᾶ-ᾼιῂ-ῄῆ-ῌῐ-ΐῖ-Ίῠ-Ῥῲ-ῴῶ-ῼⁱⁿₐ-ₜℂℇℊ-ℓℕ℘-ℝℤΩℨK-ℹℼ-ℿⅅ-ⅉⅎⅠ-ↈⰀ-Ⱞⰰ-ⱞⱠ-ⳤⳫ-ⳮⳲ-ⳳⴀ-ⴥⴧⴭⴰ-ⵧⵯⶀ-ⶖⶠ-ⶦⶨ-ⶮⶰ-ⶶⶸ-ⶾⷀ-ⷆⷈ-ⷎⷐ-ⷖⷘ-ⷞ々-〇〡-〩〱-〵〸-〼ぁ-ゖゝ-ゟァ-ヺー-ヿㄅ-ㄭㄱ-ㆎㆠ-ㆺㇰ-ㇿ㐀-䶵一-鿌ꀀ-ꒌꓐ-ꓽꔀ-ꘌꘐ-ꘟꘪ-ꘫꙀ-ꙮꙿ-ꚗꚠ-ꛯꜗ-ꜟꜢ-ꞈꞋ-ꞎꞐ-ꞓꞠ-Ɦꟸ-ꠁꠃ-ꠅꠇ-ꠊꠌ-ꠢꡀ-ꡳꢂ-ꢳꣲ-ꣷꣻꤊ-ꤥꤰ-ꥆꥠ-ꥼꦄ-ꦲꧏꨀ-ꨨꩀ-ꩂꩄ-ꩋꩠ-ꩶꩺꪀ-ꪯꪱꪵ-ꪶꪹ-ꪽꫀꫂꫛ-ꫝꫠ-ꫪꫲ-ꫴꬁ-ꬆꬉ-ꬎꬑ-ꬖꬠ-ꬦꬨ-ꬮꯀ-ꯢ가-힣ힰ-ퟆퟋ-ퟻ豈-舘並-龎ﬀ-ﬆﬓ-ﬗיִײַ-ﬨשׁ-זּטּ-לּמּנּ-סּףּ-פּצּ-ﮱﯓ-ﱝﱤ-ﴽﵐ-ﶏﶒ-ﷇﷰ-ﷹﹱﹳﹷﹹﹻﹽﹿ-ﻼＡ-Ｚａ-ｚｦ-ﾝﾠ-ﾾￂ-ￇￊ-ￏￒ-ￗￚ-ￜ𐀀-𐀋𐀍-𐀦𐀨-𐀺𐀼-𐀽𐀿-𐁍𐁐-𐁝𐂀-𐃺𐅀-𐅴𐊀-𐊜𐊠-𐋐𐌀-𐌞𐌰-𐍊𐎀-𐎝𐎠-𐏃𐏈-𐏏𐏑-𐏕𐐀-𐒝𐠀-𐠅𐠈𐠊-𐠵𐠷-𐠸𐠼𐠿-𐡕𐤀-𐤕𐤠-𐤹𐦀-𐦷𐦾-𐦿𐨀𐨐-𐨓𐨕-𐨗𐨙-𐨳𐩠-𐩼𐬀-𐬵𐭀-𐭕𐭠-𐭲𐰀-𐱈𑀃-𑀷𑂃-𑂯𑃐-𑃨𑄃-𑄦𑆃-𑆲𑇁-𑇄𑚀-𑚪𒀀-𒍮𒐀-𒑢𓀀-𓐮𖠀-𖨸𖼀-𖽄𖽐𖾓-𖾟𛀀-𛀁𝐀-𝑔𝑖-𝒜𝒞-𝒟𝒢𝒥-𝒦𝒩-𝒬𝒮-𝒹𝒻𝒽-𝓃𝓅-𝔅𝔇-𝔊𝔍-𝔔𝔖-𝔜𝔞-𝔹𝔻-𝔾𝕀-𝕄𝕆𝕊-𝕐𝕒-𝚥𝚨-𝛀𝛂-𝛚𝛜-𝛺𝛼-𝜔𝜖-𝜴𝜶-𝝎𝝐-𝝮𝝰-𝞈𝞊-𝞨𝞪-𝟂𝟄-𝟋𞸀-𞸃𞸅-𞸟𞸡-𞸢𞸤𞸧𞸩-𞸲𞸴-𞸷𞸹𞸻𞹂𞹇𞹉𞹋𞹍-𞹏𞹑-𞹒𞹔𞹗𞹙𞹛𞹝𞹟𞹡-𞹢𞹤𞹧-𞹪𞹬-𞹲𞹴-𞹷𞹹-𞹼𞹾𞺀-𞺉𞺋-𞺛𞺡-𞺣𞺥-𞺩𞺫-𞺻𠀀-𪛖𪜀-𫜴𫝀-𫠝丽-𪘀][0-9A-Z_a-zªµ·ºÀ-ÖØ-öø-ˁˆ-ˑˠ-ˤˬˮ̀-ʹͶ-ͷͻ-ͽΆ-ΊΌΎ-ΡΣ-ϵϷ-ҁ҃-҇Ҋ-ԧԱ-Ֆՙա-և֑-ֽֿׁ-ׂׄ-ׇׅא-תװ-ײؐ-ؚؠ-٩ٮ-ۓە-ۜ۟-۪ۨ-ۼۿܐ-݊ݍ-ޱ߀-ߵߺࠀ-࠭ࡀ-࡛ࢠࢢ-ࢬࣤ-ࣾऀ-ॣ०-९ॱ-ॷॹ-ॿঁ-ঃঅ-ঌএ-ঐও-নপ-রলশ-হ়-ৄে-ৈো-ৎৗড়-ঢ়য়-ৣ০-ৱਁ-ਃਅ-ਊਏ-ਐਓ-ਨਪ-ਰਲ-ਲ਼ਵ-ਸ਼ਸ-ਹ਼ਾ-ੂੇ-ੈੋ-੍ੑਖ਼-ੜਫ਼੦-ੵઁ-ઃઅ-ઍએ-ઑઓ-નપ-રલ-ળવ-હ઼-ૅે-ૉો-્ૐૠ-ૣ૦-૯ଁ-ଃଅ-ଌଏ-ଐଓ-ନପ-ରଲ-ଳଵ-ହ଼-ୄେ-ୈୋ-୍ୖ-ୗଡ଼-ଢ଼ୟ-ୣ୦-୯ୱஂ-ஃஅ-ஊஎ-ஐஒ-கங-சஜஞ-டண-தந-பம-ஹா-ூெ-ைொ-்ௐௗ௦-௯ఁ-ఃఅ-ఌఎ-ఐఒ-నప-ళవ-హఽ-ౄె-ైొ-్ౕ-ౖౘ-ౙౠ-ౣ౦-౯ಂ-ಃಅ-ಌಎ-ಐಒ-ನಪ-ಳವ-ಹ಼-ೄೆ-ೈೊ-್ೕ-ೖೞೠ-ೣ೦-೯ೱ-ೲം-ഃഅ-ഌഎ-ഐഒ-ഺഽ-ൄെ-ൈൊ-ൎൗൠ-ൣ൦-൯ൺ-ൿං-ඃඅ-ඖක-නඳ-රලව-ෆ්ා-ුූෘ-ෟෲ-ෳก-ฺเ-๎๐-๙ກ-ຂຄງ-ຈຊຍດ-ທນ-ຟມ-ຣລວສ-ຫອ-ູົ-ຽເ-ໄໆ່-ໍ໐-໙ໜ-ໟༀ༘-༙༠-༩༹༵༷༾-ཇཉ-ཬཱ-྄྆-ྗྙ-ྼ࿆က-၉ၐ-ႝႠ-ჅჇჍა-ჺჼ-ቈቊ-ቍቐ-ቖቘቚ-ቝበ-ኈኊ-ኍነ-ኰኲ-ኵኸ-ኾዀዂ-ዅወ-ዖዘ-ጐጒ-ጕጘ-ፚ፝-፟፩-፱ᎀ-ᎏᎠ-Ᏼᐁ-ᙬᙯ-ᙿᚁ-ᚚᚠ-ᛪᛮ-ᛰᜀ-ᜌᜎ-᜔ᜠ-᜴ᝀ-ᝓᝠ-ᝬᝮ-ᝰᝲ-ᝳក-៓ៗៜ-៝០-៩᠋-᠍᠐-᠙ᠠ-ᡷᢀ-ᢪᢰ-ᣵᤀ-ᤜᤠ-ᤫᤰ-᤻᥆-ᥭᥰ-ᥴᦀ-ᦫᦰ-ᧉ᧐-᧚ᨀ-ᨛᨠ-ᩞ᩠-᩿᩼-᪉᪐-᪙ᪧᬀ-ᭋ᭐-᭙᭫-᭳ᮀ-᯳ᰀ-᰷᱀-᱉ᱍ-ᱽ᳐-᳔᳒-ᳶᴀ-ᷦ᷼-ἕἘ-Ἕἠ-ὅὈ-Ὅὐ-ὗὙὛὝὟ-ώᾀ-ᾴᾶ-ᾼιῂ-ῄῆ-ῌῐ-ΐῖ-Ίῠ-Ῥῲ-ῴῶ-ῼ‿-⁀⁔ⁱⁿₐ-ₜ⃐-⃥⃜⃡-⃰ℂℇℊ-ℓℕ℘-ℝℤΩℨK-ℹℼ-ℿⅅ-ⅉⅎⅠ-ↈⰀ-Ⱞⰰ-ⱞⱠ-ⳤⳫ-ⳳⴀ-ⴥⴧⴭⴰ-ⵧⵯ⵿-ⶖⶠ-ⶦⶨ-ⶮⶰ-ⶶⶸ-ⶾⷀ-ⷆⷈ-ⷎⷐ-ⷖⷘ-ⷞⷠ-ⷿ々-〇〡-〯〱-〵〸-〼ぁ-ゖ゙-゚ゝ-ゟァ-ヺー-ヿㄅ-ㄭㄱ-ㆎㆠ-ㆺㇰ-ㇿ㐀-䶵一-鿌ꀀ-ꒌꓐ-ꓽꔀ-ꘌꘐ-ꘫꙀ-꙯ꙴ-꙽ꙿ-ꚗꚟ-꛱ꜗ-ꜟꜢ-ꞈꞋ-ꞎꞐ-ꞓꞠ-Ɦꟸ-ꠧꡀ-ꡳꢀ-꣄꣐-꣙꣠-ꣷꣻ꤀-꤭ꤰ-꥓ꥠ-ꥼꦀ-꧀ꧏ-꧙ꨀ-ꨶꩀ-ꩍ꩐-꩙ꩠ-ꩶꩺ-ꩻꪀ-ꫂꫛ-ꫝꫠ-ꫯꫲ-꫶ꬁ-ꬆꬉ-ꬎꬑ-ꬖꬠ-ꬦꬨ-ꬮꯀ-ꯪ꯬-꯭꯰-꯹가-힣ힰ-ퟆퟋ-ퟻ豈-舘並-龎ﬀ-ﬆﬓ-ﬗיִ-ﬨשׁ-זּטּ-לּמּנּ-סּףּ-פּצּ-ﮱﯓ-ﱝﱤ-ﴽﵐ-ﶏﶒ-ﷇﷰ-ﷹ︀-️︠-︦︳-︴﹍-﹏ﹱﹳﹷﹹﹻﹽﹿ-ﻼ０-９Ａ-Ｚ＿ａ-ｚｦ-ﾾￂ-ￇￊ-ￏￒ-ￗￚ-ￜ𐀀-𐀋𐀍-𐀦𐀨-𐀺𐀼-𐀽𐀿-𐁍𐁐-𐁝𐂀-𐃺𐅀-𐅴𐇽𐊀-𐊜𐊠-𐋐𐌀-𐌞𐌰-𐍊𐎀-𐎝𐎠-𐏃𐏈-𐏏𐏑-𐏕𐐀-𐒝𐒠-𐒩𐠀-𐠅𐠈𐠊-𐠵𐠷-𐠸𐠼𐠿-𐡕𐤀-𐤕𐤠-𐤹𐦀-𐦷𐦾-𐦿𐨀-𐨃𐨅-𐨆𐨌-𐨓𐨕-𐨗𐨙-𐨳𐨸-𐨿𐨺𐩠-𐩼𐬀-𐬵𐭀-𐭕𐭠-𐭲𐰀-𐱈𑀀-𑁆𑁦-𑁯𑂀-𑂺𑃐-𑃨𑃰-𑃹𑄀-𑄴𑄶-𑄿𑆀-𑇄𑇐-𑇙𑚀-𑚷𑛀-𑛉𒀀-𒍮𒐀-𒑢𓀀-𓐮𖠀-𖨸𖼀-𖽄𖽐-𖽾𖾏-𖾟𛀀-𛀁𝅥-𝅩𝅭-𝅲𝅻-𝆂𝆅-𝆋𝆪-𝆭𝉂-𝉄𝐀-𝑔𝑖-𝒜𝒞-𝒟𝒢𝒥-𝒦𝒩-𝒬𝒮-𝒹𝒻𝒽-𝓃𝓅-𝔅𝔇-𝔊𝔍-𝔔𝔖-𝔜𝔞-𝔹𝔻-𝔾𝕀-𝕄𝕆𝕊-𝕐𝕒-𝚥𝚨-𝛀𝛂-𝛚𝛜-𝛺𝛼-𝜔𝜖-𝜴𝜶-𝝎𝝐-𝝮𝝰-𝞈𝞊-𝞨𝞪-𝟂𝟄-𝟋𝟎-𝟿𞸀-𞸃𞸅-𞸟𞸡-𞸢𞸤𞸧𞸩-𞸲𞸴-𞸷𞸹𞸻𞹂𞹇𞹉𞹋𞹍-𞹏𞹑-𞹒𞹔𞹗𞹙𞹛𞹝𞹟𞹡-𞹢𞹤𞹧-𞹪𞹬-𞹲𞹴-𞹷𞹹-𞹼𞹾𞺀-𞺉𞺋-𞺛𞺡-𞺣𞺥-𞺩𞺫-𞺻𠀀-𪛖𪜀-𫜴𫝀-𫠝丽-𪘀󠄀-󠇯]*`

// Python3 lexer.
var Python3 = internal.Register(MustNewLexer(
	&Config{
		Name:      "Python 3",
		Aliases:   []string{"python3", "py3"},
		Filenames: []string{},
		MimeTypes: []string{"text/x-python3", "application/x-python3"},
	},
	Rules{
		"root": {
			{`\n`, Text, nil},
			{`^(\s*)([rRuUbB]{,2})("""(?:.|\n)*?""")`, ByGroups(Text, LiteralStringAffix, LiteralStringDoc), nil},
			{`^(\s*)([rRuUbB]{,2})('''(?:.|\n)*?''')`, ByGroups(Text, LiteralStringAffix, LiteralStringDoc), nil},
			{`[^\S\n]+`, Text, nil},
			{`\A#!.+$`, CommentHashbang, nil},
			{`#.*$`, CommentSingle, nil},
			{`[]{}:(),;[]`, Punctuation, nil},
			{`\\\n`, Text, nil},
			{`\\`, Text, nil},
			{`(in|is|and|or|not)\b`, OperatorWord, nil},
			{`!=|==|<<|>>|[-~+/*%=<>&^|.]`, Operator, nil},
			Include("keywords"),
			{`(def)((?:\s|\\\s)+)`, ByGroups(Keyword, Text), Push("funcname")},
			{`(class)((?:\s|\\\s)+)`, ByGroups(Keyword, Text), Push("classname")},
			{`(from)((?:\s|\\\s)+)`, ByGroups(KeywordNamespace, Text), Push("fromimport")},
			{`(import)((?:\s|\\\s)+)`, ByGroups(KeywordNamespace, Text), Push("import")},
			Include("builtins"),
			Include("magicfuncs"),
			Include("magicvars"),
			Include("backtick"),
			{`([rR]|[uUbB][rR]|[rR][uUbB])(""")`, ByGroups(LiteralStringAffix, LiteralStringDouble), Push("tdqs")},
			{`([rR]|[uUbB][rR]|[rR][uUbB])(''')`, ByGroups(LiteralStringAffix, LiteralStringSingle), Push("tsqs")},
			{`([rR]|[uUbB][rR]|[rR][uUbB])(")`, ByGroups(LiteralStringAffix, LiteralStringDouble), Push("dqs")},
			{`([rR]|[uUbB][rR]|[rR][uUbB])(')`, ByGroups(LiteralStringAffix, LiteralStringSingle), Push("sqs")},
			{`([uUbB]?)(""")`, ByGroups(LiteralStringAffix, LiteralStringDouble), Combined("stringescape", "tdqs")},
			{`([uUbB]?)(''')`, ByGroups(LiteralStringAffix, LiteralStringSingle), Combined("stringescape", "tsqs")},
			{`([uUbB]?)(")`, ByGroups(LiteralStringAffix, LiteralStringDouble), Combined("stringescape", "dqs")},
			{`([uUbB]?)(')`, ByGroups(LiteralStringAffix, LiteralStringSingle), Combined("stringescape", "sqs")},
			Include("name"),
			Include("numbers"),
		},
		"keywords": {
			{Words(``, `\b`, `assert`, `async`, `await`, `break`, `continue`, `del`, `elif`, `else`, `except`, `finally`, `for`, `global`, `if`, `lambda`, `pass`, `raise`, `nonlocal`, `return`, `try`, `while`, `yield`, `yield from`, `as`, `with`), Keyword, nil},
			{Words(``, `\b`, `True`, `False`, `None`), KeywordConstant, nil},
		},
		"builtins": {
			{Words(`(?<!\.)`, `\b`, `__import__`, `abs`, `all`, `any`, `bin`, `bool`, `bytearray`, `bytes`, `chr`, `classmethod`, `cmp`, `compile`, `complex`, `delattr`, `dict`, `dir`, `divmod`, `enumerate`, `eval`, `filter`, `float`, `format`, `frozenset`, `getattr`, `globals`, `hasattr`, `hash`, `hex`, `id`, `input`, `int`, `isinstance`, `issubclass`, `iter`, `len`, `list`, `locals`, `map`, `max`, `memoryview`, `min`, `next`, `object`, `oct`, `open`, `ord`, `pow`, `print`, `property`, `range`, `repr`, `reversed`, `round`, `set`, `setattr`, `slice`, `sorted`, `staticmethod`, `str`, `sum`, `super`, `tuple`, `type`, `vars`, `zip`), NameBuiltin, nil},
			{`(?<!\.)(self|Ellipsis|NotImplemented|cls)\b`, NameBuiltinPseudo, nil},
			{Words(`(?<!\.)`, `\b`, `ArithmeticError`, `AssertionError`, `AttributeError`, `BaseException`, `BufferError`, `BytesWarning`, `DeprecationWarning`, `EOFError`, `EnvironmentError`, `Exception`, `FloatingPointError`, `FutureWarning`, `GeneratorExit`, `IOError`, `ImportError`, `ImportWarning`, `IndentationError`, `IndexError`, `KeyError`, `KeyboardInterrupt`, `LookupError`, `MemoryError`, `NameError`, `NotImplementedError`, `OSError`, `OverflowError`, `PendingDeprecationWarning`, `ReferenceError`, `ResourceWarning`, `RuntimeError`, `RuntimeWarning`, `StopIteration`, `SyntaxError`, `SyntaxWarning`, `SystemError`, `SystemExit`, `TabError`, `TypeError`, `UnboundLocalError`, `UnicodeDecodeError`, `UnicodeEncodeError`, `UnicodeError`, `UnicodeTranslateError`, `UnicodeWarning`, `UserWarning`, `ValueError`, `VMSError`, `Warning`, `WindowsError`, `ZeroDivisionError`, `BlockingIOError`, `ChildProcessError`, `ConnectionError`, `BrokenPipeError`, `ConnectionAbortedError`, `ConnectionRefusedError`, `ConnectionResetError`, `FileExistsError`, `FileNotFoundError`, `InterruptedError`, `IsADirectoryError`, `NotADirectoryError`, `PermissionError`, `ProcessLookupError`, `TimeoutError`), NameException, nil},
		},
		"magicfuncs": {
			{Words(``, `\b`, `__abs__`, `__add__`, `__aenter__`, `__aexit__`, `__aiter__`, `__and__`, `__anext__`, `__await__`, `__bool__`, `__bytes__`, `__call__`, `__complex__`, `__contains__`, `__del__`, `__delattr__`, `__delete__`, `__delitem__`, `__dir__`, `__divmod__`, `__enter__`, `__eq__`, `__exit__`, `__float__`, `__floordiv__`, `__format__`, `__ge__`, `__get__`, `__getattr__`, `__getattribute__`, `__getitem__`, `__gt__`, `__hash__`, `__iadd__`, `__iand__`, `__ifloordiv__`, `__ilshift__`, `__imatmul__`, `__imod__`, `__import__`, `__imul__`, `__index__`, `__init__`, `__instancecheck__`, `__int__`, `__invert__`, `__ior__`, `__ipow__`, `__irshift__`, `__isub__`, `__iter__`, `__itruediv__`, `__ixor__`, `__le__`, `__len__`, `__length_hint__`, `__lshift__`, `__lt__`, `__matmul__`, `__missing__`, `__mod__`, `__mul__`, `__ne__`, `__neg__`, `__new__`, `__next__`, `__or__`, `__pos__`, `__pow__`, `__prepare__`, `__radd__`, `__rand__`, `__rdivmod__`, `__repr__`, `__reversed__`, `__rfloordiv__`, `__rlshift__`, `__rmatmul__`, `__rmod__`, `__rmul__`, `__ror__`, `__round__`, `__rpow__`, `__rrshift__`, `__rshift__`, `__rsub__`, `__rtruediv__`, `__rxor__`, `__set__`, `__setattr__`, `__setitem__`, `__str__`, `__sub__`, `__subclasscheck__`, `__truediv__`, `__xor__`), NameFunctionMagic, nil},
		},
		"magicvars": {
			{Words(``, `\b`, `__annotations__`, `__bases__`, `__class__`, `__closure__`, `__code__`, `__defaults__`, `__dict__`, `__doc__`, `__file__`, `__func__`, `__globals__`, `__kwdefaults__`, `__module__`, `__mro__`, `__name__`, `__objclass__`, `__qualname__`, `__self__`, `__slots__`, `__weakref__`), NameVariableMagic, nil},
		},
		"numbers": {
			{`(\d+\.\d*|\d*\.\d+)([eE][+-]?[0-9]+)?`, LiteralNumberFloat, nil},
			{`\d+[eE][+-]?[0-9]+j?`, LiteralNumberFloat, nil},
			{`0[oO][0-7]+`, LiteralNumberOct, nil},
			{`0[bB][01]+`, LiteralNumberBin, nil},
			{`0[xX][a-fA-F0-9]+`, LiteralNumberHex, nil},
			{`\d+`, LiteralNumberInteger, nil},
		},
		"backtick": {},
		"name": {
			{`@\w+`, NameDecorator, nil},
			{`@`, Operator, nil},
			{python3Identifier, Name, nil},
		},
		"funcname": {
			{python3Identifier, NameFunction, Pop(1)},
		},
		"classname": {
			{python3Identifier, NameClass, Pop(1)},
		},
		"import": {
			{`(\s+)(as)(\s+)`, ByGroups(Text, Keyword, Text), nil},
			{`\.`, NameNamespace, nil},
			{python3Identifier, NameNamespace, nil},
			{`(\s*)(,)(\s*)`, ByGroups(Text, Operator, Text), nil},
			Default(Pop(1)),
		},
		"fromimport": {
			{`(\s+)(import)\b`, ByGroups(Text, Keyword), Pop(1)},
			{`\.`, NameNamespace, nil},
			{python3Identifier, NameNamespace, nil},
			Default(Pop(1)),
		},
		"stringescape": {
			{`\\([\\abfnrtv"\']|\n|N\{.*?\}|u[a-fA-F0-9]{4}|U[a-fA-F0-9]{8}|x[a-fA-F0-9]{2}|[0-7]{1,3})`, LiteralStringEscape, nil},
		},
		"strings-single": {
			{`%(\(\w+\))?[-#0 +]*([0-9]+|[*])?(\.([0-9]+|[*]))?[hlL]?[E-GXc-giorsux%]`, LiteralStringInterpol, nil},
			{`\{((\w+)((\.\w+)|(\[[^\]]+\]))*)?(\![sra])?(\:(.?[<>=\^])?[-+ ]?#?0?(\d+)?,?(\.\d+)?[E-GXb-gnosx%]?)?\}`, LiteralStringInterpol, nil},
			{`[^\\\'"%{\n]+`, LiteralStringSingle, nil},
			{`[\'"\\]`, LiteralStringSingle, nil},
			{`%|(\{{1,2})`, LiteralStringSingle, nil},
		},
		"strings-double": {
			{`%(\(\w+\))?[-#0 +]*([0-9]+|[*])?(\.([0-9]+|[*]))?[hlL]?[E-GXc-giorsux%]`, LiteralStringInterpol, nil},
			{`\{((\w+)((\.\w+)|(\[[^\]]+\]))*)?(\![sra])?(\:(.?[<>=\^])?[-+ ]?#?0?(\d+)?,?(\.\d+)?[E-GXb-gnosx%]?)?\}`, LiteralStringInterpol, nil},
			{`[^\\\'"%{\n]+`, LiteralStringDouble, nil},
			{`[\'"\\]`, LiteralStringDouble, nil},
			{`%|(\{{1,2})`, LiteralStringDouble, nil},
		},
		"dqs": {
			{`"`, LiteralStringDouble, Pop(1)},
			{`\\\\|\\"|\\\n`, LiteralStringEscape, nil},
			Include("strings-double"),
		},
		"sqs": {
			{`'`, LiteralStringSingle, Pop(1)},
			{`\\\\|\\'|\\\n`, LiteralStringEscape, nil},
			Include("strings-single"),
		},
		"tdqs": {
			{`"""`, LiteralStringDouble, Pop(1)},
			Include("strings-double"),
			{`\n`, LiteralStringDouble, nil},
		},
		"tsqs": {
			{`'''`, LiteralStringSingle, Pop(1)},
			Include("strings-single"),
			{`\n`, LiteralStringSingle, nil},
		},
	},
))
