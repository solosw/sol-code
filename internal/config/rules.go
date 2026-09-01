package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// ProjectRulesDirName is the subdirectory under .solcode that holds
	// project-only agent rules injected into the system prompt at startup.
	ProjectRulesDirName = "rules"

	projectRulesFileName = "rules.md"

	// maxProjectRulesBytes caps a single rule file so a huge dump cannot
	// blow the system prompt. Remaining files are still considered until
	// the combined budget is reached.
	maxProjectRulesFileBytes = 64 * 1024
	maxProjectRulesBytes     = 128 * 1024
)

// DefaultProjectRulesDir returns <workDir>/.solcode/rules.
func DefaultProjectRulesDir(workDir string) string {
	if dir := ProjectConfigDir(workDir); dir != "" {
		return filepath.Join(dir, ProjectRulesDirName)
	}
	return ""
}

// LoadProjectRules reads project-scoped agent rules from
// <workDir>/.solcode/rules.md and <workDir>/.solcode/rules/*.md.
// Missing paths are ignored. Only the project tree is consulted — never
// ~/.solcode.
func LoadProjectRules(workDir string) string {
	root := ProjectConfigDir(workDir)
	if root == "" {
		return ""
	}
	var parts []string
	used := 0
	appendFile := func(path, label string) {
		if used >= maxProjectRulesBytes {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		if len(data) > maxProjectRulesFileBytes {
			data = data[:maxProjectRulesFileBytes]
		}
		if remaining := maxProjectRulesBytes - used; len(data) > remaining {
			data = data[:remaining]
		}
		if !utf8.Valid(data) {
			return
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			return
		}
		used += len(data)
		if label != "" {
			parts = append(parts, "### "+label+"\n"+text)
			return
		}
		parts = append(parts, text)
	}

	appendFile(filepath.Join(root, projectRulesFileName), "")

	dir := filepath.Join(root, ProjectRulesDirName)
	entries, err := os.ReadDir(dir)
	if err == nil {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if !strings.EqualFold(filepath.Ext(name), ".md") {
				continue
			}
			if strings.EqualFold(name, projectRulesFileName) {
				// Avoid double-loading if someone also drops rules.md in the dir.
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			label := strings.TrimSuffix(name, filepath.Ext(name))
			appendFile(filepath.Join(dir, name), label)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// FormatProjectRulesPrompt wraps loaded project rules for the system prompt.
func FormatProjectRulesPrompt(rules string) string {
	rules = strings.TrimSpace(rules)
	if rules == "" {
		return ""
	}
	return strings.Join([]string{
		"Project rules:",
		"These instructions are project-specific and live under .solcode. Follow them for this working directory. If they conflict with a direct user request, follow the user request and mention the conflict.",
		rules,
	}, "\n")
}
