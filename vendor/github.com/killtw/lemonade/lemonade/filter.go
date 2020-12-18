package lemonade

import (
	"strings"
	"unicode/utf8"
)

func Replace(message string) (string, []string) {
	matches := Trie.Search(message)
	r := make([]string, 2*len(matches))

	for i, word := range matches {
		r[i*2] = word
		r[i*2+1] = strings.Repeat("*", utf8.RuneCountInString(word))
	}

	return strings.NewReplacer(r...).Replace(message), matches
}

func Add(word string) {
	Trie.Add(word)
}
