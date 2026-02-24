package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// IgnoreMatcher holds gitignore patterns and evaluates if a file should be ignored
type IgnoreMatcher struct {
	patterns []string
}

// NewIgnoreMatcher creates a new matcher from a .gitignore file
func NewIgnoreMatcher(dir string) (*IgnoreMatcher, error) {
	matcher := &IgnoreMatcher{
		patterns: []string{".git", ".git/", ".DS_Store", "node_modules/", "vendor/"}, // Default ignores
	}

	ignorePath := filepath.Join(dir, ".gitignore")
	file, err := os.Open(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return matcher, nil
		}
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		matcher.patterns = append(matcher.patterns, line)
	}

	return matcher, scanner.Err()
}

// IsIgnored returns true if the given path matches any ignore patterns
func (m *IgnoreMatcher) IsIgnored(path string, isDir bool) bool {
	// Normalize path
	path = filepath.ToSlash(path)

	// Clean leading slash if any
	path = strings.TrimPrefix(path, "/")

	for _, pattern := range m.patterns {
		pattern = filepath.ToSlash(pattern)
		isDirPattern := strings.HasSuffix(pattern, "/")

		cleanPattern := strings.TrimPrefix(pattern, "/")
		cleanPattern = strings.TrimSuffix(cleanPattern, "/")

		// Base name check
		base := filepath.Base(path)

		// If pattern ends with /, it only applies to directories
		if isDirPattern && !isDir {
			continue
		}

		// Direct match on base name
		if base == cleanPattern {
			return true
		}

		// Match using filepath.Match
		matched, _ := filepath.Match(cleanPattern, base)
		if matched {
			return true
		}

		// Also check full path matching (e.g. for foo/bar style patterns)
		if strings.Contains(cleanPattern, "/") {
			matched, _ = filepath.Match(cleanPattern, path)
			if matched {
				return true
			}
			// Prefix match for directories
			if strings.HasPrefix(path, cleanPattern+"/") {
				return true
			}
		}
	}
	return false
}
