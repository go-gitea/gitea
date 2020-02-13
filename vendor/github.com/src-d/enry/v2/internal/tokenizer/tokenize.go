// +build !flex

package tokenizer

import (
	"bytes"

	"github.com/src-d/enry/v2/regex"
)

// Tokenize returns lexical tokens from content. The tokens returned match what
// the Linguist library returns. At most the first ByteLimit bytes of content are tokenized.
//
// BUG: Until https://github.com/src-d/enry/issues/193 is resolved, there are some
// differences between this function and the Linguist output.
func Tokenize(content []byte) []string {
	if len(content) > ByteLimit {
		content = content[:ByteLimit]
	}

	// Copy the input so that changes wrought by the tokenization steps do not
	// modify the caller's copy of the input. See #196.
	content = append([]byte(nil), content...)

	tokens := make([][]byte, 0, 50)
	for _, extract := range extractTokens {
		var extractedTokens [][]byte
		content, extractedTokens = extract(content)
		tokens = append(tokens, extractedTokens...)
	}

	return toString(tokens)
}

func toString(tokens [][]byte) []string {
	stokens := make([]string, 0, len(tokens))
	for _, token := range tokens {
		stokens = append(stokens, string(token))
	}

	return stokens
}

var (
	extractTokens = []func(content []byte) (replacedContent []byte, tokens [][]byte){
		// The order to must be this
		extractAndReplaceShebang,
		extractAndReplaceSGML,
		skipCommentsAndLiterals,
		extractAndReplacePunctuation,
		extractAndReplaceRegular,
		extractAndReplaceOperator,
		extractRemainders,
	}

	// Differences between golang regexp and oniguruma:
	// 1. no (?s) in oniguruma - makes dot match \n
	// 2. no (?U) in oniguruma - ungreedy *
	// 3. (?m) implies dot matches \n in oniguruma
	// 4. oniguruma handles \w differently - impossible, but true
	//
	// Workarounds:
	// 1. (.|\n)
	// 2. replace * with *?
	// 3. replace . with [^\n]
	// 4. replace \w with [0-9A-Za-z_]
	//
	// Original golang regexps:
	//
	// reLiteralStringQuotes = regexp.MustCompile(`(?sU)(".*"|'.*')`)
	// reSingleLineComment   = regexp.MustCompile(`(?m)(//|--|#|%|")\s(.*$)`)
	// reMultilineComment    = regexp.MustCompile(`(?sU)(/\*.*\*/|<!--.*-->|\{-.*-\}|\(\*.*\*\)|""".*"""|'''.*''')`)
	// reLiteralNumber       = regexp.MustCompile(`(0x[0-9A-Fa-f]([0-9A-Fa-f]|\.)*|\d(\d|\.)*)([uU][lL]{0,2}|([eE][-+]\d*)?[fFlL]*)`)
	// reShebang             = regexp.MustCompile(`(?m)^#!(?:/\w+)*/(?:(\w+)|\w+(?:\s*\w+=\w+\s*)*\s*(\w+))(?:\s*-\w+\s*)*$`)
	// rePunctuation         = regexp.MustCompile(`;|\{|\}|\(|\)|\[|\]`)
	// reSGML                = regexp.MustCompile(`(?sU)(<\/?[^\s<>=\d"']+)(?:\s.*\/?>|>)`)
	// reSGMLComment         = regexp.MustCompile(`(?sU)(<!--.*-->)`)
	// reSGMLAttributes      = regexp.MustCompile(`\s+(\w+=)|\s+([^\s>]+)`)
	// reSGMLLoneAttribute   = regexp.MustCompile(`(\w+)`)
	// reRegularToken        = regexp.MustCompile(`[\w\.@#\/\*]+`)
	// reOperators           = regexp.MustCompile(`<<?|\+|\-|\*|\/|%|&&?|\|\|?`)
	//
	// These regexps were converted to work in the same way for both engines:
	//
	reLiteralStringQuotes = regex.MustCompile(`("(.|\n)*?"|'(.|\n)*?')`)
	reSingleLineComment   = regex.MustCompile(`(?m)(//|--|#|%|")\s([^\n]*$)`)
	reMultilineComment    = regex.MustCompile(`(/\*(.|\n)*?\*/|<!--(.|\n)*?-->|\{-(.|\n)*?-\}|\(\*(.|\n)*?\*\)|"""(.|\n)*?"""|'''(.|\n)*?''')`)
	reLiteralNumber       = regex.MustCompile(`(0x[0-9A-Fa-f]([0-9A-Fa-f]|\.)*|\d(\d|\.)*)([uU][lL]{0,2}|([eE][-+]\d*)?[fFlL]*)`)
	reShebang             = regex.MustCompile(`(?m)^#!(?:/[0-9A-Za-z_]+)*/(?:([0-9A-Za-z_]+)|[0-9A-Za-z_]+(?:\s*[0-9A-Za-z_]+=[0-9A-Za-z_]+\s*)*\s*([0-9A-Za-z_]+))(?:\s*-[0-9A-Za-z_]+\s*)*$`)
	rePunctuation         = regex.MustCompile(`;|\{|\}|\(|\)|\[|\]`)
	reSGML                = regex.MustCompile(`(<\/?[^\s<>=\d"']+)(?:\s(.|\n)*?\/?>|>)`)
	reSGMLComment         = regex.MustCompile(`(<!--(.|\n)*?-->)`)
	reSGMLAttributes      = regex.MustCompile(`\s+([0-9A-Za-z_]+=)|\s+([^\s>]+)`)
	reSGMLLoneAttribute   = regex.MustCompile(`([0-9A-Za-z_]+)`)
	reRegularToken        = regex.MustCompile(`[0-9A-Za-z_\.@#\/\*]+`)
	reOperators           = regex.MustCompile(`<<?|\+|\-|\*|\/|%|&&?|\|\|?`)

	regexToSkip = []regex.EnryRegexp{
		// The order must be this
		reLiteralStringQuotes,
		reMultilineComment,
		reSingleLineComment,
		reLiteralNumber,
	}
)

func extractAndReplaceShebang(content []byte) ([]byte, [][]byte) {
	var shebangTokens [][]byte
	matches := reShebang.FindAllSubmatch(content, -1)
	if matches != nil {
		shebangTokens = make([][]byte, 0, 2)
		for _, match := range matches {
			shebangToken := getShebangToken(match)
			shebangTokens = append(shebangTokens, shebangToken)
		}

		reShebang.ReplaceAll(content, []byte(` `))
	}

	return content, shebangTokens
}

func getShebangToken(matchedShebang [][]byte) []byte {
	const prefix = `SHEBANG#!`
	var token []byte
	for i := 1; i < len(matchedShebang); i++ {
		if len(matchedShebang[i]) > 0 {
			token = matchedShebang[i]
			break
		}
	}

	tokenShebang := append([]byte(prefix), token...)
	return tokenShebang
}

func commonExtractAndReplace(content []byte, re regex.EnryRegexp) ([]byte, [][]byte) {
	tokens := re.FindAll(content, -1)
	content = re.ReplaceAll(content, []byte(` `))
	return content, tokens
}

func extractAndReplacePunctuation(content []byte) ([]byte, [][]byte) {
	return commonExtractAndReplace(content, rePunctuation)
}

func extractAndReplaceRegular(content []byte) ([]byte, [][]byte) {
	return commonExtractAndReplace(content, reRegularToken)
}

func extractAndReplaceOperator(content []byte) ([]byte, [][]byte) {
	return commonExtractAndReplace(content, reOperators)
}

func extractAndReplaceSGML(content []byte) ([]byte, [][]byte) {
	var SGMLTokens [][]byte
	matches := reSGML.FindAllSubmatch(content, -1)
	if matches != nil {
		SGMLTokens = make([][]byte, 0, 2)
		for _, match := range matches {
			if reSGMLComment.Match(match[0]) {
				continue
			}

			token := append(match[1], '>')
			SGMLTokens = append(SGMLTokens, token)
			attributes := getSGMLAttributes(match[0])
			SGMLTokens = append(SGMLTokens, attributes...)
		}

		content = reSGML.ReplaceAll(content, []byte(` `))
	}

	return content, SGMLTokens
}

func getSGMLAttributes(SGMLTag []byte) [][]byte {
	var attributes [][]byte
	matches := reSGMLAttributes.FindAllSubmatch(SGMLTag, -1)
	if matches != nil {
		attributes = make([][]byte, 0, 5)
		for _, match := range matches {
			if len(match[1]) != 0 {
				attributes = append(attributes, match[1])
			}

			if len(match[2]) != 0 {
				loneAttributes := reSGMLLoneAttribute.FindAll(match[2], -1)
				attributes = append(attributes, loneAttributes...)
			}
		}
	}

	return attributes
}

func skipCommentsAndLiterals(content []byte) ([]byte, [][]byte) {
	for _, skip := range regexToSkip {
		content = skip.ReplaceAll(content, []byte(` `))
	}

	return content, nil
}

func extractRemainders(content []byte) ([]byte, [][]byte) {
	splitted := bytes.Fields(content)
	remainderTokens := make([][]byte, 0, len(splitted)*3)
	for _, remainder := range splitted {
		remainders := bytes.Split(remainder, nil)
		remainderTokens = append(remainderTokens, remainders...)
	}

	return content, remainderTokens
}
