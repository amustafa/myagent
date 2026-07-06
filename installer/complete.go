package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// completeDir performs shell-style tab completion over directories for the
// given partial path. It returns the (possibly extended) input and the list of
// candidate directory names when the completion is ambiguous.
//
//   - a single match completes fully and appends a trailing slash
//   - multiple matches extend to their longest common prefix and list them
//   - no match leaves the input unchanged
func completeDir(input string) (completed string, candidates []string) {
	expanded := expandHome(input)

	dir, base := filepath.Dir(expanded), filepath.Base(expanded)
	if strings.HasSuffix(input, string(os.PathSeparator)) || input == "" {
		dir, base = strings.TrimSuffix(expanded, string(os.PathSeparator)), ""
		if dir == "" {
			dir = "."
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return input, nil
	}

	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, base) {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)

	switch len(matches) {
	case 0:
		return input, nil
	case 1:
		full := filepath.Join(dir, matches[0]) + string(os.PathSeparator)
		return reHome(input, full), nil
	default:
		lcp := longestCommonPrefix(matches)
		if len(lcp) > len(base) {
			return reHome(input, filepath.Join(dir, lcp)), matches
		}
		return input, matches
	}
}

// reHome restores a leading ~ if the user's original input used one, so
// completion doesn't visibly rewrite ~/foo into /home/you/foo.
func reHome(original, expandedResult string) string {
	if strings.HasPrefix(original, "~") {
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(expandedResult, home) {
			return "~" + strings.TrimPrefix(expandedResult, home)
		}
	}
	return expandedResult
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	prefix := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}
