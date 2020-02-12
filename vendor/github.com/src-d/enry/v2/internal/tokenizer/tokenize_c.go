// +build flex

package tokenizer

import "github.com/src-d/enry/v2/internal/tokenizer/flex"

// Tokenize returns lexical tokens from content. The tokens returned match what
// the Linguist library returns. At most the first ByteLimit bytes of content are tokenized.
func Tokenize(content []byte) []string {
	if len(content) > ByteLimit {
		content = content[:ByteLimit]
	}

	return flex.TokenizeFlex(content)
}
