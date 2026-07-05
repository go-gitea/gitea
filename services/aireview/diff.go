// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/gitrepo"
)

// FileDiff represents a parsed diff for a single file.
type FileDiff struct {
	Path     string
	Patch    string
	Language string
}

// GetPRDiff fetches the raw diff for a pull request, split by file.
func GetPRDiff(ctx context.Context, pr *issues_model.PullRequest) ([]FileDiff, error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, fmt.Errorf("load base repo: %w", err)
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.BaseRepo)
	if err != nil {
		return nil, fmt.Errorf("open git repo: %w", err)
	}
	defer closer.Close()

	headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitHeadRefName())
	if err != nil {
		return nil, fmt.Errorf("get head commit ref: %w", err)
	}

	mergeBase, err := gitrepo.MergeBase(ctx, pr.BaseRepo, pr.BaseBranch, headCommitID)
	if err != nil {
		return nil, fmt.Errorf("get merge base: %w", err)
	}

	var buf bytes.Buffer
	compareArg := mergeBase + "..." + headCommitID
	if err := gitRepo.GetDiff(compareArg, &buf); err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}

	return parseUnifiedDiff(buf.String()), nil
}

func parseUnifiedDiff(diff string) []FileDiff {
	if diff == "" {
		return nil
	}

	var files []FileDiff
	var current FileDiff
	var patchBuf strings.Builder

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if current.Path != "" {
				current.Patch = patchBuf.String()
				files = append(files, current)
				patchBuf.Reset()
			}
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				current.Path = strings.TrimPrefix(parts[len(parts)-1], "b/")
				current.Language = detectLanguage(current.Path)
			} else {
				current.Path = ""
			}
		}
		patchBuf.WriteString(line)
		patchBuf.WriteString("\n")
	}

	if current.Path != "" {
		current.Patch = patchBuf.String()
		files = append(files, current)
	}

	return files
}

func detectLanguage(path string) string {
	ext := ""
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		ext = strings.ToLower(path[idx+1:])
	}

	langMap := map[string]string{
		"go":         "Go",
		"js":         "JavaScript",
		"ts":         "TypeScript",
		"jsx":        "JavaScript (React)",
		"tsx":        "TypeScript (React)",
		"py":         "Python",
		"java":       "Java",
		"rs":         "Rust",
		"rb":         "Ruby",
		"php":        "PHP",
		"c":          "C",
		"h":          "C",
		"cpp":        "C++",
		"cc":         "C++",
		"cxx":        "C++",
		"hpp":        "C++",
		"hh":         "C++",
		"cs":         "C#",
		"swift":      "Swift",
		"kt":         "Kotlin",
		"kts":        "Kotlin",
		"scala":      "Scala",
		"html":       "HTML",
		"htm":        "HTML",
		"css":        "CSS",
		"scss":       "SCSS",
		"sass":       "Sass",
		"less":       "Less",
		"sql":        "SQL",
		"sh":         "Shell",
		"bash":       "Shell",
		"zsh":        "Shell",
		"yaml":       "YAML",
		"yml":        "YAML",
		"json":       "JSON",
		"xml":        "XML",
		"md":         "Markdown",
		"proto":      "Protocol Buffers",
		"toml":       "TOML",
		"ini":        "INI",
		"cfg":        "INI",
		"dockerfile": "Dockerfile",
		"makefile":   "Makefile",
		"cmake":      "CMake",
		"lua":        "Lua",
		"pl":         "Perl",
		"pm":         "Perl",
		"r":          "R",
		"dart":       "Dart",
		"vue":        "Vue",
		"svelte":     "Svelte",
		"tf":         "Terraform",
		"zig":        "Zig",
		"ex":         "Elixir",
		"exs":        "Elixir",
		"erl":        "Erlang",
		"clj":        "Clojure",
		"cljs":       "ClojureScript",
		"hs":         "Haskell",
		"lhs":        "Haskell",
		"ml":         "OCaml",
		"mli":        "OCaml",
	}

	if lang, ok := langMap[ext]; ok {
		return lang
	}

	base := strings.ToLower(path)
	if strings.Contains(base, "dockerfile") {
		return "Dockerfile"
	}
	if strings.Contains(base, "makefile") {
		return "Makefile"
	}
	if strings.Contains(base, "gemfile") {
		return "Ruby"
	}

	return ""
}
