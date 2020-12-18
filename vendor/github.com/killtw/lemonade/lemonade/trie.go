package lemonade

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/killtw/lemonade/trie"
	"os"
)

var dbHost = getenv("DB_HOST", "")

type Word struct {
	Keyword string
}

func InitTrie() (err error) {
	t := trie.New()

	if dbHost != "" {
		words, err := getWords()

		if err != nil {
			return err
		}

		for _, word := range words {
			t.Add(word.Keyword)
		}

		fmt.Println("Words imported")
	}

	Trie = t

	return
}

func getWords() (words []Word, err error) {
	db, err := gorm.Open("mysql", dbHost)

	if err != nil {
		return nil, err
	}

	defer db.Close()

	db.Find(&words)

	return
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
