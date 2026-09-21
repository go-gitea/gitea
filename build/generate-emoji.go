// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2015 Kenneth Shaw
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"

	"gitea.dev/modules/json"
)

const (
	emojiTestURL = "https://www.unicode.org/Public/17.0.0/emoji/emoji-test.txt"
	jsonFile     = "public/assets/emoji.json"
)

type emoji struct {
	Emoji       string   `json:"emoji"`
	Aliases     []string `json:"aliases"`
	description string
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func main() {
	if err := generate(); err != nil {
		log.Fatal(err)
	}
}

func generate() error {
	// the existing file is also the alias source, so existing aliases stay stable
	existing, err := os.ReadFile(jsonFile)
	if err != nil {
		return err
	}
	var existingEmojis []*emoji
	if err := json.Unmarshal(existing, &existingEmojis); err != nil {
		return err
	}
	existingAliases := make(map[string][]string, len(existingEmojis))
	for _, e := range existingEmojis {
		existingAliases[e.Emoji] = e.Aliases
	}

	emojis, err := fetchEmojis(existingAliases)
	if err != nil {
		return err
	}

	lines := make([]string, len(emojis))
	for i, e := range emojis {
		line, err := json.Marshal(e)
		if err != nil {
			return err
		}
		lines[i] = string(line)
	}
	return os.WriteFile(jsonFile, []byte("[\n"+strings.Join(lines, ",\n")+"\n]\n"), 0o644)
}

func isSkinTone(r rune) bool {
	return r >= 0x1f3fb && r <= 0x1f3ff // U+1F3FB to U+1F3FF are the five skin tone modifiers
}

func fetchEmojis(existingAliases map[string][]string) ([]*emoji, error) {
	res, err := http.Get(emojiTestURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", emojiTestURL, res.Status)
	}

	var emojis []*emoji
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		// e.g. "1F44D ; fully-qualified # 👍 E0.6 thumbs up"
		_, rest, _ := strings.Cut(scanner.Text(), ";")
		status, comment, _ := strings.Cut(rest, "#")
		if strings.TrimSpace(status) != "fully-qualified" {
			continue
		}
		fields := strings.SplitN(strings.TrimSpace(comment), " ", 3)
		if strings.ContainsFunc(fields[0], isSkinTone) {
			continue
		}
		emojis = append(emojis, &emoji{Emoji: fields[0], description: fields[2]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var errs []error
	aliasOwners := map[string]string{}
	for _, e := range emojis {
		e.Aliases = existingAliases[e.Emoji]
		if e.Aliases == nil {
			e.Aliases = []string{strings.Trim(slugRe.ReplaceAllString(strings.ReplaceAll(strings.ToLower(e.description), "’", ""), "_"), "_")}
		}
		delete(existingAliases, e.Emoji)
		for _, alias := range e.Aliases {
			if owner, ok := aliasOwners[alias]; ok {
				errs = append(errs, fmt.Errorf("alias %q used by both %q and %q", alias, owner, e.Emoji))
			}
			aliasOwners[alias] = e.Emoji
		}
	}
	for code, aliases := range existingAliases {
		errs = append(errs, fmt.Errorf("emoji %q with aliases %v is missing from Unicode data", code, aliases))
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	slices.SortFunc(emojis, func(a, b *emoji) int {
		return strings.Compare(a.Aliases[0], b.Aliases[0])
	})
	return emojis, nil
}
