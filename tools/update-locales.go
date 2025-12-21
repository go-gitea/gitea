// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	localeDir       = "options/locale"
	defaultLocaleFileName  = "locale_en-US.json"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// remove empty values
	localeFiles, err := filepath.Glob(filepath.Join(localeDir, "*.json"))
	if err != nil {
		return fmt.Errorf("list %s: %w", localeDir, err)
	}

	for _, file := range localeFiles {
		if err := cleanLocaleFile(file); err != nil {
			return fmt.Errorf("clean %s: %w", file, err)
		}
	}

	// remove incomplete translations
	originalLocalePath := filepath.Join(localeDir, defaultLocaleFileName)
	baselineLines, err := countLines(originalLocalePath)
	if err != nil {
		return fmt.Errorf("count %s: %w", originalLocalePath, err)
	}
	threshold := baselineLines / 4 // 25% of baseline

	for _, file := range localeFiles {
		lines, err := countLines(file)
		if err != nil {
			return fmt.Errorf("count %s: %w", file, err)
		}
    fmt.Printf("Removing locale files with less than %d non-empty keys\n", threshold)
		if lines < threshold {
      fmt.Printf("%s: %d\n", file, lines)
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("remove %s: %w", file, err)
			}
		}
	}

	return nil
}

func cleanLocaleFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	buf := &bytes.Buffer{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		// remove json lines with empty values
		if strings.HasSuffix(line, `: "",`) || strings.HasSuffix(line, `: ""`) {
			continue
		}
		buf.WriteString(line + "\n")
	}

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	buf := make([]byte, 32*1024)
	lines := 0
	for {
		n, err := reader.Read(buf)
		lines += bytes.Count(buf[:n], []byte{'\n'})
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	return lines, nil
}
